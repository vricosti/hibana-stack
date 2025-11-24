package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the Hibana Stack configuration
type Config struct {
	PrimaryDomain string              `yaml:"primary_domain"`
	ServerIP      string              `yaml:"server_ip"`
	DNSProvider   *DNSProviderConfig  `yaml:"dns_provider,omitempty"`
	SystemUsers   []SystemUser        `yaml:"system_users,omitempty"`
	DomainUser    *DomainUserConfig   `yaml:"domain_user,omitempty"`
	Subdomains    []Subdomain         `yaml:"subdomains"`
	WebAdmin      *WebAdminConfig     `yaml:"webadmin,omitempty"`
	EmailAccounts []EmailAccount      `yaml:"email_accounts"`
	TestEmail     string              `yaml:"test_email,omitempty"`
}

// WebAdminConfig represents web administration interface credentials
type WebAdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// DNSProviderConfig represents DNS provider configuration
type DNSProviderConfig struct {
	Name     string `yaml:"name"`
	APIToken string `yaml:"api_token"`
}

// Subdomain represents a subdomain configuration with its role
type Subdomain struct {
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

// SystemUser represents a system user configuration
type SystemUser struct {
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	Name      string `yaml:"name,omitempty"`
	Sudoers   bool   `yaml:"sudoers"`
	SSHPubKey string `yaml:"ssh_pub_key,omitempty"`
}

// EmailAccount represents an email account configuration
type EmailAccount struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	FullName string `yaml:"full_name,omitempty"`
}

// DomainUserConfig represents domain user SSH configuration
type DomainUserConfig struct {
	Password     string `yaml:"password,omitempty"`        // Password for SSH login
	SSHKeyMode   string `yaml:"ssh_key_mode"`              // "manual" or "auto"
	SSHPublicKey string `yaml:"ssh_public_key,omitempty"`  // SSH public key (manual mode or auto with provided key)
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
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
		// SystemUsers is optional - omitted from skeleton
		SystemUsers: []SystemUser{},
		Subdomains: []Subdomain{
			{Name: "adm", Role: "webadmin"},
			{Name: "mail", Role: "mailserver"},
			{Name: "webmail", Role: "webmail"},
			{Name: "www", Role: "website"},
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
