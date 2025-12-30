package dnsprovider

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ovh/go-ovh/ovh"
	"golang.org/x/net/idna"
)

// ============================================================================
// Provider Registration
// ============================================================================

func init() {
	Register(ProviderRegistration{
		Name:            "ovhcloud",
		DisplayName:     "OVHcloud",
		Factory:         newOVHCloudProviderFromCreds,
		PrompterFactory: func() CredentialPrompter { return &OVHCloudPrompter{} },
		CredsFromMap:    ovhcloudCredsFromMap,
		SupportsMail:    true,
		SupportsPTR:     true,
		SupportsNS:      false,
	})
}

// ============================================================================
// Credentials
// ============================================================================

// OVHCloudCredentials holds OVHcloud API credentials and implements Credentials interface
type OVHCloudCredentials struct {
	Endpoint          string
	ApplicationKey    string
	ApplicationSecret string
	ConsumerKey       string
}

// Validate checks if all required fields are present
func (c *OVHCloudCredentials) Validate() error {
	if c.ApplicationKey == "" {
		return fmt.Errorf("application_key is required for OVHcloud")
	}
	if c.ApplicationSecret == "" {
		return fmt.Errorf("application_secret is required for OVHcloud")
	}
	if c.ConsumerKey == "" {
		return fmt.Errorf("consumer_key is required for OVHcloud")
	}
	if c.Endpoint == "" {
		c.Endpoint = "ovh-eu" // Default endpoint
	}
	return nil
}

// ProviderName returns the provider identifier
func (c *OVHCloudCredentials) ProviderName() string {
	return "ovhcloud"
}

// ToYAMLFields returns the credential fields for YAML config generation
func (c *OVHCloudCredentials) ToYAMLFields() map[string]string {
	return map[string]string{
		"endpoint":           c.Endpoint,
		"application_key":    c.ApplicationKey,
		"application_secret": c.ApplicationSecret,
		"consumer_key":       c.ConsumerKey,
	}
}

// ovhcloudCredsFromMap creates OVHCloudCredentials from a map
func ovhcloudCredsFromMap(m map[string]string) (Credentials, error) {
	creds := &OVHCloudCredentials{
		Endpoint:          m["endpoint"],
		ApplicationKey:    m["application_key"],
		ApplicationSecret: m["application_secret"],
		ConsumerKey:       m["consumer_key"],
	}
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	return creds, nil
}

// ============================================================================
// Credential Prompter
// ============================================================================

// OVHCloudPrompter implements CredentialPrompter for OVHcloud
type OVHCloudPrompter struct{}

// PromptFields returns the list of fields to prompt for
func (p *OVHCloudPrompter) PromptFields() []CredentialField {
	return []CredentialField{
		{
			Key:      "application_key",
			Label:    "Application Key",
			Required: true,
			Secret:   false,
		},
		{
			Key:      "application_secret",
			Label:    "Application Secret",
			Required: true,
			Secret:   true,
		},
		{
			Key:      "consumer_key",
			Label:    "Consumer Key",
			Required: true,
			Secret:   true,
		},
	}
}

// CreateCredentials builds Credentials from user input values
func (p *OVHCloudPrompter) CreateCredentials(values map[string]string) (Credentials, error) {
	return ovhcloudCredsFromMap(values)
}

// SetupInstructions returns provider-specific setup instructions
func (p *OVHCloudPrompter) SetupInstructions() string {
	return `To get your OVHcloud API credentials:
1. Go to: https://manager.eu.ovhcloud.com/#/iam/api-keys/onboarding
2. Fill in the following permissions (add each line separately):

   DNS Zone Management:
   GET    /domain/zone
   GET    /domain/zone/*
   POST   /domain/zone/*
   PUT    /domain/zone/*
   DELETE /domain/zone/*

   IP/PTR Management (for reverse DNS):
   GET    /ip
   GET    /ip/*
   POST   /ip/*
   PUT    /ip/*
   DELETE /ip/*

3. Set validity to 'Unlimited' for permanent access
4. Copy the three keys generated`
}

// newOVHCloudProviderFromCreds creates an OVHCloudProvider from Credentials interface
func newOVHCloudProviderFromCreds(creds Credentials) (Provider, error) {
	ovhCreds, ok := creds.(*OVHCloudCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type for OVHcloud, expected *OVHCloudCredentials")
	}
	return NewOVHCloudProvider(*ovhCreds)
}

// ============================================================================
// Provider Interface Implementation
// ============================================================================

// OVHCloudProvider implements DNS provider for OVHcloud
type OVHCloudProvider struct {
	client *ovh.Client
}

// Name returns the provider identifier
func (o *OVHCloudProvider) Name() string {
	return "ovhcloud"
}

// DisplayName returns the human-readable provider name
func (o *OVHCloudProvider) DisplayName() string {
	return "OVHcloud"
}

// GetRecordsNormalized retrieves all DNS records in normalized format (implements Provider interface)
func (o *OVHCloudProvider) GetRecordsNormalized(domain string) ([]DNSRecord, error) {
	records, err := o.GetRecords(domain)
	if err != nil {
		return nil, err
	}
	result := make([]DNSRecord, len(records))
	for i, r := range records {
		result[i] = DNSRecord{
			ID:      strconv.FormatInt(r.ID, 10),
			Type:    r.FieldType,
			Name:    r.SubDomain,
			Content: r.Target,
			TTL:     r.TTL,
		}
	}
	return result, nil
}

// CreateRecordNormalized adds a new DNS record (implements Provider interface)
func (o *OVHCloudProvider) CreateRecordNormalized(domain string, record DNSRecord) error {
	ovhRecord := OVHRecord{
		FieldType: record.Type,
		SubDomain: record.Name,
		Target:    record.Content,
		TTL:       record.TTL,
	}
	return o.CreateRecord(domain, ovhRecord)
}

// UpdateRecordNormalized modifies an existing DNS record (implements Provider interface)
func (o *OVHCloudProvider) UpdateRecordNormalized(domain string, recordID string, record DNSRecord) error {
	id, err := strconv.ParseInt(recordID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid record ID: %w", err)
	}
	ovhRecord := OVHRecord{
		FieldType: record.Type,
		SubDomain: record.Name,
		Target:    record.Content,
		TTL:       record.TTL,
	}
	return o.UpdateRecord(domain, id, ovhRecord)
}

// DeleteRecordNormalized removes a DNS record by ID (implements Provider interface)
func (o *OVHCloudProvider) DeleteRecordNormalized(domain string, recordID string) error {
	id, err := strconv.ParseInt(recordID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid record ID: %w", err)
	}
	return o.DeleteRecord(domain, id)
}

// ReplaceRecords replaces all records (implements Provider interface)
func (o *OVHCloudProvider) ReplaceRecords(domain string, records []DNSRecord) error {
	for _, r := range records {
		ovhRecord := OVHRecord{
			FieldType: r.Type,
			SubDomain: r.Name,
			Target:    r.Content,
			TTL:       r.TTL,
		}
		if err := o.ReplaceOrCreateRecord(domain, ovhRecord); err != nil {
			return err
		}
	}
	return o.RefreshZone(domain)
}

// OVHRecord represents a DNS record in OVHcloud
type OVHRecord struct {
	ID        int64  `json:"id,omitempty"`
	FieldType string `json:"fieldType"`
	SubDomain string `json:"subDomain"`
	Target    string `json:"target"`
	TTL       int    `json:"ttl,omitempty"`
	Zone      string `json:"zone,omitempty"`
}

// OVHZoneRecord represents the full record response from OVH API
type OVHZoneRecord struct {
	ID        int64  `json:"id"`
	FieldType string `json:"fieldType"`
	SubDomain string `json:"subDomain"`
	Target    string `json:"target"`
	TTL       int    `json:"ttl"`
	Zone      string `json:"zone"`
}

// NewOVHCloudProvider creates a new OVHcloud DNS provider
func NewOVHCloudProvider(creds OVHCloudCredentials) (*OVHCloudProvider, error) {
	client, err := ovh.NewClient(
		creds.Endpoint,
		creds.ApplicationKey,
		creds.ApplicationSecret,
		creds.ConsumerKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OVH client: %w", err)
	}

	return &OVHCloudProvider{
		client: client,
	}, nil
}

// VerifyDomainManaged checks if the domain is managed by this OVHcloud account
func (o *OVHCloudProvider) VerifyDomainManaged(domain string) error {
	// Convert domain to punycode for API calls
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// List all zones to verify domain exists
	var zones []string
	err = o.client.Get("/domain/zone", &zones)
	if err != nil {
		return fmt.Errorf("failed to list DNS zones: %w", err)
	}

	for _, z := range zones {
		if z == domainASCII {
			fmt.Printf("  Domain %s is managed by this OVHcloud account\n", domain)
			return nil
		}
	}

	return fmt.Errorf("domain %s is not managed by this OVHcloud account", domain)
}

// GetRecords retrieves all DNS records for a domain
func (o *OVHCloudProvider) GetRecords(domain string) ([]OVHRecord, error) {
	// Convert domain to punycode for API calls
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain name: %w", err)
	}

	// Get all record IDs
	var recordIDs []int64
	err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record", domainASCII), &recordIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS records: %w", err)
	}

	// Fetch each record details
	var records []OVHRecord
	for _, id := range recordIDs {
		var record OVHZoneRecord
		err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), &record)
		if err != nil {
			fmt.Printf("  Warning: Could not fetch record %d: %v\n", id, err)
			continue
		}
		records = append(records, OVHRecord{
			ID:        record.ID,
			FieldType: record.FieldType,
			SubDomain: record.SubDomain,
			Target:    record.Target,
			TTL:       record.TTL,
			Zone:      record.Zone,
		})
	}

	return records, nil
}

// GetMailRecords returns existing MX, SPF, and DMARC records
func (o *OVHCloudProvider) GetMailRecords(domain string) ([]OVHRecord, error) {
	records, err := o.GetRecords(domain)
	if err != nil {
		return nil, err
	}

	var mailRecords []OVHRecord
	for _, r := range records {
		// MX records
		if r.FieldType == "MX" {
			mailRecords = append(mailRecords, r)
			continue
		}
		// SPF records - OVH uses "SPF" type instead of TXT for SPF records
		if r.FieldType == "SPF" {
			mailRecords = append(mailRecords, r)
			continue
		}
		// SPF records in TXT format (fallback)
		if r.FieldType == "TXT" && (r.SubDomain == "" || r.SubDomain == "@") {
			target := strings.Trim(r.Target, "\"")
			if strings.HasPrefix(target, "v=spf1") {
				mailRecords = append(mailRecords, r)
				continue
			}
		}
		// DMARC records
		if r.FieldType == "TXT" && r.SubDomain == "_dmarc" {
			mailRecords = append(mailRecords, r)
			continue
		}
	}

	return mailRecords, nil
}

// CreateRecord creates a new DNS record
func (o *OVHCloudProvider) CreateRecord(domain string, record OVHRecord) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	payload := map[string]interface{}{
		"fieldType": record.FieldType,
		"subDomain": record.SubDomain,
		"target":    record.Target,
		"ttl":       record.TTL,
	}

	var result OVHZoneRecord
	err = o.client.Post(fmt.Sprintf("/domain/zone/%s/record", domainASCII), payload, &result)
	if err != nil {
		return fmt.Errorf("failed to create DNS record: %w", err)
	}

	return nil
}

// UpdateRecord updates an existing DNS record
func (o *OVHCloudProvider) UpdateRecord(domain string, recordID int64, record OVHRecord) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	payload := map[string]interface{}{
		"subDomain": record.SubDomain,
		"target":    record.Target,
		"ttl":       record.TTL,
	}

	err = o.client.Put(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, recordID), payload, nil)
	if err != nil {
		return fmt.Errorf("failed to update DNS record: %w", err)
	}

	return nil
}

// DeleteRecord deletes a DNS record
func (o *OVHCloudProvider) DeleteRecord(domain string, recordID int64) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	err = o.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, recordID), nil)
	if err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	return nil
}

// RefreshZone applies pending changes to the DNS zone
func (o *OVHCloudProvider) RefreshZone(domain string) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	err = o.client.Post(fmt.Sprintf("/domain/zone/%s/refresh", domainASCII), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to refresh DNS zone: %w", err)
	}

	return nil
}

// ReplaceOrCreateRecord replaces an existing record or creates a new one
func (o *OVHCloudProvider) ReplaceOrCreateRecord(domain string, record OVHRecord) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// Find existing record with same type and subdomain
	var recordIDs []int64
	query := fmt.Sprintf("/domain/zone/%s/record?fieldType=%s&subDomain=%s",
		domainASCII, record.FieldType, record.SubDomain)
	err = o.client.Get(query, &recordIDs)
	if err != nil {
		return fmt.Errorf("failed to query DNS records: %w", err)
	}

	// If record exists, update it
	if len(recordIDs) > 0 {
		return o.UpdateRecord(domain, recordIDs[0], record)
	}

	// Otherwise create new record
	return o.CreateRecord(domain, record)
}

// DeleteRecordByTypeAndSubdomain deletes a record matching type and subdomain
func (o *OVHCloudProvider) DeleteRecordByTypeAndSubdomain(domain, recordType, subdomain string) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// Find existing record
	var recordIDs []int64
	query := fmt.Sprintf("/domain/zone/%s/record?fieldType=%s&subDomain=%s",
		domainASCII, recordType, subdomain)
	err = o.client.Get(query, &recordIDs)
	if err != nil {
		return fmt.Errorf("failed to query DNS records: %w", err)
	}

	// Delete all matching records
	for _, id := range recordIDs {
		if err := o.DeleteRecord(domain, id); err != nil {
			return err
		}
	}

	return nil
}

// ConfigureMailRecords configures MX, SPF, and DMARC records for mail
func (o *OVHCloudProvider) ConfigureMailRecords(domain, serverIP, mailserverSubdomain string) error {
	if mailserverSubdomain == "" {
		mailserverSubdomain = "mx" // default fallback
	}

	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		domainASCII = domain
	}

	fmt.Println("  Configuring mail DNS records...")

	// MX record
	mxRecord := OVHRecord{
		FieldType: "MX",
		SubDomain: "",
		Target:    fmt.Sprintf("10 %s.%s.", mailserverSubdomain, domainASCII),
		TTL:       14400,
	}
	if err := o.ReplaceOrCreateRecord(domain, mxRecord); err != nil {
		fmt.Printf("    Warning: Failed to create MX record: %v\n", err)
	} else {
		fmt.Printf("    MX record: 10 %s.%s\n", mailserverSubdomain, domain)
	}

	// SPF record
	spfRecord := OVHRecord{
		FieldType: "TXT",
		SubDomain: "",
		Target:    fmt.Sprintf("\"v=spf1 ip4:%s -all\"", serverIP),
		TTL:       14400,
	}
	if err := o.ReplaceOrCreateRecord(domain, spfRecord); err != nil {
		fmt.Printf("    Warning: Failed to create SPF record: %v\n", err)
	} else {
		fmt.Printf("    SPF record: v=spf1 ip4:%s -all\n", serverIP)
	}

	// DMARC record
	dmarcRecord := OVHRecord{
		FieldType: "TXT",
		SubDomain: "_dmarc",
		Target:    fmt.Sprintf("\"v=DMARC1; p=none; rua=mailto:contact@%s\"", domainASCII),
		TTL:       3600,
	}
	if err := o.ReplaceOrCreateRecord(domain, dmarcRecord); err != nil {
		fmt.Printf("    Warning: Failed to create DMARC record: %v\n", err)
	} else {
		fmt.Printf("    DMARC record: v=DMARC1; p=none\n")
	}

	// Refresh zone to apply changes
	if err := o.RefreshZone(domain); err != nil {
		fmt.Printf("    Warning: Failed to refresh zone: %v\n", err)
	}

	return nil
}

// ConfigureARecords configures A records for domain and subdomains
func (o *OVHCloudProvider) ConfigureARecords(domain string, subdomains []string, serverIP string) error {
	fmt.Println("  Configuring A records...")

	// Root domain A record
	rootRecord := OVHRecord{
		FieldType: "A",
		SubDomain: "",
		Target:    serverIP,
		TTL:       300,
	}
	if err := o.ReplaceOrCreateRecord(domain, rootRecord); err != nil {
		fmt.Printf("    Warning: Failed to create A record for %s: %v\n", domain, err)
	} else {
		fmt.Printf("    %s A %s\n", domain, serverIP)
	}

	// Subdomain A records
	for _, sub := range subdomains {
		subRecord := OVHRecord{
			FieldType: "A",
			SubDomain: sub,
			Target:    serverIP,
			TTL:       300,
		}
		if err := o.ReplaceOrCreateRecord(domain, subRecord); err != nil {
			fmt.Printf("    Warning: Failed to create A record for %s.%s: %v\n", sub, domain, err)
		} else {
			fmt.Printf("    %s.%s A %s\n", sub, domain, serverIP)
		}
	}

	// Refresh zone to apply changes
	if err := o.RefreshZone(domain); err != nil {
		fmt.Printf("    Warning: Failed to refresh zone: %v\n", err)
	}

	return nil
}

// AddDKIMRecord adds a DKIM DNS record for a domain
func (o *OVHCloudProvider) AddDKIMRecord(domain, dkimPublicKey string) error {
	if dkimPublicKey == "" {
		return nil
	}

	fmt.Printf("  Adding DKIM record for %s...\n", domain)

	// Delete existing DKIM records first (only one DKIM per selector allowed)
	fmt.Println("    Removing existing DKIM records...")
	if err := o.DeleteExistingDKIMRecords(domain, "default"); err != nil {
		fmt.Printf("    Warning: Failed to delete existing DKIM records: %v\n", err)
	}

	// Create DKIM record using type "DKIM" (OVH specific) without quotes
	dkimRecord := OVHRecord{
		FieldType: "DKIM",
		SubDomain: "default._domainkey",
		Target:    fmt.Sprintf("v=DKIM1; h=sha256; k=rsa; p=%s", dkimPublicKey), // No quotes for DKIM type
		TTL:       14400,
	}

	if err := o.CreateRecord(domain, dkimRecord); err != nil {
		return fmt.Errorf("failed to create DKIM record: %w", err)
	}

	fmt.Printf("    DKIM record created: default._domainkey.%s\n", domain)

	// Refresh zone to apply changes
	if err := o.RefreshZone(domain); err != nil {
		fmt.Printf("    Warning: Failed to refresh zone: %v\n", err)
	}

	return nil
}

// UpdateDNSRecordsPreInstallOVH configures DNS records BEFORE Ansible playbook for OVHcloud
func UpdateDNSRecordsPreInstallOVH(creds OVHCloudCredentials, domainCfg DomainConfig, serverIP, serverIPv6 string) error {
	domain := domainCfg.Name

	fmt.Printf("\n  [PRE-INSTALL] Configuring OVHcloud DNS for %s...\n", domain)

	provider, err := NewOVHCloudProvider(creds)
	if err != nil {
		return fmt.Errorf("failed to create OVH provider: %w", err)
	}

	// Fetch and display current configuration
	fmt.Println("  Fetching current DNS configuration...")
	existingRecords, err := provider.GetRecords(domain)
	if err != nil {
		fmt.Printf("    Warning: Could not fetch existing records: %v\n", err)
	} else {
		fmt.Println("\n  Current DNS Records:")
		if len(existingRecords) == 0 {
			fmt.Println("    (no records found)")
		} else {
			for _, r := range existingRecords {
				name := r.SubDomain
				if name == "" {
					name = "@"
				}
				fmt.Printf("    %-35s %-6s %5d  %s\n", name+"."+domain, r.FieldType, r.TTL, r.Target)
			}
		}
	}

	// Configure A records (IPv4)
	fmt.Println("\n  Configuring A records (IPv4)...")

	// Root domain
	rootRecord := OVHRecord{
		FieldType: "A",
		SubDomain: "",
		Target:    serverIP,
		TTL:       300,
	}
	if err := provider.ReplaceOrCreateRecord(domain, rootRecord); err != nil {
		fmt.Printf("    Warning: Failed to create A record for %s: %v\n", domain, err)
	} else {
		fmt.Printf("    %s A %s\n", domain, serverIP)
	}

	// Subdomain A records
	for _, sub := range domainCfg.Subdomains {
		subRecord := OVHRecord{
			FieldType: "A",
			SubDomain: sub.Name,
			Target:    serverIP,
			TTL:       300,
		}
		if err := provider.ReplaceOrCreateRecord(domain, subRecord); err != nil {
			fmt.Printf("    Warning: Failed to create A record for %s.%s: %v\n", sub.Name, domain, err)
		} else {
			fmt.Printf("    %s.%s A %s\n", sub.Name, domain, serverIP)
		}
	}

	// Configure AAAA records (IPv6) if IPv6 is available
	if serverIPv6 != "" {
		fmt.Println("\n  Configuring AAAA records (IPv6)...")

		// Root domain AAAA
		rootAAAARecord := OVHRecord{
			FieldType: "AAAA",
			SubDomain: "",
			Target:    serverIPv6,
			TTL:       300,
		}
		if err := provider.ReplaceOrCreateRecord(domain, rootAAAARecord); err != nil {
			fmt.Printf("    Warning: Failed to create AAAA record for %s: %v\n", domain, err)
		} else {
			fmt.Printf("    %s AAAA %s\n", domain, serverIPv6)
		}

		// Subdomain AAAA records
		for _, sub := range domainCfg.Subdomains {
			subAAAARecord := OVHRecord{
				FieldType: "AAAA",
				SubDomain: sub.Name,
				Target:    serverIPv6,
				TTL:       300,
			}
			if err := provider.ReplaceOrCreateRecord(domain, subAAAARecord); err != nil {
				fmt.Printf("    Warning: Failed to create AAAA record for %s.%s: %v\n", sub.Name, domain, err)
			} else {
				fmt.Printf("    %s.%s AAAA %s\n", sub.Name, domain, serverIPv6)
			}
		}
	} else {
		fmt.Println("\n  ℹ No IPv6 address detected, skipping AAAA records")
	}

	// MX record if mailserver role exists and get the subdomain name
	mailserverSubdomain := ""
	for _, sub := range domainCfg.Subdomains {
		if sub.Role == "mailserver" {
			mailserverSubdomain = sub.Name
			break
		}
	}

	if mailserverSubdomain != "" {
		domainASCII, _ := idna.ToASCII(domain)
		mxRecord := OVHRecord{
			FieldType: "MX",
			SubDomain: "",
			Target:    fmt.Sprintf("10 %s.%s.", mailserverSubdomain, domainASCII),
			TTL:       14400,
		}
		if err := provider.ReplaceOrCreateRecord(domain, mxRecord); err != nil {
			fmt.Printf("    Warning: Failed to create MX record: %v\n", err)
		} else {
			fmt.Printf("    %s MX 10 %s.%s\n", domain, mailserverSubdomain, domain)
		}
	}

	// Refresh zone to apply changes
	if err := provider.RefreshZone(domain); err != nil {
		fmt.Printf("    Warning: Failed to refresh zone: %v\n", err)
	}

	fmt.Println("\n  [PRE-INSTALL] OVHcloud DNS configured - waiting for propagation...")
	return nil
}

// DeleteExistingSPFRecords deletes all existing SPF records (both SPF and TXT types)
func (o *OVHCloudProvider) DeleteExistingSPFRecords(domain string) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// Delete SPF type records
	var spfIDs []int64
	err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record?fieldType=SPF&subDomain=", domainASCII), &spfIDs)
	if err == nil {
		for _, id := range spfIDs {
			o.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), nil)
			fmt.Printf("    Deleted existing SPF record (ID: %d)\n", id)
		}
	}

	// Delete TXT records that contain SPF
	var txtIDs []int64
	err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record?fieldType=TXT&subDomain=", domainASCII), &txtIDs)
	if err == nil {
		for _, id := range txtIDs {
			var record struct {
				Target string `json:"target"`
			}
			o.client.Get(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), &record)
			if strings.Contains(record.Target, "v=spf1") {
				o.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), nil)
				fmt.Printf("    Deleted existing SPF TXT record (ID: %d)\n", id)
			}
		}
	}

	return nil
}

// DeleteExistingDMARCRecords deletes all existing DMARC records (both DMARC and TXT types)
func (o *OVHCloudProvider) DeleteExistingDMARCRecords(domain string) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// Delete DMARC type records
	var dmarcIDs []int64
	err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record?fieldType=DMARC&subDomain=_dmarc", domainASCII), &dmarcIDs)
	if err == nil {
		for _, id := range dmarcIDs {
			o.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), nil)
			fmt.Printf("    Deleted existing DMARC record (ID: %d)\n", id)
		}
	}

	// Delete TXT records on _dmarc subdomain (fallback)
	var txtIDs []int64
	err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record?fieldType=TXT&subDomain=_dmarc", domainASCII), &txtIDs)
	if err == nil {
		for _, id := range txtIDs {
			o.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), nil)
			fmt.Printf("    Deleted existing DMARC TXT record (ID: %d)\n", id)
		}
	}

	return nil
}

// DeleteExistingDKIMRecords deletes all existing DKIM records (both DKIM and TXT types)
func (o *OVHCloudProvider) DeleteExistingDKIMRecords(domain, selector string) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	subdomain := fmt.Sprintf("%s._domainkey", selector)

	// Delete DKIM type records
	var dkimIDs []int64
	err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record?fieldType=DKIM&subDomain=%s", domainASCII, subdomain), &dkimIDs)
	if err == nil {
		for _, id := range dkimIDs {
			o.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), nil)
			fmt.Printf("    Deleted existing DKIM record (ID: %d)\n", id)
		}
	}

	// Delete TXT records on _domainkey subdomain (fallback)
	var txtIDs []int64
	err = o.client.Get(fmt.Sprintf("/domain/zone/%s/record?fieldType=TXT&subDomain=%s", domainASCII, subdomain), &txtIDs)
	if err == nil {
		for _, id := range txtIDs {
			o.client.Delete(fmt.Sprintf("/domain/zone/%s/record/%d", domainASCII, id), nil)
			fmt.Printf("    Deleted existing DKIM TXT record (ID: %d)\n", id)
		}
	}

	return nil
}

// UpdateDNSRecordsPostInstallOVH configures SPF and DMARC records AFTER Ansible for OVHcloud
func UpdateDNSRecordsPostInstallOVH(creds OVHCloudCredentials, domainCfg DomainConfig, serverIP, serverIPv6 string) error {
	domain := domainCfg.Name

	// Check if mailserver role exists
	hasMailserver := false
	for _, sub := range domainCfg.Subdomains {
		if sub.Role == "mailserver" {
			hasMailserver = true
			break
		}
	}

	if !hasMailserver {
		return nil
	}

	fmt.Printf("\n  [POST-INSTALL] Configuring OVHcloud mail DNS records for %s...\n", domain)

	provider, err := NewOVHCloudProvider(creds)
	if err != nil {
		return fmt.Errorf("failed to create OVH provider: %w", err)
	}

	domainASCII, _ := idna.ToASCII(domain)

	// Delete existing SPF records first (only one SPF allowed)
	fmt.Println("  Removing existing SPF records...")
	if err := provider.DeleteExistingSPFRecords(domain); err != nil {
		fmt.Printf("    Warning: Failed to delete existing SPF records: %v\n", err)
	}

	// Build SPF record with IPv4 and optionally IPv6
	spfContent := fmt.Sprintf("v=spf1 ip4:%s", serverIP)
	if serverIPv6 != "" {
		spfContent += fmt.Sprintf(" ip6:%s", serverIPv6)
	}
	spfContent += " -all"

	// Create SPF record using type "SPF" (OVH specific) without quotes
	spfRecord := OVHRecord{
		FieldType: "SPF",
		SubDomain: "",
		Target:    spfContent,
		TTL:       14400,
	}
	if err := provider.CreateRecord(domain, spfRecord); err != nil {
		fmt.Printf("    Warning: Failed to create SPF record: %v\n", err)
	} else {
		fmt.Printf("    %s SPF %s\n", domain, spfContent)
	}

	// Delete existing DMARC records first (only one DMARC allowed)
	fmt.Println("  Removing existing DMARC records...")
	if err := provider.DeleteExistingDMARCRecords(domain); err != nil {
		fmt.Printf("    Warning: Failed to delete existing DMARC records: %v\n", err)
	}

	// Create DMARC record using type "DMARC" (OVH specific) without quotes
	dmarcRecord := OVHRecord{
		FieldType: "DMARC",
		SubDomain: "_dmarc",
		Target:    fmt.Sprintf("v=DMARC1; p=none; rua=mailto:contact@%s", domainASCII), // No quotes for DMARC type
		TTL:       3600,
	}
	if err := provider.CreateRecord(domain, dmarcRecord); err != nil {
		fmt.Printf("    Warning: Failed to create DMARC record: %v\n", err)
	} else {
		fmt.Printf("    _dmarc.%s DMARC v=DMARC1; p=none\n", domain)
	}

	// Refresh zone to apply changes
	if err := provider.RefreshZone(domain); err != nil {
		fmt.Printf("    Warning: Failed to refresh zone: %v\n", err)
	}

	fmt.Println("\n  [POST-INSTALL] OVHcloud mail DNS records configured")
	return nil
}

// AddDKIMRecordOVH adds DKIM record for OVHcloud
func AddDKIMRecordOVH(creds OVHCloudCredentials, domain, dkimPublicKey string) error {
	if dkimPublicKey == "" {
		return nil
	}

	provider, err := NewOVHCloudProvider(creds)
	if err != nil {
		return fmt.Errorf("failed to create OVH provider: %w", err)
	}

	return provider.AddDKIMRecord(domain, dkimPublicKey)
}

// ============================================================================
// PTR Record (Reverse DNS) Management
// ============================================================================

// OVHIPBlock represents an IP block in OVH
type OVHIPBlock struct {
	IP          string `json:"ip"`
	Description string `json:"description,omitempty"`
}

// OVHReverseRecord represents a reverse DNS record in OVH
type OVHReverseRecord struct {
	IPReverse string `json:"ipReverse"`
	Reverse   string `json:"reverse"`
}

// GetIPBlocks retrieves all IP blocks associated with the OVH account
func (o *OVHCloudProvider) GetIPBlocks() ([]string, error) {
	var ipBlocks []string
	if err := o.client.Get("/ip", &ipBlocks); err != nil {
		return nil, fmt.Errorf("failed to list IP blocks: %w", err)
	}
	return ipBlocks, nil
}

// FindIPBlock finds the IP block that contains the given IP address
func (o *OVHCloudProvider) FindIPBlock(targetIP string) (string, error) {
	ipBlocks, err := o.GetIPBlocks()
	if err != nil {
		return "", err
	}

	// First, try to find an exact match (for single IPs like "1.2.3.4")
	for _, block := range ipBlocks {
		// URL-encode the IP block for API calls (/ becomes %2F)
		if block == targetIP || strings.HasPrefix(block, targetIP+"/") {
			return block, nil
		}
	}

	// For CIDR blocks, we need to check if the IP is in the range
	// For now, we'll try each block and see if it contains our IP
	for _, block := range ipBlocks {
		// Try to get reverse records for this block
		// If we can query it successfully with our IP, it's the right block
		encodedBlock := strings.ReplaceAll(block, "/", "%2F")
		var reverseRecords []string
		err := o.client.Get(fmt.Sprintf("/ip/%s/reverse", encodedBlock), &reverseRecords)
		if err == nil {
			// This block exists, check if our IP can be used
			return block, nil
		}
	}

	return "", fmt.Errorf("no IP block found containing %s", targetIP)
}

// CreateReverseDNS creates a reverse DNS (PTR) record for an IP address
func (o *OVHCloudProvider) CreateReverseDNS(ipBlock, ipAddress, hostname string) error {
	// Ensure hostname ends with a dot (required for PTR records)
	if !strings.HasSuffix(hostname, ".") {
		hostname = hostname + "."
	}

	// Convert hostname to Punycode for IDN support
	hostnameASCII, err := idna.ToASCII(strings.TrimSuffix(hostname, "."))
	if err != nil {
		hostnameASCII = strings.TrimSuffix(hostname, ".")
	}
	hostnameASCII = hostnameASCII + "."

	// URL-encode the IP block for API calls
	encodedBlock := strings.ReplaceAll(ipBlock, "/", "%2F")

	// Check if reverse DNS already exists
	var existingReverse OVHReverseRecord
	encodedIP := strings.ReplaceAll(ipAddress, ":", "%3A") // Encode colons for IPv6
	err = o.client.Get(fmt.Sprintf("/ip/%s/reverse/%s", encodedBlock, encodedIP), &existingReverse)
	if err == nil {
		// Reverse exists, check if it's already correct
		if existingReverse.Reverse == hostnameASCII {
			fmt.Printf("  ℹ PTR already configured: %s → %s\n", ipAddress, hostnameASCII)
			return nil
		}
		// Delete existing record first
		err = o.client.Delete(fmt.Sprintf("/ip/%s/reverse/%s", encodedBlock, encodedIP), nil)
		if err != nil {
			fmt.Printf("    Warning: Failed to delete existing PTR: %v\n", err)
		}
	}

	// Create the reverse DNS record
	params := struct {
		IPReverse string `json:"ipReverse"`
		Reverse   string `json:"reverse"`
	}{
		IPReverse: ipAddress,
		Reverse:   hostnameASCII,
	}

	var result interface{}
	err = o.client.Post(fmt.Sprintf("/ip/%s/reverse", encodedBlock), params, &result)
	if err != nil {
		return fmt.Errorf("failed to create PTR record: %w", err)
	}

	return nil
}

// ConfigurePTRRecords configures PTR records for the mail server (IPv4 and IPv6)
func (o *OVHCloudProvider) ConfigurePTRRecords(serverIPv4, serverIPv6, mailHostname string) error {
	fmt.Println("→ Looking up IP blocks in OVHcloud account...")

	ipBlocks, err := o.GetIPBlocks()
	if err != nil {
		return fmt.Errorf("failed to get IP blocks: %w", err)
	}

	if len(ipBlocks) == 0 {
		return fmt.Errorf("no IP blocks found in OVHcloud account")
	}

	fmt.Printf("  Found %d IP block(s)\n", len(ipBlocks))

	// Convert mail hostname to Punycode for display
	mailHostnameASCII, err := idna.ToASCII(mailHostname)
	if err != nil {
		mailHostnameASCII = mailHostname
	}

	ipv4Success := false
	ipv6Success := false

	// Configure PTR for IPv4
	if serverIPv4 != "" {
		fmt.Printf("\n→ Configuring PTR for %s (IPv4)...\n", serverIPv4)

		// Find the IP block for this IP
		// For OVH VPS, the IP is usually represented as a /32 block
		ipBlock := serverIPv4 // Try the IP directly first

		err := o.CreateReverseDNS(ipBlock, serverIPv4, mailHostname)
		if err != nil {
			// Try with /32 suffix
			ipBlock = serverIPv4 + "/32"
			err = o.CreateReverseDNS(ipBlock, serverIPv4, mailHostname)
		}
		if err != nil {
			fmt.Printf("  ⚠ Warning: Failed to set PTR for %s: %v\n", serverIPv4, err)
			fmt.Println("  ℹ Note: PTR records require the IP to be in your OVHcloud account")
		} else {
			fmt.Printf("  ✓ PTR record set: %s → %s\n", serverIPv4, mailHostnameASCII)
			ipv4Success = true
		}
	}

	// Configure PTR for IPv6
	if serverIPv6 != "" {
		fmt.Printf("\n→ Configuring PTR for %s (IPv6)...\n", serverIPv6)

		// For IPv6, we need to find the appropriate block
		// OVH typically assigns /128 or /64 blocks
		ipBlock := serverIPv6 // Try the IP directly first

		err := o.CreateReverseDNS(ipBlock, serverIPv6, mailHostname)
		if err != nil {
			// Try with /128 suffix
			ipBlock = serverIPv6 + "/128"
			err = o.CreateReverseDNS(ipBlock, serverIPv6, mailHostname)
		}
		if err != nil {
			fmt.Printf("  ⚠ Warning: Failed to set PTR for %s: %v\n", serverIPv6, err)
		} else {
			fmt.Printf("  ✓ PTR record set: %s → %s\n", serverIPv6, mailHostnameASCII)
			ipv6Success = true
		}
	}

	// Show manual instructions if needed
	if !ipv4Success && !ipv6Success {
		fmt.Println("\n⚠️  PTR Configuration Failed")
		fmt.Println("   Please configure PTR records manually in OVHcloud Manager:")
		fmt.Println("   1. Go to: https://www.ovh.com/manager/dedicated/#/iplb")
		fmt.Println("   2. Select your IP address")
		fmt.Println("   3. Click 'Edit reverse'")
		fmt.Printf("   4. Set value to: %s\n", mailHostnameASCII)
		return fmt.Errorf("failed to configure PTR records automatically")
	}

	if !ipv4Success {
		fmt.Println("\n⚠️  IPv4 PTR not configured. Please configure manually if needed.")
	}
	if serverIPv6 != "" && !ipv6Success {
		fmt.Println("\n⚠️  IPv6 PTR not configured. Please configure manually if needed.")
	}

	return nil
}

// ConfigurePTROVH is a helper function to configure PTR records for OVHcloud
func ConfigurePTROVH(creds OVHCloudCredentials, serverIPv4, serverIPv6, mailHostname string) error {
	provider, err := NewOVHCloudProvider(creds)
	if err != nil {
		return fmt.Errorf("failed to create OVH provider: %w", err)
	}

	return provider.ConfigurePTRRecords(serverIPv4, serverIPv6, mailHostname)
}
