package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the Hibana Stack configuration
type Config struct {
	ServerIP        string             `yaml:"server_ip"`
	DNSProvider     *DNSProviderConfig `yaml:"dns_provider,omitempty"`
	Domains         []Domain           `yaml:"domains"`
	DomainRedirects []DomainRedirect   `yaml:"domain_redirects,omitempty"`
	SystemUsers     []SystemUser       `yaml:"system_users,omitempty"`
}

// DNSProviderConfig represents DNS provider configuration
type DNSProviderConfig struct {
	Type     string `yaml:"type"`               // "local", "external", or "manual"
	Name     string `yaml:"name,omitempty"`     // Provider name (required for local and external)
	APIToken string `yaml:"api_token,omitempty"` // API token (required for local and external)
}

// Domain represents a domain configuration
type Domain struct {
	Name          string            `yaml:"name"`
	IsPrimary     bool              `yaml:"is_primary"`
	Subdomains    []Subdomain       `yaml:"subdomains"`
	WebAdmin      *WebAdminConfig   `yaml:"webadmin,omitempty"`
	EmailAccounts []EmailAccount    `yaml:"email_accounts"`
	DomainUser    *DomainUserConfig `yaml:"domain_user,omitempty"`
}

// WebAdminConfig represents web administration interface credentials
type WebAdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
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
	Password     string `yaml:"password,omitempty"`       // Password for SSH login
	SSHKeyMode   string `yaml:"ssh_key_mode"`             // "manual" or "auto"
	SSHPublicKey string `yaml:"ssh_public_key,omitempty"` // SSH public key (manual mode or auto with provided key)
}

// DomainRedirect represents a domain redirect configuration
type DomainRedirect struct {
	From      string `yaml:"from"`               // Source domain (e.g., example.fr)
	To        string `yaml:"to,omitempty"`       // Target URL (default: primary domain)
	Permanent bool   `yaml:"permanent,omitempty"` // Use 301 (permanent) instead of 302 (temporary)
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
	if c.ServerIP == "" {
		return fmt.Errorf("server_ip is required")
	}

	if len(c.Domains) == 0 {
		return fmt.Errorf("at least one domain is required")
	}

	// Check for exactly one primary domain
	primaryCount := 0
	for _, d := range c.Domains {
		if d.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount == 0 {
		return fmt.Errorf("exactly one domain must have is_primary: true")
	}
	if primaryCount > 1 {
		return fmt.Errorf("only one domain can have is_primary: true (found %d)", primaryCount)
	}

	// Validate DNS provider
	if c.DNSProvider != nil {
		if err := c.DNSProvider.Validate(); err != nil {
			return fmt.Errorf("dns_provider: %w", err)
		}
	}

	// Validate each domain
	for i, d := range c.Domains {
		if d.Name == "" {
			return fmt.Errorf("domains[%d]: name is required", i)
		}

		if len(d.EmailAccounts) == 0 {
			return fmt.Errorf("domains[%d] (%s): at least one email account is required", i, d.Name)
		}

		for j, acc := range d.EmailAccounts {
			if acc.Username == "" {
				return fmt.Errorf("domains[%d] (%s): email_accounts[%d]: username is required", i, d.Name, j)
			}
			if acc.Password == "" {
				return fmt.Errorf("domains[%d] (%s): email_accounts[%d]: password is required", i, d.Name, j)
			}
		}

		// Primary domain must have webadmin configured
		if d.IsPrimary && d.WebAdmin == nil {
			return fmt.Errorf("domains[%d] (%s): webadmin is required for primary domain", i, d.Name)
		}
	}

	return nil
}

// Validate validates the DNS provider configuration
func (d *DNSProviderConfig) Validate() error {
	validTypes := map[string]bool{"local": true, "external": true, "manual": true}
	if !validTypes[d.Type] {
		return fmt.Errorf("type must be 'local', 'external', or 'manual' (got '%s')", d.Type)
	}

	// For local and external, name and api_token are required
	if d.Type == "local" || d.Type == "external" {
		if d.Name == "" {
			return fmt.Errorf("name is required for type '%s'", d.Type)
		}
		if d.APIToken == "" {
			return fmt.Errorf("api_token is required for type '%s'", d.Type)
		}
	}

	return nil
}

// GetPrimaryDomain returns the primary domain configuration
func (c *Config) GetPrimaryDomain() *Domain {
	for i := range c.Domains {
		if c.Domains[i].IsPrimary {
			return &c.Domains[i]
		}
	}
	return nil
}

// GetPrimaryDomainName returns the primary domain name
func (c *Config) GetPrimaryDomainName() string {
	if d := c.GetPrimaryDomain(); d != nil {
		return d.Name
	}
	return ""
}

// GetSecondaryDomains returns all non-primary domains
func (c *Config) GetSecondaryDomains() []Domain {
	var secondary []Domain
	for _, d := range c.Domains {
		if !d.IsPrimary {
			secondary = append(secondary, d)
		}
	}
	return secondary
}

// HasSubdomainRole checks if a domain has a subdomain with the specified role
func (d *Domain) HasSubdomainRole(role string) bool {
	for _, sub := range d.Subdomains {
		if sub.Role == role {
			return true
		}
	}
	return false
}

// GetSubdomainByRole returns the subdomain with the specified role
func (d *Domain) GetSubdomainByRole(role string) *Subdomain {
	for i := range d.Subdomains {
		if d.Subdomains[i].Role == role {
			return &d.Subdomains[i]
		}
	}
	return nil
}

// GenerateSkeleton creates a skeleton configuration
func GenerateSkeleton() *Config {
	return &Config{
		ServerIP: "YOUR_SERVER_IP",
		DNSProvider: &DNSProviderConfig{
			Type:     "external",
			Name:     "hostinger",
			APIToken: "YOUR_API_TOKEN",
		},
		Domains: []Domain{
			{
				Name:      "example.com",
				IsPrimary: true,
				Subdomains: []Subdomain{
					{Name: "adm", Role: "webadmin"},
					{Name: "mail", Role: "mailserver"},
					{Name: "webmail", Role: "webmail"},
					{Name: "www", Role: "website"},
				},
				WebAdmin: &WebAdminConfig{
					Username: "admin",
					Password: "__RANDOM__",
				},
				EmailAccounts: []EmailAccount{
					{
						Username: "admin",
						Password: "__RANDOM__",
						FullName: "Administrator",
					},
				},
				DomainUser: &DomainUserConfig{
					Password:   "__RANDOM__",
					SSHKeyMode: "auto",
				},
			},
		},
		SystemUsers: []SystemUser{},
	}
}

// GetDNSRecords generates DNS records for a specific domain
func (c *Config) GetDNSRecords(domain *Domain, dkimPublicKey string) []DNSRecord {
	// Get current timestamp for SOA serial (YYYYMMDDnn format)
	serial := time.Now().Format("2006010215") // YYYYMMDDnn format

	primaryDomain := c.GetPrimaryDomainName()
	mailServer := fmt.Sprintf("mail.%s", primaryDomain)

	records := []DNSRecord{
		// SOA record (required for PowerDNS to be authoritative)
		{Type: "SOA", Name: "@", Content: fmt.Sprintf("ns1.%s. hostmaster.%s. %s 10800 3600 604800 3600", primaryDomain, domain.Name, serial), TTL: 86400},

		// NS record (nameserver) - points to primary domain's NS
		{Type: "NS", Name: "@", Content: fmt.Sprintf("ns1.%s.", primaryDomain), TTL: 3600},

		// A record for domain
		{Type: "A", Name: "@", Content: c.ServerIP, TTL: 300},

		// MX record - all domains point to the primary mail server
		{Type: "MX", Name: "@", Content: mailServer, Priority: 10, TTL: 14400},

		// SPF record
		{Type: "TXT", Name: "@", Content: fmt.Sprintf("v=spf1 ip4:%s -all", c.ServerIP), TTL: 14400},

		// DMARC record
		{Type: "TXT", Name: "_dmarc", Content: fmt.Sprintf("v=DMARC1; p=none; rua=mailto:dmarc@%s", domain.Name), TTL: 3600},

		// CAA records for Let's Encrypt
		{Type: "CAA", Name: "@", Content: `0 issue "letsencrypt.org"`, TTL: 14400},
		{Type: "CAA", Name: "@", Content: `0 issuewild "letsencrypt.org"`, TTL: 14400},
	}

	// Add A records for subdomains
	for _, sub := range domain.Subdomains {
		records = append(records, DNSRecord{
			Type:    "A",
			Name:    sub.Name,
			Content: c.ServerIP,
			TTL:     300,
		})
	}

	// For primary domain, add NS1 A record
	if domain.IsPrimary {
		records = append(records, DNSRecord{
			Type:    "A",
			Name:    "ns1",
			Content: c.ServerIP,
			TTL:     3600,
		})
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
