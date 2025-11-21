package installer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	masterKeyPath = "/etc/hibana/secrets/master.key"
	keySize       = 32 // AES-256
)

// EnsureMasterKey creates or loads the master encryption key
func (i *Installer) EnsureMasterKey() error {
	// Create secrets directory if it doesn't exist
	secretsDir := filepath.Dir(masterKeyPath)
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return fmt.Errorf("failed to create secrets directory: %w", err)
	}

	// Check if master key already exists
	if _, err := os.Stat(masterKeyPath); err == nil {
		// Key exists, validate it
		key, err := os.ReadFile(masterKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read master key: %w", err)
		}
		if len(key) != keySize {
			return fmt.Errorf("invalid master key size: expected %d, got %d", keySize, len(key))
		}
		return nil
	}

	// Generate new master key
	fmt.Println("  • Generating master encryption key...")
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}

	// Write key to file with restrictive permissions
	if err := os.WriteFile(masterKeyPath, key, 0600); err != nil {
		return fmt.Errorf("failed to write master key: %w", err)
	}

	// Ensure root ownership
	if err := os.Chown(masterKeyPath, 0, 0); err != nil {
		return fmt.Errorf("failed to set master key ownership: %w", err)
	}

	fmt.Println("    ✓ Master key generated and secured")
	return nil
}

// EncryptPrivateKey encrypts a private key using AES-256-GCM
func (i *Installer) EncryptPrivateKey(plaintext string) (string, error) {
	// Read master key
	key, err := os.ReadFile(masterKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read master key: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPrivateKey decrypts a private key using AES-256-GCM
func (i *Installer) DecryptPrivateKey(ciphertext string) (string, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Read master key
	key, err := os.ReadFile(masterKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read master key: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}
