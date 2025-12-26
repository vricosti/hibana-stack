package services

import (
	"database/sql"
	"fmt"
)

// ConfigService handles configuration operations
type ConfigService struct {
	db *sql.DB
}

// NewConfigService creates a new config service
func NewConfigService(db *sql.DB) *ConfigService {
	return &ConfigService{db: db}
}

// DNSProviderConfig represents the DNS provider configuration
type DNSProviderConfig struct {
	Type string `json:"type"` // "local", "external", "manual"
	Name string `json:"name"` // "powerdns", "hostinger", "cloudflare", "ovhcloud"
}

// GetDNSProvider returns the DNS provider configuration
func (s *ConfigService) GetDNSProvider() (*DNSProviderConfig, error) {
	config := &DNSProviderConfig{
		Type: "local",
		Name: "powerdns",
	}

	// Get type
	var value string
	err := s.db.QueryRow("SELECT value FROM configuration WHERE key = $1", "dns_provider_type").Scan(&value)
	if err == nil {
		config.Type = value
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get dns_provider_type: %w", err)
	}

	// Get name
	err = s.db.QueryRow("SELECT value FROM configuration WHERE key = $1", "dns_provider_name").Scan(&value)
	if err == nil {
		config.Name = value
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get dns_provider_name: %w", err)
	}

	return config, nil
}

// SetDNSProvider sets the DNS provider configuration
func (s *ConfigService) SetDNSProvider(config *DNSProviderConfig) error {
	// Upsert type
	_, err := s.db.Exec(`
		INSERT INTO configuration (key, value, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP
	`, "dns_provider_type", config.Type)
	if err != nil {
		return fmt.Errorf("failed to set dns_provider_type: %w", err)
	}

	// Upsert name
	_, err = s.db.Exec(`
		INSERT INTO configuration (key, value, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP
	`, "dns_provider_name", config.Name)
	if err != nil {
		return fmt.Errorf("failed to set dns_provider_name: %w", err)
	}

	return nil
}

// GetConfig returns a configuration value by key
func (s *ConfigService) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM configuration WHERE key = $1", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get config %s: %w", key, err)
	}
	return value, nil
}

// SetConfig sets a configuration value
func (s *ConfigService) SetConfig(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO configuration (key, value, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}
