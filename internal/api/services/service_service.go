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
	"github.com/vricosti/hibana-stack/internal/utils"
)

// ServiceService handles service/container operations
type ServiceService struct {
	db *sql.DB
}

// NewServiceService creates a new service service
func NewServiceService(db *sql.DB) *ServiceService {
	return &ServiceService{db: db}
}

// ListServices returns all services for a domain
func (s *ServiceService) ListServices(domainName string) ([]models.Service, error) {
	// Get subdomains from database
	query := `
		SELECT d.id, d.name FROM domains d WHERE d.name = $1
	`
	var domainID int
	var name string
	err := s.db.QueryRow(query, domainName).Scan(&domainID, &name)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	systemName := utils.DomainToSystemName(domainName)

	// Define standard services
	services := []models.Service{
		{
			Name:          "www",
			Role:          "website",
			ContainerName: "www-" + systemName,
			Deployable:    true,
		},
		{
			Name:          "adm",
			Role:          "webadmin",
			ContainerName: "adm-" + systemName,
			Deployable:    false,
		},
		{
			Name:          "webmail",
			Role:          "webmail",
			ContainerName: "webmail-" + systemName,
			Deployable:    false,
		},
		{
			Name:          "mail",
			Role:          "mailserver",
			ContainerName: "", // System service, no container
			Deployable:    false,
		},
	}

	// Get status for each service
	for i := range services {
		if services[i].ContainerName != "" {
			status, _ := s.getContainerStatus(services[i].ContainerName)
			services[i].Status = status
		} else {
			services[i].Status = "system"
		}
	}

	return services, nil
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
	// Only www is deployable
	if serviceName != "www" {
		return nil, fmt.Errorf("only www service is deployable")
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
