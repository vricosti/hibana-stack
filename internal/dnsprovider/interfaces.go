package dnsprovider

// DNSRecord is the normalized record type used across all providers
type DNSRecord struct {
	ID       string // Provider-specific ID (converted to string for uniformity)
	Type     string // A, AAAA, CNAME, MX, TXT, SPF, DKIM, DMARC, NS, SOA, CAA
	Name     string // Subdomain name ("@" or "" for root, "www", "_dmarc", etc.)
	Content  string // Record value (IP, target, mail server, etc.)
	TTL      int
	Priority int // For MX records
}

// Credentials is an interface for provider-specific credential handling
type Credentials interface {
	// Validate checks if all required fields are present
	Validate() error
	// ProviderName returns the provider identifier (e.g., "hostinger", "ovhcloud")
	ProviderName() string
	// ToYAMLFields returns the credential fields for YAML config generation
	ToYAMLFields() map[string]string
}

// CredentialField describes a single credential input field for interactive prompting
type CredentialField struct {
	Key      string // Field key used in config (e.g., "api_token")
	Label    string // Display label for prompting (e.g., "API Token")
	Required bool
	Secret   bool   // Whether to mask input
	Default  string // Default value if any
	HelpText string // Additional guidance
}

// CredentialPrompter defines how to interactively collect credentials for a provider
type CredentialPrompter interface {
	// PromptFields returns the list of fields to prompt for
	PromptFields() []CredentialField
	// CreateCredentials builds Credentials from user input values
	CreateCredentials(values map[string]string) (Credentials, error)
	// SetupInstructions returns provider-specific setup instructions
	SetupInstructions() string
}

// Provider is the core interface all DNS providers must implement
// Note: Method names are designed to avoid conflicts with existing provider-specific methods
type Provider interface {
	// Name returns the provider identifier
	Name() string
	// DisplayName returns the human-readable provider name
	DisplayName() string

	// VerifyDomainManaged checks if the domain is managed by this provider
	VerifyDomainManaged(domain string) error

	// RefreshZone triggers a zone refresh/reload if supported
	RefreshZone(domain string) error
}

// RecordProvider extends Provider with normalized DNS record operations
// These methods use DNSRecord for provider-agnostic record handling
type RecordProvider interface {
	Provider

	// GetRecordsNormalized retrieves all DNS records in normalized format
	GetRecordsNormalized(domain string) ([]DNSRecord, error)
	// CreateRecordNormalized adds a new DNS record
	CreateRecordNormalized(domain string, record DNSRecord) error
	// UpdateRecordNormalized modifies an existing DNS record
	UpdateRecordNormalized(domain string, recordID string, record DNSRecord) error
	// DeleteRecordNormalized removes a DNS record by ID
	DeleteRecordNormalized(domain string, recordID string) error
	// ReplaceRecords replaces all records of specified types (batch operation)
	ReplaceRecords(domain string, records []DNSRecord) error
}

// MailProvider extends Provider with mail-specific DNS operations
type MailProvider interface {
	Provider
	// ConfigureSPF sets up SPF record for the domain
	ConfigureSPF(domain, serverIP string) error
	// ConfigureDMARC sets up DMARC record for the domain
	ConfigureDMARC(domain, policy string) error
	// ConfigureDKIM sets up DKIM record for the domain
	ConfigureDKIM(domain, selector, publicKey string) error
	// ConfigureMX sets up MX record for the domain
	ConfigureMX(domain, mailServer string, priority int) error
}

// PTRProvider is for providers that support PTR record management (VPS providers)
type PTRProvider interface {
	// ConfigurePTR sets up reverse DNS for the server IP
	ConfigurePTR(serverIP, hostname string) error
}

// NameserverProvider is for providers that support nameserver management
type NameserverProvider interface {
	// GetNameservers retrieves current nameservers for the domain
	GetNameservers(domain string) ([]string, error)
	// UpdateNameservers sets custom nameservers for the domain
	UpdateNameservers(domain string, ns1, ns2 string) error
	// ResetNameserversToDefault restores default provider nameservers
	ResetNameserversToDefault(domain string) error
	// EnsureNSRecords ensures NS records point to the correct server
	EnsureNSRecords(domain, serverIP string) error
}

// DomainConfig holds domain configuration for DNS operations
type DomainConfig struct {
	Name       string
	Subdomains []SubdomainConfig
}

// SubdomainConfig holds subdomain configuration
type SubdomainConfig struct {
	Name string
	Role string // webadmin, mailserver, webmail, website
}
