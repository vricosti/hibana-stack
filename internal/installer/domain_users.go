package installer

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vricosti/hibana-stack/internal/utils"
	"golang.org/x/crypto/ssh"
)

// DomainUserResult contains the result of domain user creation
type DomainUserResult struct {
	Username   string
	PrivateKey string // Only set if auto-generated
	PublicKey  string
}

// CreateDomainUser creates a system user for a domain with SSH access
func (i *Installer) CreateDomainUser(domainName, sshPubKey string) (*DomainUserResult, error) {
	username := domainNameToUsername(domainName)
	result := &DomainUserResult{
		Username: username,
	}

	// Check if user already exists
	if _, err := exec.Command("id", username).Output(); err == nil {
		fmt.Printf("  • User %s already exists, updating...\n", username)
		return i.UpdateDomainUser(domainName, sshPubKey)
	}

	fmt.Printf("  • Creating domain user: %s\n", username)

	// Determine SSH key to use
	publicKey := sshPubKey
	var privateKey string

	if publicKey == "" {
		// Auto-generate SSH key pair
		fmt.Println("    • Generating SSH key pair...")
		pub, priv, err := generateSSHKeyPair(username, domainName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SSH key pair: %w", err)
		}
		publicKey = pub
		privateKey = priv
		result.PrivateKey = privateKey
		fmt.Println("      ✓ SSH key pair generated")
	} else {
		// Validate provided SSH key
		if err := validateSSHPublicKey(publicKey); err != nil {
			return nil, fmt.Errorf("invalid SSH public key: %w", err)
		}
		fmt.Println("    ✓ SSH public key validated")
	}

	result.PublicKey = publicKey

	// Create domain directory in /srv/ first (will be the home directory)
	// Example: linkedingue.com -> /srv/linkedingue-com
	domainDir := utils.DomainToPath(domainName)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create domain directory: %w", err)
	}
	fmt.Println("    ✓ Domain directory created")

	// Create user with /srv/domaine.com/ as home directory
	createCmd := exec.Command("useradd",
		"-d", domainDir,         // Home directory in /srv/
		"-s", "/bin/bash",       // Set shell
		"-g", "hibana-domains",  // Primary group
		"-c", fmt.Sprintf("Domain user for %s", domainName), // Comment
		username,
	)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create user: %w, output: %s", err, string(output))
	}
	fmt.Println("    ✓ User created with home in /srv/")

	// Set ownership of domain directory to the user
	if err := i.setDomainDirectoryOwnership(username, domainDir); err != nil {
		return nil, fmt.Errorf("failed to set domain directory ownership: %w", err)
	}
	fmt.Println("    ✓ Domain directory ownership configured")

	// Disable password authentication
	if err := i.disablePasswordAuth(username); err != nil {
		return nil, fmt.Errorf("failed to disable password auth: %w", err)
	}
	fmt.Println("    ✓ Password authentication disabled")

	// Setup SSH key
	if err := i.setupSSHKey(username, publicKey); err != nil {
		return nil, fmt.Errorf("failed to setup SSH key: %w", err)
	}
	fmt.Println("    ✓ SSH key configured")

	// Add user to docker group
	if err := i.addUserToDockerGroup(username); err != nil {
		return nil, fmt.Errorf("failed to add to docker group: %w", err)
	}
	fmt.Println("    ✓ Added to docker group")

	// Configure sudoers for limited docker commands
	if err := i.configureDomainUserSudoers(username, domainName); err != nil {
		return nil, fmt.Errorf("failed to configure sudoers: %w", err)
	}
	fmt.Println("    ✓ Sudoers configured for docker commands")

	return result, nil
}

// UpdateDomainUser updates an existing domain user
func (i *Installer) UpdateDomainUser(domainName, sshPubKey string) (*DomainUserResult, error) {
	username := domainNameToUsername(domainName)
	result := &DomainUserResult{
		Username: username,
	}

	fmt.Printf("  • Updating domain user: %s\n", username)

	// If SSH key provided, update it
	if sshPubKey != "" {
		// Validate SSH key
		if err := validateSSHPublicKey(sshPubKey); err != nil {
			return nil, fmt.Errorf("invalid SSH public key: %w", err)
		}

		if err := i.setupSSHKey(username, sshPubKey); err != nil {
			return nil, fmt.Errorf("failed to update SSH key: %w", err)
		}
		fmt.Println("    ✓ SSH key updated")
		result.PublicKey = sshPubKey
	}

	// Ensure password auth is disabled
	if err := i.disablePasswordAuth(username); err != nil {
		return nil, fmt.Errorf("failed to disable password auth: %w", err)
	}

	// Ensure domain directory exists with correct permissions
	if err := i.setupDomainDirectory(username, domainName); err != nil {
		return nil, fmt.Errorf("failed to setup domain directory: %w", err)
	}
	fmt.Println("    ✓ Domain directory verified")

	// Ensure user is in docker group
	if err := i.addUserToDockerGroup(username); err != nil {
		return nil, fmt.Errorf("failed to add to docker group: %w", err)
	}

	// Update sudoers configuration
	if err := i.configureDomainUserSudoers(username, domainName); err != nil {
		return nil, fmt.Errorf("failed to configure sudoers: %w", err)
	}
	fmt.Println("    ✓ Sudoers configuration updated")

	return result, nil
}

// disablePasswordAuth locks the user account to disable password authentication
func (i *Installer) disablePasswordAuth(username string) error {
	cmd := exec.Command("usermod", "-L", username)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("usermod -L failed: %w, output: %s", err, string(output))
	}
	return nil
}

// setDomainDirectoryOwnership sets ownership and strict permissions on domain directory
func (i *Installer) setDomainDirectoryOwnership(username, domainDir string) error {
	// Set ownership to domain user with hibana-domains group
	cmd := exec.Command("chown", "-R", username+":hibana-domains", domainDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown failed: %w, output: %s", err, string(output))
	}

	// Set strict permissions (700 - only owner can access)
	if err := os.Chmod(domainDir, 0700); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	return nil
}

// setupDomainDirectory creates the /srv/domain-tld directory with proper permissions
// Example: linkedingue.com -> /srv/linkedingue-com
func (i *Installer) setupDomainDirectory(username, domainName string) error {
	domainDir := utils.DomainToPath(domainName)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("failed to create domain directory: %w", err)
	}

	return i.setDomainDirectoryOwnership(username, domainDir)
}

// addUserToDockerGroup adds the user to the docker group
func (i *Installer) addUserToDockerGroup(username string) error {
	// Check if user is already in docker group
	output, err := exec.Command("groups", username).Output()
	if err != nil {
		return fmt.Errorf("groups command failed: %w", err)
	}

	if strings.Contains(string(output), "docker") {
		return nil // Already in docker group
	}

	cmd := exec.Command("usermod", "-aG", "docker", username)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("usermod failed: %w, output: %s", err, string(output))
	}

	return nil
}

// configureDomainUserSudoers configures sudoers for limited docker commands
func (i *Installer) configureDomainUserSudoers(username, domainName string) error {
	sudoersFile := fmt.Sprintf("/etc/sudoers.d/hibana-domain-%s", username)
	systemName := utils.DomainToSystemName(domainName)

	// Build sudoers content with limited docker commands
	content := fmt.Sprintf(`# Hibana domain user sudoers for %s
# Allow docker commands only for domain containers
%s ALL=(ALL) NOPASSWD: /usr/bin/docker ps
%s ALL=(ALL) NOPASSWD: /usr/bin/docker ps -a
%s ALL=(ALL) NOPASSWD: /usr/bin/docker logs *
%s ALL=(ALL) NOPASSWD: /usr/bin/docker inspect *
%s ALL=(ALL) NOPASSWD: /usr/bin/docker restart *%s*
%s ALL=(ALL) NOPASSWD: /usr/bin/docker stop *%s*
%s ALL=(ALL) NOPASSWD: /usr/bin/docker start *%s*
%s ALL=(ALL) NOPASSWD: /usr/bin/docker exec -it *%s* *
%s ALL=(ALL) NOPASSWD: /usr/bin/docker compose -f /srv/%s/* up -d
%s ALL=(ALL) NOPASSWD: /usr/bin/docker compose -f /srv/%s/* down
%s ALL=(ALL) NOPASSWD: /usr/bin/docker compose -f /srv/%s/* restart
%s ALL=(ALL) NOPASSWD: /usr/bin/docker compose -f /srv/%s/* logs
%s ALL=(ALL) NOPASSWD: /usr/bin/docker compose -f /srv/%s/* ps
`,
		domainName,
		username, username, username, username,
		username, systemName,
		username, systemName,
		username, systemName,
		username, systemName,
		username, systemName,
		username, systemName,
		username, systemName,
		username, systemName,
		username, systemName,
	)

	// Write sudoers file
	if err := os.WriteFile(sudoersFile, []byte(content), 0440); err != nil {
		return fmt.Errorf("failed to write sudoers file: %w", err)
	}

	// Verify sudoers file syntax
	cmd := exec.Command("visudo", "-cf", sudoersFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Remove invalid file
		os.Remove(sudoersFile)
		return fmt.Errorf("invalid sudoers syntax: %w, output: %s", err, string(output))
	}

	return nil
}

// domainNameToUsername converts a domain name to a valid username
// Example: linkedingue.com -> linkedingue-com
func domainNameToUsername(domain string) string {
	return utils.DomainToSystemName(domain)
}

// validateSSHPublicKey validates an SSH public key format
func validateSSHPublicKey(pubKey string) error {
	pubKey = strings.TrimSpace(pubKey)

	if pubKey == "" {
		return fmt.Errorf("empty SSH public key")
	}

	// Parse the SSH public key
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return fmt.Errorf("invalid SSH public key format: %w", err)
	}

	return nil
}

// generateSSHKeyPair generates an ED25519 SSH key pair using ssh-keygen
func generateSSHKeyPair(username, domain string) (publicKey, privateKey string, err error) {
	// Create temporary directory for key generation
	tmpDir, err := ioutil.TempDir("", "hibana-ssh-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	keyPath := filepath.Join(tmpDir, "id_ed25519")
	comment := fmt.Sprintf("%s@%s", username, domain)

	// Generate ED25519 key pair using ssh-keygen
	cmd := exec.Command("ssh-keygen",
		"-t", "ed25519",
		"-f", keyPath,
		"-N", "",        // No passphrase
		"-C", comment,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("ssh-keygen failed: %w, output: %s", err, string(output))
	}

	// Read private key
	privKeyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read private key: %w", err)
	}
	privateKey = string(privKeyData)

	// Read public key
	pubKeyData, err := ioutil.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", "", fmt.Errorf("failed to read public key: %w", err)
	}
	publicKey = string(pubKeyData)

	return publicKey, privateKey, nil
}

// StoreDomainUser stores domain user information in the database
func (i *Installer) StoreDomainUser(domainName string, result *DomainUserResult) error {
	if i.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Get domain ID
	var domainID int
	err := i.db.QueryRow("SELECT id FROM domains WHERE name = $1", domainName).Scan(&domainID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	// Encrypt private key if present
	var encryptedPrivateKey sql.NullString
	if result.PrivateKey != "" {
		encrypted, err := i.EncryptPrivateKey(result.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt private key: %w", err)
		}
		encryptedPrivateKey = sql.NullString{String: encrypted, Valid: true}
	}

	// Check if user already exists
	var existingID int
	err = i.db.QueryRow("SELECT id FROM domain_users WHERE domain_id = $1", domainID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new user
		_, err = i.db.Exec(
			"INSERT INTO domain_users (domain_id, username, ssh_public_key, ssh_private_key_encrypted) VALUES ($1, $2, $3, $4)",
			domainID, result.Username, result.PublicKey, encryptedPrivateKey,
		)
		if err != nil {
			return fmt.Errorf("failed to insert domain user: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check existing domain user: %w", err)
	} else {
		// Update existing user
		_, err = i.db.Exec(
			"UPDATE domain_users SET username = $1, ssh_public_key = $2, ssh_private_key_encrypted = $3, updated_at = NOW() WHERE id = $4",
			result.Username, result.PublicKey, encryptedPrivateKey, existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update domain user: %w", err)
		}
	}

	return nil
}
