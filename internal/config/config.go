package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config represents the Hibana Stack configuration
type Config struct {
	PrimaryDomain string         `json:"primary_domain"`
	ServerIP      string         `json:"server_ip"`
	Subdomains    []string       `json:"subdomains"`
	EmailAccounts []EmailAccount `json:"email_accounts"`
	TestEmail     string         `json:"test_email,omitempty"`
}

// EmailAccount represents an email account configuration
type EmailAccount struct {
	Username string `json:"username"`
	Password string `json:"password"`
	FullName string `json:"full_name,omitempty"`
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.PrimaryDomain == "" {
		return fmt.Errorf("primary_domain is required")
	}

	if c.ServerIP == "" {
		return fmt.Errorf("server_ip is required")
	}

	if len(c.EmailAccounts) == 0 {
		return fmt.Errorf("at least one email account is required")
	}

	for i, acc := range c.EmailAccounts {
		if acc.Username == "" {
			return fmt.Errorf("email_accounts[%d]: username is required", i)
		}
		if acc.Password == "" {
			return fmt.Errorf("email_accounts[%d]: password is required", i)
		}
	}

	return nil
}

// GenerateSkeleton creates a skeleton configuration
func GenerateSkeleton() *Config {
	return &Config{
		PrimaryDomain: "example.com",
		ServerIP:      "YOUR_SERVER_IP",
		Subdomains: []string{
			"adm",
			"mail",
			"webmail",
			"www",
		},
		EmailAccounts: []EmailAccount{
			{
				Username: "admin",
				Password: "CHANGE_THIS_PASSWORD",
				FullName: "Administrator",
			},
		},
		TestEmail: "your-email@example.com",
	}
}

// GetDNSRecords generates DNS records for the configuration
func (c *Config) GetDNSRecords(dkimPublicKey string) []DNSRecord {
	// Get current timestamp for SOA serial (YYYYMMDDnn format)
	serial := time.Now().Format("2006010215") // YYYYMMDDnn format

	records := []DNSRecord{
		// SOA record (required for PowerDNS to be authoritative)
		{Type: "SOA", Name: "@", Content: fmt.Sprintf("ns1.%s. hostmaster.%s. %s 10800 3600 604800 3600", c.PrimaryDomain, c.PrimaryDomain, serial), TTL: 86400},

		// NS record (nameserver)
		{Type: "NS", Name: "@", Content: fmt.Sprintf("ns1.%s.", c.PrimaryDomain), TTL: 3600},

		// A record for nameserver
		{Type: "A", Name: "ns1", Content: c.ServerIP, TTL: 3600},

		// A record for domain
		{Type: "A", Name: "@", Content: c.ServerIP, TTL: 300},

		// A records for subdomains
		{Type: "A", Name: "mail", Content: c.ServerIP, TTL: 3600},
		{Type: "A", Name: "webmail", Content: c.ServerIP, TTL: 300},
		{Type: "A", Name: "adm", Content: c.ServerIP, TTL: 300},
		{Type: "A", Name: "www", Content: c.ServerIP, TTL: 300},

		// CNAME
		{Type: "CNAME", Name: "www", Content: c.PrimaryDomain, TTL: 300},

		// MX record
		{Type: "MX", Name: "@", Content: fmt.Sprintf("mail.%s", c.PrimaryDomain), Priority: 10, TTL: 14400},

		// SPF record
		{Type: "TXT", Name: "@", Content: fmt.Sprintf("v=spf1 ip4:%s -all", c.ServerIP), TTL: 14400},

		// DMARC record
		{Type: "TXT", Name: "_dmarc", Content: fmt.Sprintf("v=DMARC1; p=none; rua=mailto:dmarc@%s", c.PrimaryDomain), TTL: 3600},

		// CAA records for Let's Encrypt
		{Type: "CAA", Name: "@", Content: `0 issue "letsencrypt.org"`, TTL: 14400},
		{Type: "CAA", Name: "@", Content: `0 issuewild "letsencrypt.org"`, TTL: 14400},
	}

	// Add DKIM record if key is provided
	if dkimPublicKey != "" {
		records = append(records, DNSRecord{
			Type:    "TXT",
			Name:    "default._domainkey",
			Content: fmt.Sprintf("v=DKIM1; h=sha256; k=rsa; p=%s", dkimPublicKey),
			TTL:     14400,
		})
	}

	return records
}

// DNSRecord represents a DNS record
type DNSRecord struct {
	Type     string
	Name     string
	Content  string
	Priority int
	TTL      int
}
