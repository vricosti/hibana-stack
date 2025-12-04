package installer

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vricosti/hibana-stack/internal/config"
)

// SetupPowerDNS configures PowerDNS server
func (i *Installer) SetupPowerDNS() error {
	// Open firewall ports for DNS
	if err := i.openDNSPorts(); err != nil {
		fmt.Printf("Warning: failed to open DNS ports in firewall: %v\n", err)
		fmt.Println("   You may need to open ports 53/tcp and 53/udp manually")
	}

	// Test PostgreSQL connection first
	if err := i.testPowerDNSConnection(); err != nil {
		return fmt.Errorf("PowerDNS database connection test failed: %w", err)
	}

	// Configure PowerDNS to use PostgreSQL backend
	if err := i.configurePowerDNSConfig(); err != nil {
		return err
	}

	// Start and enable PowerDNS
	if err := i.startPowerDNS(); err != nil {
		return err
	}

	// Update resolv.conf to use PowerDNS
	if err := i.configureResolvConf(); err != nil {
		return fmt.Errorf("failed to configure resolv.conf: %w", err)
	}

	// Store all domains in Hibana database now (before mail server setup)
	if err := i.ensureDomainsInHibana(); err != nil {
		return fmt.Errorf("failed to store domains in Hibana database: %w", err)
	}

	return nil
}

// testPowerDNSConnection tests the connection to PowerDNS database
func (i *Installer) testPowerDNSConnection() error {
	connStr := fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable",
		pdnsUser, i.getOrGeneratePassword(pdnsUser), pdnsDatabaseName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Test that tables exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'domains'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check domains table: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("domains table does not exist")
	}

	fmt.Println("PowerDNS database connection successful")
	return nil
}

// configurePowerDNSConfig creates PowerDNS configuration file
func (i *Installer) configurePowerDNSConfig() error {
	pdnsConfig := fmt.Sprintf(`# PowerDNS Configuration
launch=gpgsql
gpgsql-host=127.0.0.1
gpgsql-port=5432
gpgsql-dbname=%s
gpgsql-user=%s
gpgsql-password=%s
gpgsql-dnssec=yes

# API Configuration
api=yes
api-key=%s
webserver=yes
webserver-address=127.0.0.1
webserver-port=8081
webserver-allow-from=127.0.0.1

# Other settings
local-address=0.0.0.0
local-port=53
master=yes
slave=yes
`,
		pdnsDatabaseName,
		pdnsUser,
		i.getOrGeneratePassword(pdnsUser),
		i.getOrGeneratePassword("pdns-api"),
	)

	configPath := "/etc/powerdns/pdns.conf"

	// Backup existing config
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".backup"
		if err := exec.Command("cp", configPath, backupPath).Run(); err != nil {
			return fmt.Errorf("failed to backup PowerDNS config: %w", err)
		}
	}

	// Write new config
	if err := os.WriteFile(configPath, []byte(pdnsConfig), 0640); err != nil {
		return fmt.Errorf("failed to write PowerDNS config: %w", err)
	}

	// Set proper ownership
	if err := exec.Command("chown", "pdns:pdns", configPath).Run(); err != nil {
		return fmt.Errorf("failed to set PowerDNS config ownership: %w", err)
	}

	return nil
}

// startPowerDNS starts and enables PowerDNS service
func (i *Installer) startPowerDNS() error {
	// Restart PowerDNS
	cmd := exec.Command("systemctl", "restart", "pdns")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to get more details from systemctl status
		statusCmd := exec.Command("systemctl", "status", "pdns", "--no-pager", "-l")
		statusOutput, _ := statusCmd.CombinedOutput()

		return fmt.Errorf("failed to restart PowerDNS: %w\nOutput: %s\nStatus: %s",
			err, string(output), string(statusOutput))
	}

	// Enable PowerDNS
	cmd = exec.Command("systemctl", "enable", "pdns")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable PowerDNS: %w", err)
	}

	// Verify service is actually running
	cmd = exec.Command("systemctl", "is-active", "pdns")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("PowerDNS service is not active after restart")
	}

	return nil
}

// ConfigureDNSRecords adds DNS records for all domains
func (i *Installer) ConfigureDNSRecords() error {
	// Connect to PowerDNS database
	connStr := fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable",
		pdnsUser, i.getOrGeneratePassword(pdnsUser), pdnsDatabaseName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to PowerDNS database: %w", err)
	}
	defer db.Close()

	// Configure DNS records for each domain
	for _, domain := range i.config.Domains {
		// Get DKIM public key for this domain
		dkimPublicKey, err := i.getDKIMPublicKeyForDomain(domain.Name)
		if err != nil {
			fmt.Printf("Warning: failed to get DKIM public key for %s: %v\n", domain.Name, err)
		}

		// Configure DNS records for this domain
		if err := i.configureDNSRecordsForDomain(db, &domain, dkimPublicKey); err != nil {
			return fmt.Errorf("failed to configure DNS for %s: %w", domain.Name, err)
		}
	}

	return nil
}

// configureDNSRecordsForDomain configures DNS records for a specific domain
func (i *Installer) configureDNSRecordsForDomain(db *sql.DB, domain *Domain, dkimPublicKey string) error {
	// Check if domain already exists
	var domainID int
	err := db.QueryRow("SELECT id FROM domains WHERE name = $1", domain.Name).Scan(&domainID)

	if err == sql.ErrNoRows {
		// Create domain with MASTER type (required for PowerDNS to be authoritative)
		err = db.QueryRow(
			"INSERT INTO domains (name, type) VALUES ($1, 'MASTER') RETURNING id",
			domain.Name,
		).Scan(&domainID)

		if err != nil {
			return fmt.Errorf("failed to create domain: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check domain: %w", err)
	}

	// Update existing domains to MASTER type if they are NATIVE
	_, err = db.Exec("UPDATE domains SET type = 'MASTER' WHERE name = $1 AND type = 'NATIVE'", domain.Name)
	if err != nil {
		return fmt.Errorf("failed to update domain type: %w", err)
	}

	// Delete existing records for this domain (for idempotency)
	_, err = db.Exec("DELETE FROM records WHERE domain_id = $1", domainID)
	if err != nil {
		return fmt.Errorf("failed to delete old records: %w", err)
	}

	// Get DNS records from config
	records := i.config.GetDNSRecords(domain, dkimPublicKey)

	// Insert records
	for _, record := range records {
		name := record.Name
		if name == "@" || name == "" {
			name = domain.Name
		} else if name != domain.Name {
			name = fmt.Sprintf("%s.%s", record.Name, domain.Name)
		}

		// Handle CNAME specially
		content := record.Content
		if record.Type == "CNAME" && !hasDot(content) {
			content = content + "."
		}

		_, err := db.Exec(
			"INSERT INTO records (domain_id, name, type, content, ttl, prio) VALUES ($1, $2, $3, $4, $5, $6)",
			domainID, name, record.Type, content, record.TTL, record.Priority,
		)

		if err != nil {
			return fmt.Errorf("failed to insert record %s %s: %w", record.Type, name, err)
		}
	}

	// Store domain in Hibana database
	if err := i.storeDomainInHibana(domain, domainID); err != nil {
		return fmt.Errorf("failed to store domain in Hibana database: %w", err)
	}

	return nil
}

// Domain type alias for config.Domain
type Domain = config.Domain

// getDKIMPublicKeyForDomain retrieves the DKIM public key for a specific domain
func (i *Installer) getDKIMPublicKeyForDomain(domainName string) (string, error) {
	keyPath := filepath.Join("/etc/opendkim/keys", domainName, "default.txt")

	data, err := os.ReadFile(keyPath)
	if err != nil {
		// Key might not exist yet, return empty string
		return "", nil
	}

	// Extract public key from DNS record format
	publicKey := extractDKIMPublicKey(string(data))
	return publicKey, nil
}

// getDKIMPublicKey retrieves the DKIM public key (for primary domain - backwards compatibility)
func (i *Installer) getDKIMPublicKey() (string, error) {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return "", fmt.Errorf("no primary domain configured")
	}
	return i.getDKIMPublicKeyForDomain(primaryDomain.Name)
}

// storeDomainInHibana stores domain info in Hibana database and creates domain user
func (i *Installer) storeDomainInHibana(domain *Domain, pdnsDomainID int) error {
	if i.db == nil {
		return fmt.Errorf("Hibana database not connected")
	}

	// Check if domain exists
	var id int
	err := i.db.QueryRow("SELECT id FROM domains WHERE name = $1", domain.Name).Scan(&id)

	if err == sql.ErrNoRows {
		// Insert domain
		_, err = i.db.Exec(
			"INSERT INTO domains (name, server_ip) VALUES ($1, $2)",
			domain.Name, i.config.ServerIP,
		)
		if err != nil {
			return fmt.Errorf("failed to insert domain: %w", err)
		}
	} else if err != nil {
		return err
	}

	// Create domain user if configuration is provided
	if domain.DomainUser != nil {
		fmt.Printf("\nSetting up domain user for %s\n", domain.Name)

		// Ensure hibana-domains group exists
		if err := i.EnsureHibanaDomainGroup(); err != nil {
			return fmt.Errorf("failed to ensure hibana-domains group: %w", err)
		}

		// Ensure master encryption key exists
		if err := i.EnsureMasterKey(); err != nil {
			return fmt.Errorf("failed to ensure master encryption key: %w", err)
		}

		// Get SSH public key from config
		sshPubKey := ""
		if domain.DomainUser.SSHPublicKey != "" {
			sshPubKey = domain.DomainUser.SSHPublicKey
		}

		// Create or update domain user
		result, err := i.CreateDomainUser(domain.Name, sshPubKey)
		if err != nil {
			return fmt.Errorf("failed to create domain user: %w", err)
		}

		// Store domain user in database
		if err := i.StoreDomainUser(domain.Name, result); err != nil {
			return fmt.Errorf("failed to store domain user: %w", err)
		}

		// If private key was generated, display it to the user
		if result.PrivateKey != "" {
			fmt.Println("\n" + strings.Repeat("=", 80))
			fmt.Println("IMPORTANT: SSH Private Key Generated")
			fmt.Println(strings.Repeat("=", 80))
			fmt.Printf("\nA new SSH key pair has been generated for user: %s\n", result.Username)
			fmt.Println("\nPrivate Key (save this securely - it will not be shown again):")
			fmt.Println(strings.Repeat("-", 80))
			fmt.Println(result.PrivateKey)
			fmt.Println(strings.Repeat("-", 80))
			fmt.Printf("\nTo connect to the server as this user:\n")
			fmt.Printf("  1. Save the private key to a file (e.g., ~/.ssh/%s_key)\n", result.Username)
			fmt.Printf("  2. Set proper permissions: chmod 600 ~/.ssh/%s_key\n", result.Username)
			fmt.Printf("  3. Connect: ssh -i ~/.ssh/%s_key %s@%s\n", result.Username, result.Username, i.config.ServerIP)
			fmt.Println("\n" + strings.Repeat("=", 80))
		}

		fmt.Printf("\nDomain user %s created successfully\n", result.Username)
		fmt.Printf("   - Access restricted to: /srv/%s/\n", domain.Name)
		fmt.Printf("   - Docker commands enabled for domain containers\n")
		fmt.Printf("   - SSH key authentication only (no password)\n\n")
	}

	return nil
}

// Helper functions

func hasDot(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '.'
}

func extractDKIMPublicKey(content string) string {
	// Simple extraction - parse p= value from DKIM record
	start := -1
	for j := 0; j < len(content)-2; j++ {
		if content[j] == 'p' && content[j+1] == '=' {
			start = j + 2
			break
		}
	}

	if start == -1 {
		return ""
	}

	end := len(content)
	for j := start; j < len(content); j++ {
		if content[j] == '"' || content[j] == ';' || content[j] == ')' {
			end = j
			break
		}
	}

	return content[start:end]
}

// configureResolvConf configures /etc/resolv.conf to use PowerDNS and fallback DNS
func (i *Installer) configureResolvConf() error {
	fmt.Println("  Configuring DNS resolution...")

	// Create resolv.conf with PowerDNS as primary and public DNS as fallback
	resolvConf := `# Generated by Hibana Stack
# Use local PowerDNS first
nameserver 127.0.0.1
# Fallback to public DNS servers
nameserver 8.8.8.8
nameserver 8.8.4.4
nameserver 1.1.1.1
`

	if err := os.WriteFile("/etc/resolv.conf", []byte(resolvConf), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/resolv.conf: %w", err)
	}

	// Make it immutable to prevent systemd from overwriting it
	cmd := exec.Command("chattr", "+i", "/etc/resolv.conf")
	if err := cmd.Run(); err != nil {
		// Just warn, not critical
		fmt.Println("  Warning: Could not make /etc/resolv.conf immutable")
	}

	fmt.Println("  DNS resolution configured")
	return nil
}

// ensureDomainsInHibana ensures all domains exist in Hibana database
func (i *Installer) ensureDomainsInHibana() error {
	if i.db == nil {
		return fmt.Errorf("Hibana database not connected")
	}

	for _, domain := range i.config.Domains {
		// Check if domain exists
		var id int
		err := i.db.QueryRow("SELECT id FROM domains WHERE name = $1", domain.Name).Scan(&id)

		if err == sql.ErrNoRows {
			// Insert domain
			fmt.Printf("  Storing domain %s in Hibana database...\n", domain.Name)
			_, err = i.db.Exec(
				"INSERT INTO domains (name, server_ip) VALUES ($1, $2)",
				domain.Name, i.config.ServerIP,
			)
			if err != nil {
				return fmt.Errorf("failed to insert domain %s: %w", domain.Name, err)
			}
			fmt.Printf("  Domain %s stored\n", domain.Name)
		} else if err != nil {
			return fmt.Errorf("failed to check domain %s: %w", domain.Name, err)
		} else {
			fmt.Printf("  Domain %s already exists in database\n", domain.Name)
		}
	}

	return nil
}

// openDNSPorts opens firewall ports for DNS (53/tcp and 53/udp)
func (i *Installer) openDNSPorts() error {
	fmt.Println("  Opening firewall ports for DNS (53/tcp, 53/udp)...")

	ports := []string{"53/tcp", "53/udp"}
	portDescriptions := map[string]string{
		"53/tcp": "DNS TCP",
		"53/udp": "DNS UDP",
	}

	return i.openPortsWithTracking("dns_setup", ports, portDescriptions)
}
