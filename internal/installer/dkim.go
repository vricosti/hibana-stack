package installer

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SetupDKIM configures OpenDKIM for the domain
func (i *Installer) SetupDKIM() error {
	domain := i.config.PrimaryDomain
	selector := "default"

	// Create OpenDKIM directories
	keysDir := "/etc/opendkim/keys"
	domainDir := filepath.Join(keysDir, domain)

	if err := os.MkdirAll(domainDir, 0750); err != nil {
		return fmt.Errorf("failed to create OpenDKIM keys directory: %w", err)
	}

	// Generate DKIM keys if they don't exist
	privateKeyPath := filepath.Join(domainDir, selector+".private")
	publicKeyPath := filepath.Join(domainDir, selector+".txt")

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		if err := i.generateDKIMKeys(domain, selector, domainDir); err != nil {
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
	if err := i.storeDKIMKeys(domain, selector, privateKeyPath, publicKey); err != nil {
		return fmt.Errorf("failed to store DKIM keys: %w", err)
	}

	// Configure OpenDKIM
	if err := i.configureOpenDKIM(domain, selector); err != nil {
		return fmt.Errorf("failed to configure OpenDKIM: %w", err)
	}

	// Set proper permissions
	if err := exec.Command("chown", "-R", "opendkim:opendkim", keysDir).Run(); err != nil {
		return fmt.Errorf("failed to set OpenDKIM permissions: %w", err)
	}

	// Restart OpenDKIM
	if err := i.restartOpenDKIM(); err != nil {
		return fmt.Errorf("failed to restart OpenDKIM: %w", err)
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

// configureOpenDKIM creates OpenDKIM configuration files
func (i *Installer) configureOpenDKIM(domain, selector string) error {
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

	// Key table
	keyTablePath := "/etc/opendkim/key.table"
	keyTable := fmt.Sprintf("%s._domainkey.%s %s:%s:/etc/opendkim/keys/%s/%s.private\n",
		selector, domain, domain, selector, domain, selector)

	if err := i.appendOrCreateFile(keyTablePath, keyTable); err != nil {
		return fmt.Errorf("failed to write key.table: %w", err)
	}

	// Signing table
	signingTablePath := "/etc/opendkim/signing.table"
	signingTable := fmt.Sprintf("*@%s %s._domainkey.%s\n", domain, selector, domain)

	if err := i.appendOrCreateFile(signingTablePath, signingTable); err != nil {
		return fmt.Errorf("failed to write signing.table: %w", err)
	}

	// Trusted hosts
	trustedHostsPath := "/etc/opendkim/trusted.hosts"
	trustedHosts := fmt.Sprintf(`127.0.0.1
localhost
%s
*.%s
`, domain, domain)

	if err := i.appendOrCreateFile(trustedHostsPath, trustedHosts); err != nil {
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
		fmt.Println("  ⚠️  Warning: Could not add postfix to opendkim group")
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
func (i *Installer) storeDKIMKeys(domain, selector, privateKeyPath, publicKey string) error {
	if i.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Get domain ID
	var domainID int
	err := i.db.QueryRow("SELECT id FROM domains WHERE name = $1", domain).Scan(&domainID)
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
