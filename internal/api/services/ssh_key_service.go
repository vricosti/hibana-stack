package services

import (
	"crypto/ed25519"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/utils"
	"golang.org/x/crypto/ssh"
)

// SSHKeyService handles SSH key operations
type SSHKeyService struct {
	db *sql.DB
}

// NewSSHKeyService creates a new SSH key service
func NewSSHKeyService(db *sql.DB) *SSHKeyService {
	return &SSHKeyService{db: db}
}

// ListKeys lists all SSH keys for a domain
func (s *SSHKeyService) ListKeys(domainName string) ([]models.SSHKey, error) {
	// Get domain info and username
	var domainID int
	var username string
	err := s.db.QueryRow(`
		SELECT d.id, du.username
		FROM domains d
		LEFT JOIN domain_users du ON d.id = du.domain_id
		WHERE d.name = $1
	`, domainName).Scan(&domainID, &username)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("domain not found or no domain user configured")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get domain info: %w", err)
	}

	// Read authorized_keys file
	domainPath := utils.DomainToPath(domainName)
	authorizedKeysPath := filepath.Join(domainPath, ".ssh", "authorized_keys")
	content, err := os.ReadFile(authorizedKeysPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.SSHKey{}, nil // No keys yet
		}
		return nil, fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	// Parse keys
	lines := strings.Split(string(content), "\n")
	var keys []models.SSHKey
	id := 1

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fingerprint, err := calculateFingerprint(line)
		if err != nil {
			// Skip invalid keys
			continue
		}

		// Try to get label from database
		var label string
		var createdAt sql.NullTime
		_ = s.db.QueryRow(`
			SELECT label, created_at
			FROM ssh_keys
			WHERE domain_id = $1 AND fingerprint = $2
		`, domainID, fingerprint).Scan(&label, &createdAt)

		key := models.SSHKey{
			ID:          id,
			DomainID:    domainID,
			DomainName:  domainName,
			Username:    username,
			PublicKey:   line,
			Fingerprint: fingerprint,
			Label:       label,
		}

		if createdAt.Valid {
			key.CreatedAt = createdAt.Time
		}

		keys = append(keys, key)
		id++
	}

	return keys, nil
}

// AddKey adds a new SSH key for a domain
func (s *SSHKeyService) AddKey(domainName, publicKey, label string) error {
	// Validate SSH key
	publicKey = strings.TrimSpace(publicKey)
	if err := validateSSHKey(publicKey); err != nil {
		return fmt.Errorf("invalid SSH key: %w", err)
	}

	// Get domain info and username
	var domainID int
	var username string
	err := s.db.QueryRow(`
		SELECT d.id, du.username
		FROM domains d
		LEFT JOIN domain_users du ON d.id = du.domain_id
		WHERE d.name = $1
	`, domainName).Scan(&domainID, &username)

	if err == sql.ErrNoRows {
		return fmt.Errorf("domain not found or no domain user configured")
	}
	if err != nil {
		return fmt.Errorf("failed to get domain info: %w", err)
	}

	// Calculate fingerprint
	fingerprint, err := calculateFingerprint(publicKey)
	if err != nil {
		return fmt.Errorf("failed to calculate fingerprint: %w", err)
	}

	// Check if key already exists
	domainPath := utils.DomainToPath(domainName)
	authorizedKeysPath := filepath.Join(domainPath, ".ssh", "authorized_keys")
	if content, err := os.ReadFile(authorizedKeysPath); err == nil {
		if strings.Contains(string(content), publicKey) {
			return fmt.Errorf("SSH key already exists")
		}
	}

	// Create .ssh directory if it doesn't exist
	sshDir := filepath.Join(domainPath, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Append key to authorized_keys
	f, err := os.OpenFile(authorizedKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(publicKey + "\n"); err != nil {
		return fmt.Errorf("failed to write SSH key: %w", err)
	}

	// Set ownership
	if err := setSSHOwnership(username, sshDir); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	// Store metadata in database
	_, err = s.db.Exec(`
		INSERT INTO ssh_keys (domain_id, fingerprint, label, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (domain_id, fingerprint) DO UPDATE SET label = $3
	`, domainID, fingerprint, label)

	if err != nil {
		return fmt.Errorf("failed to store key metadata: %w", err)
	}

	return nil
}

// RemoveKey removes an SSH key from a domain
func (s *SSHKeyService) RemoveKey(domainName, fingerprint string) error {
	// Get domain info
	var domainID int
	var username string
	err := s.db.QueryRow(`
		SELECT d.id, du.username
		FROM domains d
		LEFT JOIN domain_users du ON d.id = du.domain_id
		WHERE d.name = $1
	`, domainName).Scan(&domainID, &username)

	if err == sql.ErrNoRows {
		return fmt.Errorf("domain not found or no domain user configured")
	}
	if err != nil {
		return fmt.Errorf("failed to get domain info: %w", err)
	}

	// Read authorized_keys file
	domainPath := utils.DomainToPath(domainName)
	authorizedKeysPath := filepath.Join(domainPath, ".ssh", "authorized_keys")
	content, err := os.ReadFile(authorizedKeysPath)
	if err != nil {
		return fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	// Filter out the key with matching fingerprint
	lines := strings.Split(string(content), "\n")
	var newLines []string
	removed := false

	for _, line := range lines {
		origLine := line
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			newLines = append(newLines, origLine)
			continue
		}

		keyFingerprint, err := calculateFingerprint(line)
		if err != nil {
			// Keep lines we can't parse (invalid keys)
			newLines = append(newLines, origLine)
			continue
		}

		if keyFingerprint == fingerprint {
			// Skip this key (remove it)
			removed = true
			continue
		}

		newLines = append(newLines, origLine)
	}

	if !removed {
		return fmt.Errorf("SSH key with fingerprint %s not found", fingerprint)
	}

	// Write back to file
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(authorizedKeysPath, []byte(newContent), 0600); err != nil {
		return fmt.Errorf("failed to write authorized_keys: %w", err)
	}

	// Set ownership
	sshDir := filepath.Join(domainPath, ".ssh")
	if err := setSSHOwnership(username, sshDir); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	// Remove from database
	_, err = s.db.Exec("DELETE FROM ssh_keys WHERE domain_id = $1 AND fingerprint = $2", domainID, fingerprint)
	if err != nil {
		return fmt.Errorf("failed to remove key metadata: %w", err)
	}

	return nil
}

// validateSSHKey validates an SSH public key
func validateSSHKey(pubKey string) error {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return fmt.Errorf("empty SSH public key")
	}

	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return fmt.Errorf("invalid SSH public key format: %w", err)
	}

	return nil
}

// calculateFingerprint calculates the SSH key fingerprint (SHA256)
func calculateFingerprint(pubKey string) (string, error) {
	pubKey = strings.TrimSpace(pubKey)

	// Parse the SSH public key
	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return "", err
	}

	// Calculate SHA256 fingerprint
	hash := sha256.Sum256(parsedKey.Marshal())
	fingerprint := "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])

	return fingerprint, nil
}

// calculateFingerprintMD5 calculates the MD5 fingerprint (legacy format)
func calculateFingerprintMD5(pubKey string) (string, error) {
	pubKey = strings.TrimSpace(pubKey)

	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return "", err
	}

	hash := md5.Sum(parsedKey.Marshal())
	fingerprint := fmt.Sprintf("MD5:%x", hash)
	fingerprint = strings.Replace(fingerprint, "MD5:", "MD5:", 1)

	// Format as xx:xx:xx:xx...
	var parts []string
	for i := 0; i < len(hash); i++ {
		parts = append(parts, fmt.Sprintf("%02x", hash[i]))
	}
	return "MD5:" + strings.Join(parts, ":"), nil
}

// GenerateKeyPair generates a new ED25519 SSH key pair
func (s *SSHKeyService) GenerateKeyPair() (publicKey, privateKey string, err error) {
	// Generate ED25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Convert to SSH format for public key
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Format public key
	publicKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPubKey)))

	// Format private key in OpenSSH format
	pemBlock, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key: %w", err)
	}

	privateKeyPEM := pem.EncodeToMemory(pemBlock)
	privateKeyStr := string(privateKeyPEM)

	return publicKeyStr, privateKeyStr, nil
}

// setSSHOwnership sets ownership for SSH directory and files
func setSSHOwnership(username, sshDir string) error {
	// Get the parent directory (e.g., /srv/domainname)
	parentDir := filepath.Dir(sshDir)

	// Get UID and GID from parent directory
	fileInfo, err := os.Stat(parentDir)
	if err != nil {
		return fmt.Errorf("failed to stat parent directory %s: %w", parentDir, err)
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to get syscall.Stat_t for %s", parentDir)
	}

	uid := stat.Uid
	gid := stat.Gid

	// Use numeric UID:GID
	cmd := exec.Command("chown", "-R", strconv.Itoa(int(uid))+":"+strconv.Itoa(int(gid)), sshDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown failed: %w, output: %s", err, string(output))
	}

	// Ensure correct permissions
	if err := os.Chmod(sshDir, 0700); err != nil {
		return fmt.Errorf("chmod .ssh failed: %w", err)
	}

	authorizedKeysFile := filepath.Join(sshDir, "authorized_keys")
	if _, err := os.Stat(authorizedKeysFile); err == nil {
		if err := os.Chmod(authorizedKeysFile, 0600); err != nil {
			return fmt.Errorf("chmod authorized_keys failed: %w", err)
		}
	}

	return nil
}

// AddExternalKey adds a private key for external service access (e.g., GitHub)
func (s *SSHKeyService) AddExternalKey(domainName string, req *models.ExternalSSHKeyRequest) error {
	// Get domain info and username
	var username string
	err := s.db.QueryRow(`
		SELECT du.username
		FROM domains d
		LEFT JOIN domain_users du ON d.id = du.domain_id
		WHERE d.name = $1
	`, domainName).Scan(&username)

	if err == sql.ErrNoRows {
		return fmt.Errorf("domain not found or no domain user configured")
	}
	if err != nil {
		return fmt.Errorf("failed to get domain info: %w", err)
	}

	domainPath := utils.DomainToPath(domainName)
	sshDir := filepath.Join(domainPath, ".ssh")

	// Create .ssh directory if it doesn't exist
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Generate a safe filename from the label
	safeLabel := strings.ReplaceAll(req.Label, " ", "_")
	safeLabel = strings.ReplaceAll(safeLabel, "/", "_")
	keyFileName := fmt.Sprintf("id_%s", safeLabel)
	keyFilePath := filepath.Join(sshDir, keyFileName)

	// Write private key
	if err := os.WriteFile(keyFilePath, []byte(req.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Update SSH config file
	configPath := filepath.Join(sshDir, "config")
	if err := s.updateSSHConfig(configPath, req, keyFileName); err != nil {
		// Cleanup key file on config error
		os.Remove(keyFilePath)
		return fmt.Errorf("failed to update SSH config: %w", err)
	}

	// Set ownership
	if err := setSSHOwnership(username, sshDir); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	return nil
}

// updateSSHConfig adds or updates an entry in the SSH config file
func (s *SSHKeyService) updateSSHConfig(configPath string, req *models.ExternalSSHKeyRequest, keyFileName string) error {
	hostname := req.Hostname
	if hostname == "" {
		hostname = req.Host
	}

	// Build config entry
	entry := fmt.Sprintf(`
# %s
Host %s
    HostName %s
    User %s
    IdentityFile ~/.ssh/%s
    IdentitiesOnly yes
`, req.Label, req.Host, hostname, req.User, keyFileName)

	// Read existing config
	existingConfig, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing config: %w", err)
	}

	// Check if host already exists in config
	if strings.Contains(string(existingConfig), fmt.Sprintf("Host %s", req.Host)) {
		return fmt.Errorf("SSH config entry for host %s already exists", req.Host)
	}

	// Append new entry
	newConfig := string(existingConfig) + entry

	// Write config
	if err := os.WriteFile(configPath, []byte(newConfig), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
