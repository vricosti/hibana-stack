package dnsprovider

import (
	"fmt"
	"strings"

	"github.com/ovh/go-ovh/ovh"
	"golang.org/x/net/idna"
)

// OVHCloudProvider implements DNS provider for OVHcloud
type OVHCloudProvider struct {
	client *ovh.Client
}

// OVHCloudCredentials holds OVHcloud API credentials
type OVHCloudCredentials struct {
	Endpoint          string
	ApplicationKey    string
	ApplicationSecret string
	ConsumerKey       string
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
func (o *OVHCloudProvider) ConfigureMailRecords(domain, serverIP string) error {
	domainASCII, err := idna.ToASCII(domain)
	if err != nil {
		domainASCII = domain
	}

	fmt.Println("  Configuring mail DNS records...")

	// MX record
	mxRecord := OVHRecord{
		FieldType: "MX",
		SubDomain: "",
		Target:    fmt.Sprintf("10 mx.%s.", domainASCII),
		TTL:       14400,
	}
	if err := o.ReplaceOrCreateRecord(domain, mxRecord); err != nil {
		fmt.Printf("    Warning: Failed to create MX record: %v\n", err)
	} else {
		fmt.Printf("    MX record: 10 mx.%s\n", domain)
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
func UpdateDNSRecordsPreInstallOVH(creds OVHCloudCredentials, domainCfg DomainConfig, serverIP string) error {
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

	// Configure A records
	fmt.Println("\n  Configuring A records...")

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

	// MX record if mailserver role exists
	hasMailserver := false
	for _, sub := range domainCfg.Subdomains {
		if sub.Role == "mailserver" {
			hasMailserver = true
			break
		}
	}

	if hasMailserver {
		domainASCII, _ := idna.ToASCII(domain)
		mxRecord := OVHRecord{
			FieldType: "MX",
			SubDomain: "",
			Target:    fmt.Sprintf("10 mx.%s.", domainASCII),
			TTL:       14400,
		}
		if err := provider.ReplaceOrCreateRecord(domain, mxRecord); err != nil {
			fmt.Printf("    Warning: Failed to create MX record: %v\n", err)
		} else {
			fmt.Printf("    %s MX 10 mx.%s\n", domain, domain)
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
func UpdateDNSRecordsPostInstallOVH(creds OVHCloudCredentials, domainCfg DomainConfig, serverIP string) error {
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

	// Create SPF record using type "SPF" (OVH specific) without quotes
	spfRecord := OVHRecord{
		FieldType: "SPF",
		SubDomain: "",
		Target:    fmt.Sprintf("v=spf1 ip4:%s -all", serverIP), // No quotes for SPF type
		TTL:       14400,
	}
	if err := provider.CreateRecord(domain, spfRecord); err != nil {
		fmt.Printf("    Warning: Failed to create SPF record: %v\n", err)
	} else {
		fmt.Printf("    %s SPF v=spf1 ip4:%s -all\n", domain, serverIP)
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
