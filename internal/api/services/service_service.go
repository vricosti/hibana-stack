package services

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/dnsprovider"
	"github.com/vricosti/hibana-stack/internal/utils"
)

// ServiceService handles service/container operations
type ServiceService struct {
	db     *sql.DB
	pdnsDB *sql.DB
}

// NewServiceService creates a new service service
func NewServiceService(db *sql.DB, pdnsDB *sql.DB) *ServiceService {
	return &ServiceService{db: db, pdnsDB: pdnsDB}
}

// ListServices returns all services for a domain by scanning the filesystem
func (s *ServiceService) ListServices(domainName string) ([]models.Service, error) {
	// Get domain from database
	query := `SELECT d.id, d.name FROM domains d WHERE d.name = $1`
	var domainID int
	var name string
	err := s.db.QueryRow(query, domainName).Scan(&domainID, &name)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	systemName := utils.DomainToSystemName(domainName)
	domainPath := utils.DomainToPath(domainName)

	var services []models.Service
	foundServices := make(map[string]bool)

	// Scan domain directory for services (subdirectories with docker-compose.yml)
	entries, err := os.ReadDir(domainPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read domain directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		serviceName := entry.Name()
		servicePath := filepath.Join(domainPath, serviceName)

		// Check if it has a docker-compose.yml
		composePath := filepath.Join(servicePath, "docker-compose.yml")
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			continue
		}

		// Determine role and deployability based on service name
		role := "website"
		deployable := true
		isCustom := true

		switch serviceName {
		case "www":
			role = "website"
			deployable = true
			isCustom = false
		case "adm":
			role = "webadmin"
			deployable = false
			isCustom = false
		case "webmail":
			role = "webmail"
			deployable = false
			isCustom = false
		case "mail":
			// Skip - mail is handled separately as dockerized container
			continue
		}

		containerName := serviceName + "-" + systemName
		status, _ := s.getContainerStatus(containerName)

		services = append(services, models.Service{
			Name:          serviceName,
			Role:          role,
			ContainerName: containerName,
			Status:        status,
			Deployable:    deployable,
			IsCustom:      isCustom,
		})
		foundServices[serviceName] = true
	}

	// Check for mail service (dockerized) - check if mail container exists
	mailContainerName := "hibana-mail-" + systemName
	if s.isMailContainerDeployed(mailContainerName) {
		services = append(services, models.Service{
			Name:          "mail",
			Role:          "mailserver",
			ContainerName: mailContainerName,
			Status:        s.getMailContainerStatus(mailContainerName),
			Deployable:    false,
			IsCustom:      false,
		})
	}

	// Add custom subdomains from database that don't have directories yet (pending creation)
	subdomainQuery := `SELECT name, role FROM subdomains WHERE domain_id = $1 ORDER BY name`
	rows, err := s.db.Query(subdomainQuery, domainID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var subName, subRole string
			if err := rows.Scan(&subName, &subRole); err != nil {
				continue
			}
			// Only add if not already found in filesystem
			if !foundServices[subName] {
				services = append(services, models.Service{
					Name:          subName,
					Role:          subRole,
					ContainerName: subName + "-" + systemName,
					Status:        "not_deployed",
					Deployable:    true,
					IsCustom:      true,
				})
			}
		}
	}

	return services, nil
}

// isMailContainerDeployed checks if a dockerized mail container exists for this domain
func (s *ServiceService) isMailContainerDeployed(containerName string) bool {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=^"+containerName+"$", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == containerName
}

// getMailContainerStatus returns the status of a dockerized mail container
func (s *ServiceService) getMailContainerStatus(containerName string) string {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=^"+containerName+"$", "--format", "{{.Status}}")
	output, err := cmd.Output()
	if err != nil {
		return "not_deployed"
	}

	status := strings.TrimSpace(string(output))
	if status == "" {
		return "not_deployed"
	}

	if strings.HasPrefix(status, "Up") {
		return "running"
	}

	return "stopped"
}

// IsDeployable checks if a service is deployable
func (s *ServiceService) IsDeployable(domainName, serviceName string) bool {
	// www is always deployable
	if serviceName == "www" {
		return true
	}

	// Check if it's a custom subdomain (which are deployable)
	query := `
		SELECT 1 FROM subdomains s
		JOIN domains d ON s.domain_id = d.id
		WHERE d.name = $1 AND s.name = $2
	`
	var exists int
	err := s.db.QueryRow(query, domainName, serviceName).Scan(&exists)
	return err == nil
}

// CreateSubdomain creates a new custom subdomain service
func (s *ServiceService) CreateSubdomain(domainName, subdomainName string) (*models.Service, error) {
	// Get domain ID and server IP
	var domainID int
	var serverIP string
	err := s.db.QueryRow(`SELECT id, server_ip FROM domains WHERE name = $1`, domainName).Scan(&domainID, &serverIP)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	// Insert subdomain into database
	_, err = s.db.Exec(`
		INSERT INTO subdomains (domain_id, name, role) VALUES ($1, $2, 'website')
	`, domainID, subdomainName)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("subdomain '%s' already exists", subdomainName)
		}
		return nil, fmt.Errorf("failed to create subdomain: %w", err)
	}

	// Create directory structure
	if err := s.createSubdomainDirectories(domainName, subdomainName); err != nil {
		// Rollback database insert
		s.db.Exec(`DELETE FROM subdomains WHERE domain_id = $1 AND name = $2`, domainID, subdomainName)
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// Create DNS A record for the subdomain
	if err := s.createSubdomainDNSRecord(domainName, subdomainName, serverIP); err != nil {
		// Log but don't fail - DNS can be added manually
		fmt.Printf("Warning: could not create DNS record for %s.%s: %v\n", subdomainName, domainName, err)
	}

	systemName := utils.DomainToSystemName(domainName)
	return &models.Service{
		Name:          subdomainName,
		Role:          "website",
		ContainerName: subdomainName + "-" + systemName,
		Status:        "not_deployed",
		Deployable:    true,
		IsCustom:      true,
	}, nil
}

// DeleteSubdomain deletes a custom subdomain service
func (s *ServiceService) DeleteSubdomain(domainName, subdomainName string) error {
	// Get domain ID
	var domainID int
	err := s.db.QueryRow(`SELECT id FROM domains WHERE name = $1`, domainName).Scan(&domainID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	// Check if subdomain exists
	var exists int
	err = s.db.QueryRow(`SELECT 1 FROM subdomains WHERE domain_id = $1 AND name = $2`, domainID, subdomainName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("subdomain not found: %w", err)
	}

	// Stop and remove container
	systemName := utils.DomainToSystemName(domainName)
	containerName := subdomainName + "-" + systemName
	domainPath := utils.DomainToPath(domainName)
	composePath := filepath.Join(domainPath, subdomainName, "docker-compose.yml")

	if _, err := os.Stat(composePath); err == nil {
		cmd := exec.Command("docker-compose", "-f", composePath, "down", "--rmi", "local", "-v")
		cmd.Run() // Ignore errors
	}

	// Remove container if it exists
	exec.Command("docker", "rm", "-f", containerName).Run()

	// Remove directory
	servicePath := filepath.Join(domainPath, subdomainName)
	if err := os.RemoveAll(servicePath); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: could not remove directory %s: %v\n", servicePath, err)
	}

	// Delete DNS record for the subdomain
	if err := s.deleteSubdomainDNSRecord(domainName, subdomainName); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: could not delete DNS record for %s.%s: %v\n", subdomainName, domainName, err)
	}

	// Delete from database
	_, err = s.db.Exec(`DELETE FROM subdomains WHERE domain_id = $1 AND name = $2`, domainID, subdomainName)
	if err != nil {
		return fmt.Errorf("failed to delete subdomain: %w", err)
	}

	return nil
}

// createSubdomainDirectories creates the directory structure for a new subdomain
func (s *ServiceService) createSubdomainDirectories(domainName, subdomainName string) error {
	domainPath := utils.DomainToPath(domainName)
	servicePath := filepath.Join(domainPath, subdomainName)
	systemName := utils.DomainToSystemName(domainName)
	containerName := subdomainName + "-" + systemName

	// Get domain username for ownership
	var domainUsername string
	_ = s.db.QueryRow(`
		SELECT du.username FROM domain_users du
		JOIN domains d ON du.domain_id = d.id
		WHERE d.name = $1
	`, domainName).Scan(&domainUsername)

	// If no domain_users entry, derive username from domain name
	// (hibana add domain creates user but doesn't insert into domain_users)
	if domainUsername == "" {
		domainUsername = systemName // e.g., vricosti-com
	}

	// Create directories
	dirs := []string{
		servicePath,
		filepath.Join(servicePath, "src"),
		filepath.Join(servicePath, "nginx"),
		filepath.Join(servicePath, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create default Dockerfile
	// Note: Build context is ./src, so paths are relative to that directory
	// The nginx config is mounted as a volume, not copied in the image
	dockerfile := `FROM nginx:alpine
COPY index.html /usr/share/nginx/html/
`
	if err := os.WriteFile(filepath.Join(servicePath, "src", "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}

	// Create default index.html
	indexHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s.%s</title>
    <style>
        body { font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f5f5f5; }
        .container { text-align: center; }
        h1 { color: #333; }
    </style>
</head>
<body>
    <div class="container">
        <h1>%s.%s</h1>
        <p>Your website is ready to be deployed.</p>
    </div>
</body>
</html>
`, subdomainName, domainName, subdomainName, domainName)
	if err := os.WriteFile(filepath.Join(servicePath, "src", "index.html"), []byte(indexHTML), 0644); err != nil {
		return fmt.Errorf("failed to create index.html: %w", err)
	}

	// Create nginx config
	nginxConf := `server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    error_page 500 502 503 504 /50x.html;
    location = /50x.html {
        root /usr/share/nginx/html;
    }
}
`
	if err := os.WriteFile(filepath.Join(servicePath, "nginx", "default.conf"), []byte(nginxConf), 0644); err != nil {
		return fmt.Errorf("failed to create nginx config: %w", err)
	}

	// Create docker-compose.yml
	dockerCompose := fmt.Sprintf("version: '3.8'\n\nservices:\n  %s:\n    build:\n      context: ./src\n      dockerfile: Dockerfile\n    container_name: %s\n    restart: unless-stopped\n    volumes:\n      - ./logs:/var/log/nginx\n      - ./nginx/default.conf:/etc/nginx/conf.d/default.conf:ro\n    networks:\n      - traefik-network\n    labels:\n      - \"traefik.enable=true\"\n      - \"traefik.http.routers.%s.rule=Host(`%s.%s`)\"\n      - \"traefik.http.routers.%s.entrypoints=websecure\"\n      - \"traefik.http.routers.%s.tls.certresolver=letsencrypt\"\n      - \"traefik.http.services.%s.loadbalancer.server.port=80\"\n      - \"traefik.http.routers.%s.middlewares=security-headers@file,compress@file\"\n\nnetworks:\n  traefik-network:\n    external: true\n",
		subdomainName, containerName, containerName, subdomainName, domainName, containerName, containerName, containerName, containerName)

	if err := os.WriteFile(filepath.Join(servicePath, "docker-compose.yml"), []byte(dockerCompose), 0644); err != nil {
		return fmt.Errorf("failed to create docker-compose.yml: %w", err)
	}

	// Set ownership to domain user if configured
	if domainUsername != "" {
		cmd := exec.Command("chown", "-R", domainUsername+":hibana-domains", servicePath)
		if err := cmd.Run(); err != nil {
			// Log but don't fail - ownership can be fixed manually
			fmt.Printf("Warning: could not set ownership for %s: %v\n", servicePath, err)
		}
	}

	return nil
}

// getContainerStatus returns the status of a Docker container
func (s *ServiceService) getContainerStatus(containerName string) (string, error) {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=^"+containerName+"$", "--format", "{{.Status}}")
	output, err := cmd.Output()
	if err != nil {
		return "not_deployed", nil
	}

	status := strings.TrimSpace(string(output))
	if status == "" {
		return "not_deployed", nil
	}

	if strings.HasPrefix(status, "Up") {
		return "running", nil
	}

	return "stopped", nil
}

// getDNSProviderConfig retrieves the DNS provider configuration from the database
func (s *ServiceService) getDNSProviderConfig() (providerType, providerName, apiToken string, err error) {
	// Get DNS provider type and name from configuration
	err = s.db.QueryRow(`SELECT value FROM configuration WHERE key = 'dns_provider_type'`).Scan(&providerType)
	if err != nil {
		return "", "", "", fmt.Errorf("DNS provider not configured")
	}

	err = s.db.QueryRow(`SELECT value FROM configuration WHERE key = 'dns_provider_name'`).Scan(&providerName)
	if err != nil {
		return "", "", "", fmt.Errorf("DNS provider name not found")
	}

	// Get API token from dns_providers table
	err = s.db.QueryRow(`SELECT api_token FROM dns_providers WHERE provider = $1`, providerName).Scan(&apiToken)
	if err != nil {
		// API token may not be required for all providers
		apiToken = ""
	}

	return providerType, providerName, apiToken, nil
}

// createSubdomainDNSRecord creates an A record for the subdomain
func (s *ServiceService) createSubdomainDNSRecord(domainName, subdomainName, serverIP string) error {
	providerType, providerName, apiToken, err := s.getDNSProviderConfig()
	if err != nil {
		return err
	}

	// Skip DNS record creation for manual type
	if providerType == "manual" {
		return nil
	}

	switch providerName {
	case "hostinger":
		if apiToken == "" {
			return fmt.Errorf("Hostinger API token not configured")
		}
		provider := dnsprovider.NewHostingerProvider(apiToken)
		// Use DeleteCNAMEAndCreateA to handle the case where a CNAME exists (e.g., www -> domain.com)
		// This will delete the CNAME and create an A record pointing to the server IP
		return provider.DeleteCNAMEAndCreateA(domainName, subdomainName, serverIP)

	case "powerdns":
		// For local PowerDNS, insert directly into the database
		if s.pdnsDB == nil {
			return fmt.Errorf("PowerDNS database not configured")
		}

		// Get PowerDNS domain ID
		var pdnsID int
		err := s.pdnsDB.QueryRow(`SELECT id FROM domains WHERE name = $1`, domainName).Scan(&pdnsID)
		if err != nil {
			return fmt.Errorf("domain not found in PowerDNS: %w", err)
		}

		// Create the A record
		recordName := subdomainName + "." + domainName
		_, err = s.pdnsDB.Exec(`
			INSERT INTO records (domain_id, name, type, content, ttl)
			VALUES ($1, $2, 'A', $3, 300)
		`, pdnsID, recordName, serverIP)
		if err != nil {
			return fmt.Errorf("failed to create DNS record: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}

// deleteSubdomainDNSRecord deletes the A record for the subdomain
func (s *ServiceService) deleteSubdomainDNSRecord(domainName, subdomainName string) error {
	providerType, providerName, apiToken, err := s.getDNSProviderConfig()
	if err != nil {
		return err
	}

	// Skip DNS record deletion for manual type
	if providerType == "manual" {
		return nil
	}

	switch providerName {
	case "hostinger":
		if apiToken == "" {
			return fmt.Errorf("Hostinger API token not configured")
		}
		provider := dnsprovider.NewHostingerProvider(apiToken)
		return provider.DeleteRecordByNameType(domainName, subdomainName, "A")

	case "powerdns":
		// For local PowerDNS, delete from the database
		if s.pdnsDB == nil {
			return fmt.Errorf("PowerDNS database not configured")
		}

		// Get PowerDNS domain ID
		var pdnsID int
		err := s.pdnsDB.QueryRow(`SELECT id FROM domains WHERE name = $1`, domainName).Scan(&pdnsID)
		if err != nil {
			return fmt.Errorf("domain not found in PowerDNS: %w", err)
		}

		// Delete the A record
		recordName := subdomainName + "." + domainName
		_, err = s.pdnsDB.Exec(`
			DELETE FROM records WHERE domain_id = $1 AND name = $2 AND type = 'A'
		`, pdnsID, recordName)
		if err != nil {
			return fmt.Errorf("failed to delete DNS record: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}

// StartService starts a service
func (s *ServiceService) StartService(domainName, serviceName string) error {
	domainPath := utils.DomainToPath(domainName)
	composePath := filepath.Join(domainPath, serviceName, "docker-compose.yml")

	// Check if compose file exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found for service %s", serviceName)
	}

	cmd := exec.Command("docker-compose", "-f", composePath, "up", "-d")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service: %w, output: %s", err, string(output))
	}

	return nil
}

// StopService stops a service
func (s *ServiceService) StopService(domainName, serviceName string) error {
	domainPath := utils.DomainToPath(domainName)
	composePath := filepath.Join(domainPath, serviceName, "docker-compose.yml")

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found for service %s", serviceName)
	}

	cmd := exec.Command("docker-compose", "-f", composePath, "down")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop service: %w, output: %s", err, string(output))
	}

	return nil
}

// RestartService restarts a service
func (s *ServiceService) RestartService(domainName, serviceName string) error {
	domainPath := utils.DomainToPath(domainName)
	composePath := filepath.Join(domainPath, serviceName, "docker-compose.yml")

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("docker-compose.yml not found for service %s", serviceName)
	}

	cmd := exec.Command("docker-compose", "-f", composePath, "restart")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart service: %w, output: %s", err, string(output))
	}

	return nil
}

// GetLogs returns logs from a service
func (s *ServiceService) GetLogs(domainName, serviceName string, lines int) (*models.ServiceLogs, error) {
	systemName := utils.DomainToSystemName(domainName)
	containerName := serviceName + "-" + systemName

	if lines <= 0 {
		lines = 100
	}

	cmd := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", lines), containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	return &models.ServiceLogs{
		ServiceName: serviceName,
		Logs:        string(output),
		Lines:       lines,
	}, nil
}

// Deploy deploys a new version of a service
func (s *ServiceService) Deploy(domainName, serviceName string, req *models.DeployRequest) (*models.DeployResponse, error) {
	// Check if service is deployable
	if !s.IsDeployable(domainName, serviceName) {
		return nil, fmt.Errorf("service '%s' is not deployable", serviceName)
	}

	domainPath := utils.DomainToPath(domainName)
	servicePath := filepath.Join(domainPath, serviceName)

	var output strings.Builder

	// Step 1: Stop existing container
	output.WriteString("Stopping existing container...\n")
	composePath := filepath.Join(servicePath, "docker-compose.yml")
	hibanComposePath := filepath.Join(servicePath, "docker-compose.hibana.yml")

	// Try hibana compose first, then regular
	activeComposePath := composePath
	if _, err := os.Stat(hibanComposePath); err == nil {
		activeComposePath = hibanComposePath
	}

	if _, err := os.Stat(activeComposePath); err == nil {
		cmd := exec.Command("docker-compose", "-f", activeComposePath, "down")
		cmdOutput, _ := cmd.CombinedOutput()
		output.WriteString(string(cmdOutput))
	}

	// Step 2: Backup important files
	output.WriteString("\nBacking up configuration files...\n")
	backupDir, err := os.MkdirTemp("", "hibana-backup-")
	if err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}
	defer os.RemoveAll(backupDir)

	filesToBackup := []string{".ssh", ".env", "secrets", ".gitignore"}
	for _, f := range filesToBackup {
		src := filepath.Join(servicePath, f)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(backupDir, f)
			if err := copyPath(src, dst); err != nil {
				output.WriteString(fmt.Sprintf("Warning: could not backup %s: %v\n", f, err))
			} else {
				output.WriteString(fmt.Sprintf("Backed up %s\n", f))
			}
		}
	}

	// Step 3: Clear service directory (except backups)
	output.WriteString("\nClearing service directory...\n")
	entries, err := os.ReadDir(servicePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read service directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip files we backed up
		skip := false
		for _, b := range filesToBackup {
			if name == b {
				skip = true
				break
			}
		}
		if !skip {
			path := filepath.Join(servicePath, name)
			if err := os.RemoveAll(path); err != nil {
				output.WriteString(fmt.Sprintf("Warning: could not remove %s: %v\n", name, err))
			}
		}
	}

	// Step 4: Deploy based on source
	if req.Source == "git" {
		output.WriteString(fmt.Sprintf("\nCloning from %s (branch: %s)...\n", req.GitURL, req.GitBranch))

		branch := req.GitBranch
		if branch == "" {
			branch = "main"
		}

		// Use SSH config from domain's .ssh directory
		sshDir := filepath.Join(domainPath, ".ssh")
		env := os.Environ()
		env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s/id_ed25519 -o StrictHostKeyChecking=no", sshDir))

		// Clone into service directory
		cmd := exec.Command("git", "clone", "--branch", branch, "--depth", "1", req.GitURL, ".")
		cmd.Dir = servicePath
		cmd.Env = env
		cmdOutput, err := cmd.CombinedOutput()
		output.WriteString(string(cmdOutput))
		if err != nil {
			return &models.DeployResponse{
				Success: false,
				Message: "Git clone failed",
				Output:  output.String(),
			}, nil
		}
	} else if req.Source == "upload" {
		output.WriteString("\nExtracting uploaded archive...\n")
		// File should already be uploaded to a temp location
		// This is handled by the handler
	}

	// Step 5: Restore backed up files
	output.WriteString("\nRestoring configuration files...\n")
	for _, f := range filesToBackup {
		src := filepath.Join(backupDir, f)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(servicePath, f)
			// Remove if exists (from git clone)
			os.RemoveAll(dst)
			if err := copyPath(src, dst); err != nil {
				output.WriteString(fmt.Sprintf("Warning: could not restore %s: %v\n", f, err))
			} else {
				output.WriteString(fmt.Sprintf("Restored %s\n", f))
			}
		}
	}

	// Step 6: Find and run docker-compose
	output.WriteString("\nStarting container...\n")

	// Look for docker-compose.hibana.yml first
	activeComposePath = filepath.Join(servicePath, "docker-compose.hibana.yml")
	if _, err := os.Stat(activeComposePath); os.IsNotExist(err) {
		activeComposePath = filepath.Join(servicePath, "docker-compose.yml")
	}

	if _, err := os.Stat(activeComposePath); os.IsNotExist(err) {
		return &models.DeployResponse{
			Success: false,
			Message: "No docker-compose.yml or docker-compose.hibana.yml found",
			Output:  output.String(),
		}, nil
	}

	// Update container name in compose file to match domain
	output.WriteString(fmt.Sprintf("Using compose file: %s\n", filepath.Base(activeComposePath)))

	cmd := exec.Command("docker-compose", "-f", activeComposePath, "up", "-d", "--build")
	cmd.Dir = servicePath
	cmdOutput, err := cmd.CombinedOutput()
	output.WriteString(string(cmdOutput))

	if err != nil {
		return &models.DeployResponse{
			Success: false,
			Message: "Docker compose failed",
			Output:  output.String(),
		}, nil
	}

	return &models.DeployResponse{
		Success: true,
		Message: fmt.Sprintf("Service %s deployed successfully", serviceName),
		Output:  output.String(),
	}, nil
}

// ExtractArchive extracts a ZIP or TAR.GZ archive to a directory
func (s *ServiceService) ExtractArchive(archivePath, destPath string) error {
	// Determine archive type
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destPath)
	} else if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return extractTarGz(archivePath, destPath)
	} else if strings.HasSuffix(archivePath, ".tar") {
		return extractTar(archivePath, destPath)
	}

	return fmt.Errorf("unsupported archive format")
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func extractTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return extractTarReader(tar.NewReader(gzr), dest)
}

func extractTar(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	return extractTarReader(tar.NewReader(file), dest)
}

func extractTarReader(tr *tar.Reader, dest string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fpath := filepath.Join(dest, header.Name)

		// Check for path traversal
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
				return err
			}

			outFile, err := os.Create(fpath)
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			if err := os.Chmod(fpath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyPath copies a file or directory
func copyPath(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}

	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
