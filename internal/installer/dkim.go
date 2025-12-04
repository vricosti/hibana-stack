package installer

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SetupDKIM configures OpenDKIM for all domains
func (i *Installer) SetupDKIM() error {
	// Setup DKIM for each domain
	for _, domain := range i.config.Domains {
		if err := i.setupDKIMForDomain(domain.Name); err != nil {
			return fmt.Errorf("failed to setup DKIM for %s: %w", domain.Name, err)
		}
	}

	// Configure OpenDKIM with all domains
	if err := i.configureOpenDKIMMultiDomain(); err != nil {
		return fmt.Errorf("failed to configure OpenDKIM: %w", err)
	}

	// Set proper permissions
	keysDir := "/etc/opendkim/keys"
	if err := exec.Command("chown", "-R", "opendkim:opendkim", keysDir).Run(); err != nil {
		return fmt.Errorf("failed to set OpenDKIM permissions: %w", err)
	}

	// Restart OpenDKIM
	if err := i.restartOpenDKIM(); err != nil {
		return fmt.Errorf("failed to restart OpenDKIM: %w", err)
	}

	return nil
}

// setupDKIMForDomain sets up DKIM keys for a specific domain
func (i *Installer) setupDKIMForDomain(domainName string) error {
	selector := "default"

	// Create OpenDKIM directories
	keysDir := "/etc/opendkim/keys"
	domainDir := filepath.Join(keysDir, domainName)

	if err := os.MkdirAll(domainDir, 0750); err != nil {
		return fmt.Errorf("failed to create OpenDKIM keys directory: %w", err)
	}

	// Generate DKIM keys if they don't exist
	privateKeyPath := filepath.Join(domainDir, selector+".private")
	publicKeyPath := filepath.Join(domainDir, selector+".txt")

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		if err := i.generateDKIMKeys(domainName, selector, domainDir); err != nil {
			return fmt.Errorf("failed to generate DKIM keys: %w", err)
		}
	}

	// Read public key
	publicKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read DKIM public key: %w", err)
	}

	publicKey := extractDKIMPublicKey(string(publicKeyData))

	// Store DKIM keys in database
	if err := i.storeDKIMKeys(domainName, selector, privateKeyPath, publicKey); err != nil {
		return fmt.Errorf("failed to store DKIM keys: %w", err)
	}

	return nil
}

// generateDKIMKeys generates DKIM key pair
func (i *Installer) generateDKIMKeys(domain, selector, outputDir string) error {
	cmd := exec.Command(
		"opendkim-genkey",
		"-b", "2048",
		"-d", domain,
		"-s", selector,
		"-D", outputDir,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("opendkim-genkey failed: %w\n%s", err, output)
	}

	return nil
}

// configureOpenDKIMMultiDomain creates OpenDKIM configuration files for all domains
func (i *Installer) configureOpenDKIMMultiDomain() error {
	selector := "default"

	// Main configuration
	opendkimConf := `# OpenDKIM Configuration
Syslog                  yes
SyslogSuccess           yes
LogWhy                  yes

Canonicalization        relaxed/simple
Mode                    sv
SubDomains              no

AutoRestart             yes
AutoRestartRate         10/1M
DNSTimeout              5
SignatureAlgorithm      rsa-sha256

UserID                  opendkim:opendkim
UMask                   007

Socket                  local:/run/opendkim/opendkim.sock
PidFile                 /run/opendkim/opendkim.pid

KeyTable                /etc/opendkim/key.table
SigningTable            /etc/opendkim/signing.table
ExternalIgnoreList      /etc/opendkim/trusted.hosts
InternalHosts           /etc/opendkim/trusted.hosts
`

	if err := os.WriteFile("/etc/opendkim.conf", []byte(opendkimConf), 0644); err != nil {
		return fmt.Errorf("failed to write opendkim.conf: %w", err)
	}

	// Key table - entries for all domains
	keyTablePath := "/etc/opendkim/key.table"
	var keyTable string
	for _, domain := range i.config.Domains {
		keyTable += fmt.Sprintf("%s._domainkey.%s %s:%s:/etc/opendkim/keys/%s/%s.private\n",
			selector, domain.Name, domain.Name, selector, domain.Name, selector)
	}

	if err := os.WriteFile(keyTablePath, []byte(keyTable), 0644); err != nil {
		return fmt.Errorf("failed to write key.table: %w", err)
	}

	// Signing table - entries for all domains
	signingTablePath := "/etc/opendkim/signing.table"
	var signingTable string
	for _, domain := range i.config.Domains {
		signingTable += fmt.Sprintf("*@%s %s._domainkey.%s\n", domain.Name, selector, domain.Name)
	}

	if err := os.WriteFile(signingTablePath, []byte(signingTable), 0644); err != nil {
		return fmt.Errorf("failed to write signing.table: %w", err)
	}

	// Trusted hosts - entries for all domains
	trustedHostsPath := "/etc/opendkim/trusted.hosts"
	trustedHosts := "127.0.0.1\nlocalhost\n"
	for _, domain := range i.config.Domains {
		trustedHosts += fmt.Sprintf("%s\n*.%s\n", domain.Name, domain.Name)
	}

	if err := os.WriteFile(trustedHostsPath, []byte(trustedHosts), 0644); err != nil {
		return fmt.Errorf("failed to write trusted.hosts: %w", err)
	}

	return nil
}

// restartOpenDKIM restarts the OpenDKIM service
func (i *Installer) restartOpenDKIM() error {
	// Create runtime directory
	runDir := "/run/opendkim"
	if err := os.MkdirAll(runDir, 0750); err != nil {
		return fmt.Errorf("failed to create %s: %w", runDir, err)
	}

	// Set ownership
	if err := exec.Command("chown", "opendkim:opendkim", runDir).Run(); err != nil {
		return fmt.Errorf("failed to set ownership on %s: %w", runDir, err)
	}

	// Add postfix user to opendkim group so it can access the socket
	if err := exec.Command("usermod", "-a", "-G", "opendkim", "postfix").Run(); err != nil {
		fmt.Println("  Warning: Could not add postfix to opendkim group")
	}

	// Restart service
	cmd := exec.Command("systemctl", "restart", "opendkim")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart opendkim: %w\nOutput: %s", err, string(output))
	}

	// Enable service
	cmd = exec.Command("systemctl", "enable", "opendkim")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable opendkim: %w", err)
	}

	// Verify service is running
	cmd = exec.Command("systemctl", "is-active", "opendkim")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opendkim service is not active after restart")
	}

	return nil
}

// storeDKIMKeys stores DKIM keys in the Hibana database
func (i *Installer) storeDKIMKeys(domainName, selector, privateKeyPath, publicKey string) error {
	if i.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Get domain ID
	var domainID int
	err := i.db.QueryRow("SELECT id FROM domains WHERE name = $1", domainName).Scan(&domainID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	// Read private key
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	// Check if DKIM key already exists
	var existingID int
	err = i.db.QueryRow(
		"SELECT id FROM dkim_keys WHERE domain_id = $1 AND selector = $2",
		domainID, selector,
	).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new key
		_, err = i.db.Exec(
			"INSERT INTO dkim_keys (domain_id, selector, private_key, public_key) VALUES ($1, $2, $3, $4)",
			domainID, selector, string(privateKey), publicKey,
		)
		return err
	} else if err != nil {
		return err
	}

	// Update existing key
	_, err = i.db.Exec(
		"UPDATE dkim_keys SET private_key = $1, public_key = $2 WHERE id = $3",
		string(privateKey), publicKey, existingID,
	)

	return err
}

// appendOrCreateFile appends content to a file or creates it if it doesn't exist
func (i *Installer) appendOrCreateFile(path, content string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.WriteFile(path, []byte(content), 0644)
	}

	// Read existing content
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Check if content already exists
	if strings.Contains(string(existing), content) {
		return nil
	}

	// Append content
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}
