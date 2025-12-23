package dnsprovider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

const (
	hostingerAPIBaseURL    = "https://developers.hostinger.com"
	hostingerDNSAPIURL     = "https://developers.hostinger.com/api/dns/v1"
	hostingerDomainsAPIURL = "https://developers.hostinger.com/api/domains/v1"
)

// HostingerProvider implements DNS provider for Hostinger
type HostingerProvider struct {
	apiToken string
	client   *http.Client
}

// NewHostingerProvider creates a new Hostinger DNS provider
func NewHostingerProvider(apiToken string) *HostingerProvider {
	return &HostingerProvider{
		apiToken: apiToken,
		client:   &http.Client{},
	}
}

// toPunycode converts an internationalized domain name (IDN) to ASCII (Punycode)
// This is required for domains with special characters (accents, etc.)
// Example: "tiéuntigre.fr" -> "xn--tiuntigre-c4a.fr"
func toPunycode(domain string) (string, error) {
	// Convert to ASCII using IDNA2008
	ascii, err := idna.ToASCII(domain)
	if err != nil {
		return "", fmt.Errorf("failed to convert domain to punycode: %w", err)
	}
	return ascii, nil
}

// HostingerRecord represents a DNS record in Hostinger
type HostingerRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

// HostingerDomain represents a domain in the portfolio
type HostingerDomain struct {
	ID        int      `json:"id"`
	Domain    string   `json:"domain"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
	ExpiresAt string   `json:"expires_at"`
}

// VerifyDomainManaged checks if the domain is managed by this Hostinger account
func (h *HostingerProvider) VerifyDomainManaged(domain string) error {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	url := hostingerDomainsAPIURL + "/portfolio"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch portfolio: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed: invalid API token")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// The API returns an array of domains directly, not wrapped in an object
	var domains []HostingerDomain
	if err := json.Unmarshal(body, &domains); err != nil {
		return fmt.Errorf("failed to parse portfolio response: %w", err)
	}

	// Check if domain is in the portfolio (compare using ASCII/punycode version)
	for _, d := range domains {
		if d.Domain == domainASCII {
			fmt.Printf("✓ Domain %s is managed by this Hostinger account\n", domain)
			return nil
		}
	}

	return fmt.Errorf("domain %s is not managed by this Hostinger account", domain)
}

// EnsureNSRecords ensures that ns1 and ns2 A records exist for the domain with the correct IP
func (h *HostingerProvider) EnsureNSRecords(domain, serverIP string) error {
	// Get existing records
	existingRecords, err := h.getRecords(domain)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	// Check if ns1 and ns2 records exist with correct IP
	var ns1Record, ns2Record *HostingerRecord
	for i := range existingRecords {
		if existingRecords[i].Type == "A" && existingRecords[i].Name == "ns1" {
			ns1Record = &existingRecords[i]
		}
		if existingRecords[i].Type == "A" && existingRecords[i].Name == "ns2" {
			ns2Record = &existingRecords[i]
		}
	}

	// Check if records exist and have correct IP
	ns1OK := ns1Record != nil && ns1Record.Content == serverIP
	ns2OK := ns2Record != nil && ns2Record.Content == serverIP

	// Create or update records as needed
	newRecords := []HostingerRecord{}

	if !ns1OK {
		newRecords = append(newRecords, HostingerRecord{
			Type:    "A",
			Name:    "ns1",
			Content: serverIP,
			TTL:     14400,
		})
	}

	if !ns2OK {
		newRecords = append(newRecords, HostingerRecord{
			Type:    "A",
			Name:    "ns2",
			Content: serverIP,
			TTL:     14400,
		})
	}

	if len(newRecords) > 0 {
		if err := h.updateRecords(domain, newRecords); err != nil {
			return fmt.Errorf("failed to create/update NS records: %w", err)
		}

		if !ns1OK {
			if ns1Record != nil {
				fmt.Printf("✓ Updated DNS record: ns1 A %s (was %s)\n", serverIP, ns1Record.Content)
			} else {
				fmt.Println("✓ Created DNS record: ns1 A " + serverIP)
			}
		}
		if !ns2OK {
			if ns2Record != nil {
				fmt.Printf("✓ Updated DNS record: ns2 A %s (was %s)\n", serverIP, ns2Record.Content)
			} else {
				fmt.Println("✓ Created DNS record: ns2 A " + serverIP)
			}
		}
	}

	if ns1OK {
		fmt.Printf("✓ DNS record already correct: ns1 A %s\n", serverIP)
	}
	if ns2OK {
		fmt.Printf("✓ DNS record already correct: ns2 A %s\n", serverIP)
	}

	return nil
}

// APIZoneRecord represents the DNS zone record format returned by the Hostinger API
type APIZoneRecord struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	TTL     int    `json:"ttl"`
	Records []struct {
		Content    string `json:"content"`
		IsDisabled bool   `json:"is_disabled"`
	} `json:"records"`
}

// getRecords retrieves all DNS records for a domain
func (h *HostingerProvider) getRecords(domain string) ([]HostingerRecord, error) {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain name: %w", err)
	}

	req, err := http.NewRequest("GET", hostingerDNSAPIURL+"/zones/"+domainASCII, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the API response format
	var apiRecords []APIZoneRecord
	if err := json.Unmarshal(body, &apiRecords); err != nil {
		return nil, fmt.Errorf("failed to parse records response: %w", err)
	}

	// Convert to flat HostingerRecord format for internal use
	var records []HostingerRecord
	for _, apiRec := range apiRecords {
		for _, content := range apiRec.Records {
			records = append(records, HostingerRecord{
				Name:    apiRec.Name,
				Type:    apiRec.Type,
				TTL:     apiRec.TTL,
				Content: content.Content,
			})
		}
	}

	return records, nil
}

// GetRecordsPublic retrieves all DNS records for a domain (public method)
func (h *HostingerProvider) GetRecordsPublic(domain string) ([]HostingerRecord, error) {
	return h.getRecords(domain)
}

// ZoneRecord represents the API request format for a DNS zone record
type ZoneRecord struct {
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	TTL     int            `json:"ttl"`
	Records []RecordContent `json:"records"`
}

// RecordContent represents the content of a DNS record
type RecordContent struct {
	Content string `json:"content"`
}

// UpdateRequest represents the Hostinger API request format for updating DNS records
type UpdateRequest struct {
	Overwrite bool         `json:"overwrite"`
	Zone      []ZoneRecord `json:"zone"`
}

// updateRecords updates DNS records for the domain (adds new records)
func (h *HostingerProvider) updateRecords(domain string, records []HostingerRecord) error {
	return h.updateRecordsWithOverwrite(domain, records, false)
}

// replaceRecords replaces DNS records for the domain (overwrites existing records of same name/type)
func (h *HostingerProvider) replaceRecords(domain string, records []HostingerRecord) error {
	return h.updateRecordsWithOverwrite(domain, records, true)
}

// updateRecordsWithOverwrite updates DNS records with optional overwrite mode
func (h *HostingerProvider) updateRecordsWithOverwrite(domain string, records []HostingerRecord, overwrite bool) error {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// Convert HostingerRecord to the API's expected format
	zoneRecords := make([]ZoneRecord, 0, len(records))
	for _, r := range records {
		zoneRecords = append(zoneRecords, ZoneRecord{
			Name: r.Name,
			Type: r.Type,
			TTL:  r.TTL,
			Records: []RecordContent{
				{Content: r.Content},
			},
		})
	}

	updateReq := UpdateRequest{
		Overwrite: overwrite,
		Zone:      zoneRecords,
	}

	payload, err := json.Marshal(updateReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", hostingerDNSAPIURL+"/zones/"+domainASCII, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AddRecords adds new DNS records to a domain (public method)
func (h *HostingerProvider) AddRecords(domain string, records []HostingerRecord) error {
	return h.updateRecords(domain, records)
}

// ReplaceRecords replaces DNS records for the domain (overwrites existing records of same name/type)
func (h *HostingerProvider) ReplaceRecords(domain string, records []HostingerRecord) error {
	return h.replaceRecords(domain, records)
}

// DeleteCNAMEAndCreateA deletes a CNAME record if it exists and creates an A record
// This handles the common case where Hostinger has a default CNAME for www pointing to the root domain
func (h *HostingerProvider) DeleteCNAMEAndCreateA(domain, name, ip string) error {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// Get existing records
	existingRecords, err := h.getRecords(domain)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	// Check if there's a CNAME for this name
	hasCNAME := false
	for _, r := range existingRecords {
		if r.Name == name && r.Type == "CNAME" {
			hasCNAME = true
			break
		}
	}

	if hasCNAME {
		// Build new zone without the CNAME, keeping all other records
		var newRecords []HostingerRecord
		for _, r := range existingRecords {
			if r.Name == name && r.Type == "CNAME" {
				continue // Skip the CNAME
			}
			newRecords = append(newRecords, r)
		}

		// Add the new A record
		newRecords = append(newRecords, HostingerRecord{
			Type:    "A",
			Name:    name,
			Content: ip,
			TTL:     300,
		})

		// Reset zone and rebuild
		resetURL := hostingerDNSAPIURL + "/zones/" + domainASCII + "/reset"
		req, err := http.NewRequest("POST", resetURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create reset request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+h.apiToken)

		resp, err := h.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to reset zone: %w", err)
		}
		resp.Body.Close()

		// Re-add all records
		if len(newRecords) > 0 {
			return h.updateRecords(domain, newRecords)
		}
		return nil
	}

	// No CNAME, just use replaceRecords to update/create the A record
	return h.replaceRecords(domain, []HostingerRecord{
		{
			Type:    "A",
			Name:    name,
			Content: ip,
			TTL:     300,
		},
	})
}

// DeleteRecord deletes a DNS record from a domain
func (h *HostingerProvider) DeleteRecord(domain, name, recordType string) error {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// Get existing records
	existingRecords, err := h.getRecords(domain)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	// Filter out the record to delete
	var newRecords []HostingerRecord
	found := false
	for _, r := range existingRecords {
		if r.Name == name && r.Type == recordType {
			found = true
			continue // Skip this record (delete it)
		}
		newRecords = append(newRecords, r)
	}

	if !found {
		// Record doesn't exist, nothing to delete
		return nil
	}

	// The Hostinger API doesn't have a direct delete endpoint for individual records
	// We need to reset the zone and re-add all records except the one we want to delete
	// First, reset the zone
	resetURL := hostingerDNSAPIURL + "/zones/" + domainASCII + "/reset"
	req, err := http.NewRequest("POST", resetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create reset request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.apiToken)

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reset zone: %w", err)
	}
	resp.Body.Close()

	// Re-add all records except the deleted one
	if len(newRecords) > 0 {
		return h.updateRecords(domain, newRecords)
	}

	return nil
}

// UpdateNameservers updates the domain's nameservers to use ns1 and ns2
// It first checks if they are already correctly configured
func (h *HostingerProvider) UpdateNameservers(domain string) error {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	desiredNS1 := fmt.Sprintf("ns1.%s", domainASCII)
	desiredNS2 := fmt.Sprintf("ns2.%s", domainASCII)

	// First, check current nameservers
	currentNameservers, err := h.GetNameservers(domain)
	if err != nil {
		fmt.Printf("  ⚠ Warning: Could not fetch current nameservers: %v\n", err)
		// Continue anyway to try the update
	} else {
		// Check if nameservers are already correctly configured
		if len(currentNameservers) >= 2 &&
			currentNameservers[0] == desiredNS1 &&
			currentNameservers[1] == desiredNS2 {
			fmt.Printf("✓ Nameservers already configured correctly: %s, %s\n", desiredNS1, desiredNS2)
			return nil
		}
		fmt.Printf("  Current nameservers: %v\n", currentNameservers)
		fmt.Printf("  Desired nameservers: %s, %s\n", desiredNS1, desiredNS2)
	}

	// Use correct payload format with ns1 and ns2 as separate fields
	payload := map[string]string{
		"ns1": desiredNS1,
		"ns2": desiredNS2,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", hostingerDomainsAPIURL+"/portfolio/"+domainASCII+"/nameservers", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("✓ Updated nameservers to %s and %s\n", desiredNS1, desiredNS2)
	return nil
}

// GetNameservers retrieves the current nameservers for a domain
func (h *HostingerProvider) GetNameservers(domain string) ([]string, error) {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain name: %w", err)
	}

	// Use the domain details endpoint which includes nameservers
	req, err := http.NewRequest("GET", hostingerDomainsAPIURL+"/portfolio/"+domainASCII, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Domain details response includes nameservers as a map
	var response struct {
		NameServers map[string]string `json:"name_servers"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse domain details response: %w", err)
	}

	// Convert the name_servers map to a sorted slice
	var nameservers []string
	if response.NameServers != nil {
		// Extract and sort keys to ensure consistent order (ns1, ns2, ns3, etc.)
		keys := make([]string, 0, len(response.NameServers))
		for k := range response.NameServers {
			keys = append(keys, k)
		}
		// Sort to get consistent order
		for _, k := range []string{"ns1", "ns2", "ns3", "ns4", "ns5", "ns6", "ns7", "ns8"} {
			if ns, exists := response.NameServers[k]; exists && ns != "" {
				nameservers = append(nameservers, ns)
			}
		}
	}

	return nameservers, nil
}

// Hostinger default nameservers
const (
	hostingerDefaultNS1 = "ns1.dns-parking.com"
	hostingerDefaultNS2 = "ns2.dns-parking.com"
)

// ResetNameserversToDefault resets the domain's nameservers to Hostinger defaults
// This also clears any child nameservers (glue records) that were set for local DNS
func (h *HostingerProvider) ResetNameserversToDefault(domain string) error {
	// Convert domain to punycode for API calls
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name: %w", err)
	}

	// First, check current nameservers
	currentNameservers, err := h.GetNameservers(domain)
	if err != nil {
		fmt.Printf("  ⚠ Warning: Could not fetch current nameservers: %v\n", err)
		// Continue anyway to try the update
	} else {
		// Check if nameservers are already set to Hostinger defaults
		if len(currentNameservers) >= 2 &&
			currentNameservers[0] == hostingerDefaultNS1 &&
			currentNameservers[1] == hostingerDefaultNS2 {
			fmt.Printf("  ✓ Nameservers already set to Hostinger defaults: %s, %s\n", hostingerDefaultNS1, hostingerDefaultNS2)
			return nil
		}
		fmt.Printf("  Current nameservers: %v\n", currentNameservers)
	}

	// Update to Hostinger default nameservers
	payload := map[string]string{
		"ns1": hostingerDefaultNS1,
		"ns2": hostingerDefaultNS2,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", hostingerDomainsAPIURL+"/portfolio/"+domainASCII+"/nameservers", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("  ✓ Nameservers reset to Hostinger defaults: %s, %s\n", hostingerDefaultNS1, hostingerDefaultNS2)
	fmt.Println("  ℹ Child nameservers (glue records) will be automatically cleared")
	return nil
}

// VerifyDomainOwnership verifies that the domain is managed by the DNS provider
func VerifyDomainOwnership(providerName, apiToken, domain string) error {
	if providerName == "" || apiToken == "" {
		// DNS provider not configured, skip
		return nil
	}

	// Normalize provider name (case insensitive)
	providerName = strings.ToLower(strings.TrimSpace(providerName))

	fmt.Println("\n🌐 Verifying DNS provider configuration...")

	switch providerName {
	case "hostinger":
		provider := NewHostingerProvider(apiToken)
		fmt.Println("→ Checking domain ownership...")
		if err := provider.VerifyDomainManaged(domain); err != nil {
			return fmt.Errorf("domain verification failed: %w", err)
		}
		fmt.Println("✓ DNS provider verification complete")
		return nil
	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}

// DomainConfig holds domain configuration for DNS updates
type DomainConfig struct {
	Name       string
	Subdomains []SubdomainConfig
}

// SubdomainConfig holds subdomain configuration
type SubdomainConfig struct {
	Name string
	Role string
}

// UpdateDNSRecords updates DNS records for the given domain and server IP
// providerType: "local" (PowerDNS - create NS records and update nameservers) or "external" (only add A records for subdomains)
// If simulate is true, it will only log what would be done without making actual API calls
func UpdateDNSRecords(providerName, providerType, apiToken string, domainCfg DomainConfig, serverIP string, simulate bool) error {
	if providerName == "" || apiToken == "" {
		// DNS provider not configured, skip
		return nil
	}

	// Normalize provider name (case insensitive)
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	domain := domainCfg.Name

	fmt.Printf("\n🌐 Configuring DNS for %s (type: %s)...\n", domain, providerType)

	switch providerName {
	case "hostinger":
		provider := NewHostingerProvider(apiToken)

		// Fetch and display current configuration
		fmt.Println("→ Fetching current DNS configuration...")

		existingRecords, err := provider.getRecords(domain)
		if err != nil {
			fmt.Printf("  ⚠ Warning: Could not fetch existing records: %v\n", err)
		} else {
			fmt.Println("\n  Current DNS Records:")
			if len(existingRecords) == 0 {
				fmt.Println("    (no records found)")
			} else {
				for _, r := range existingRecords {
					name := r.Name
					if name == "" || name == "@" {
						name = domain
					} else {
						name = name + "." + domain
					}
					fmt.Printf("    %-35s %-6s %5d  %s\n", name, r.Type, r.TTL, r.Content)
				}
			}
		}

		if simulate {
			fmt.Println("\n✓ [SIMULATION] DNS configuration simulated successfully")
			return nil
		}

		// Build list of records to create based on provider type
		var recordsToCreate []HostingerRecord

		if providerType == "local" {
			// Local DNS (PowerDNS) - create NS records and update nameservers
			fmt.Println("\n→ Configuring NS records for local PowerDNS...")
			if err := provider.EnsureNSRecords(domain, serverIP); err != nil {
				return fmt.Errorf("failed to configure NS records: %w", err)
			}

			fmt.Println("→ Updating domain nameservers...")
			if err := provider.UpdateNameservers(domain); err != nil {
				return fmt.Errorf("failed to update nameservers: %w", err)
			}
		} else if providerType == "external" {
			// External DNS - reset nameservers to Hostinger defaults and add DNS records
			fmt.Println("\n→ Resetting nameservers to Hostinger defaults...")
			if err := provider.ResetNameserversToDefault(domain); err != nil {
				fmt.Printf("  ⚠ Warning: Could not reset nameservers: %v\n", err)
			}

			fmt.Println("→ Configuring DNS records for external provider...")

			// Build map of existing records for quick lookup
			existingByNameType := make(map[string]HostingerRecord)
			for _, r := range existingRecords {
				key := fmt.Sprintf("%s:%s", r.Name, r.Type)
				existingByNameType[key] = r
			}

			// Helper function to check if we should skip a record
			shouldSkip := func(name, recordType string) (bool, string) {
				// Check if exact record exists with correct IP
				key := fmt.Sprintf("%s:%s", name, recordType)
				if existing, ok := existingByNameType[key]; ok {
					if existing.Content == serverIP {
						return true, fmt.Sprintf("already exists with correct IP")
					}
					// Record exists but with different IP - we'll update it
					return false, ""
				}
				// For A records, check if a CNAME exists (conflict)
				if recordType == "A" {
					cnameKey := fmt.Sprintf("%s:CNAME", name)
					if _, ok := existingByNameType[cnameKey]; ok {
						return true, fmt.Sprintf("CNAME exists, skipping A record")
					}
				}
				return false, ""
			}

			// Process A record for root domain
			if skip, reason := shouldSkip("@", "A"); skip {
				fmt.Printf("  ℹ %s A: %s\n", domain, reason)
			} else {
				recordsToCreate = append(recordsToCreate, HostingerRecord{
					Type:    "A",
					Name:    "@",
					Content: serverIP,
					TTL:     300,
				})
			}

			// Process A records for each subdomain
			for _, sub := range domainCfg.Subdomains {
				if skip, reason := shouldSkip(sub.Name, "A"); skip {
					fmt.Printf("  ℹ %s.%s A: %s\n", sub.Name, domain, reason)
				} else {
					recordsToCreate = append(recordsToCreate, HostingerRecord{
						Type:    "A",
						Name:    sub.Name,
						Content: serverIP,
						TTL:     300,
					})
				}
			}

			// Check if mailserver subdomain exists
			hasMailserver := false
			for _, sub := range domainCfg.Subdomains {
				if sub.Role == "mailserver" {
					hasMailserver = true
					break
				}
			}

			if hasMailserver {
				// Check MX record
				mxContent := fmt.Sprintf("10 mail.%s", domain)
				if existing, ok := existingByNameType["@:MX"]; ok && strings.Contains(existing.Content, fmt.Sprintf("mail.%s", domain)) {
					fmt.Printf("  ℹ %s MX: already configured\n", domain)
				} else {
					recordsToCreate = append(recordsToCreate, HostingerRecord{
						Type:    "MX",
						Name:    "@",
						Content: mxContent,
						TTL:     14400,
					})
				}

				// Check SPF record - look for existing SPF in TXT records
				spfContent := fmt.Sprintf("v=spf1 ip4:%s -all", serverIP)
				hasSPF := false
				for _, r := range existingRecords {
					if r.Type == "TXT" && r.Name == "@" && strings.HasPrefix(r.Content, "v=spf1") {
						hasSPF = true
						if strings.Contains(r.Content, serverIP) {
							fmt.Printf("  ℹ %s SPF: already configured\n", domain)
						} else {
							fmt.Printf("  ⚠ %s SPF: exists but doesn't include %s, please update manually\n", domain, serverIP)
						}
						break
					}
				}
				if !hasSPF {
					recordsToCreate = append(recordsToCreate, HostingerRecord{
						Type:    "TXT",
						Name:    "@",
						Content: spfContent,
						TTL:     14400,
					})
				}

				// Check DMARC record
				// Convert domain to Punycode for email address in DMARC record (RFC compliance)
				domainASCII, err := toPunycode(domain)
				if err != nil {
					domainASCII = domain // Fallback to original if conversion fails
				}
				dmarcContent := fmt.Sprintf("v=DMARC1; p=none; rua=mailto:dmarc@%s", domainASCII)
				if _, ok := existingByNameType["_dmarc:TXT"]; ok {
					fmt.Printf("  ℹ %s DMARC: already configured\n", domain)
				} else {
					recordsToCreate = append(recordsToCreate, HostingerRecord{
						Type:    "TXT",
						Name:    "_dmarc",
						Content: dmarcContent,
						TTL:     3600,
					})
				}
			}

			// Create the records one by one to handle partial failures
			if len(recordsToCreate) > 0 {
				fmt.Println("\n  Creating/updating DNS records:")
				for _, r := range recordsToCreate {
					name := r.Name
					if name == "@" {
						name = domain
					} else {
						name = name + "." + domain
					}

					// Use replaceRecords for A records to overwrite any existing IP (e.g., parking IP)
					// Use updateRecords for other record types to append without removing existing records
					var err error
					if r.Type == "A" {
						err = provider.replaceRecords(domain, []HostingerRecord{r})
					} else {
						err = provider.updateRecords(domain, []HostingerRecord{r})
					}

					if err != nil {
						fmt.Printf("    ✗ %s %s %s - %v\n", name, r.Type, r.Content, err)
					} else {
						fmt.Printf("    ✓ %s %s %s\n", name, r.Type, r.Content)
					}
				}
			} else {
				fmt.Println("  ℹ All DNS records already configured correctly")
			}

			// Note: DKIM will be added automatically after the playbook generates the keys
			if hasMailserver {
				fmt.Println("\n  ℹ DKIM record will be added after keys are generated")
			}
		}

		fmt.Println("\n✓ DNS provider configured successfully")
		return nil
	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}

// ============================================================================
// VPS and PTR Record Management
// ============================================================================

const hostingerVPSAPIURL = "https://developers.hostinger.com/api/vps/v1"

// VirtualMachine represents a VPS instance
type VirtualMachine struct {
	ID       int         `json:"id"`
	Hostname string      `json:"hostname"`
	State    string      `json:"state"`
	IPv4     []IPAddress `json:"ipv4"`
	IPv6     []IPAddress `json:"ipv6"`
}

// IPAddress represents an IP address attached to a VPS
type IPAddress struct {
	ID      int    `json:"id"`
	Address string `json:"address"`
	PTR     string `json:"ptr,omitempty"`
}

// GetAllIPs returns all IP addresses (IPv4 and IPv6) for a VM
func (vm *VirtualMachine) GetAllIPs() []IPAddress {
	var all []IPAddress
	all = append(all, vm.IPv4...)
	all = append(all, vm.IPv6...)
	return all
}

// GetVirtualMachines retrieves all VPS instances
func (h *HostingerProvider) GetVirtualMachines() ([]VirtualMachine, error) {
	req, err := http.NewRequest("GET", hostingerVPSAPIURL+"/virtual-machines", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var vms []VirtualMachine
	if err := json.Unmarshal(body, &vms); err != nil {
		return nil, fmt.Errorf("failed to parse VPS list: %w", err)
	}

	return vms, nil
}

// FindVMByIP finds a virtual machine by its IP address
func (h *HostingerProvider) FindVMByIP(serverIP string) (*VirtualMachine, *IPAddress, error) {
	vms, err := h.GetVirtualMachines()
	if err != nil {
		return nil, nil, err
	}

	for i := range vms {
		// Check IPv4 addresses
		for j := range vms[i].IPv4 {
			if vms[i].IPv4[j].Address == serverIP {
				return &vms[i], &vms[i].IPv4[j], nil
			}
		}
		// Check IPv6 addresses
		for j := range vms[i].IPv6 {
			if vms[i].IPv6[j].Address == serverIP {
				return &vms[i], &vms[i].IPv6[j], nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no VPS found with IP address %s", serverIP)
}

// CreatePTRRecord creates or updates a PTR record for a VPS IP address
func (h *HostingerProvider) CreatePTRRecord(vmID int, ipAddressID int, domain string) error {
	// Convert domain to Punycode for API compatibility
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return fmt.Errorf("invalid domain name for PTR record: %w", err)
	}

	url := fmt.Sprintf("%s/virtual-machines/%d/ptr/%d", hostingerVPSAPIURL, vmID, ipAddressID)

	payload := map[string]string{
		"domain": domainASCII,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ConfigurePTRRecords configures PTR records for the mail server
func (h *HostingerProvider) ConfigurePTRRecords(serverIP, mailHostname string) error {
	fmt.Println("→ Looking up VPS by IP address...")

	vm, _, err := h.FindVMByIP(serverIP)
	if err != nil {
		return fmt.Errorf("failed to find VPS: %w", err)
	}

	fmt.Printf("✓ Found VPS: %s (ID: %d)\n", vm.Hostname, vm.ID)

	mailHostnameASCII, err := toPunycode(mailHostname)
	if err != nil {
		mailHostnameASCII = mailHostname // Should not happen, but fallback
	}

	ipv4Success := false
	ipv6Failed := false
	var ipv6Addresses []string

	// Configure PTR for IPv4 addresses first (required before IPv6)
	for _, ip := range vm.IPv4 {
		fmt.Printf("→ Configuring PTR for %s (IPv4)...\n", ip.Address)

		if ip.PTR == mailHostname || ip.PTR == mailHostnameASCII {
			fmt.Printf("  ℹ PTR already configured: %s → %s\n", ip.Address, ip.PTR)
			ipv4Success = true
			continue
		}

		if err := h.CreatePTRRecord(vm.ID, ip.ID, mailHostname); err != nil {
			fmt.Printf("  ⚠ Warning: Failed to set PTR for %s: %v\n", ip.Address, err)
			continue
		}

		fmt.Printf("  ✓ PTR record set: %s → %s\n", ip.Address, mailHostnameASCII)
		ipv4Success = true
	}

	// Configure PTR for IPv6 addresses
	for _, ip := range vm.IPv6 {
		fmt.Printf("→ Configuring PTR for %s (IPv6)...\n", ip.Address)

		if ip.PTR == mailHostname || ip.PTR == mailHostnameASCII {
			fmt.Printf("  ℹ PTR already configured: %s → %s\n", ip.Address, ip.PTR)
			continue
		}

		if err := h.CreatePTRRecord(vm.ID, ip.ID, mailHostname); err != nil {
			// Known API bug: Hostinger API returns error 422 for IPv6 PTR records
			// This is a bug in the Hostinger API, not in our code
			if strings.Contains(err.Error(), "422") || strings.Contains(err.Error(), "VPS:2004") {
				fmt.Printf("  ⚠ Warning: IPv6 PTR configuration failed due to Hostinger API limitation\n")
				ipv6Failed = true
				ipv6Addresses = append(ipv6Addresses, ip.Address)
			} else {
				fmt.Printf("  ⚠ Warning: Failed to set PTR for %s: %v\n", ip.Address, err)
			}
			continue
		}

		fmt.Printf("  ✓ PTR record set: %s → %s\n", ip.Address, mailHostnameASCII)
	}

	// Display manual configuration instructions if IPv6 failed
	if ipv6Failed && len(ipv6Addresses) > 0 {
		fmt.Println("\n⚠️  IPv6 PTR Configuration Required")
		fmt.Println("   The Hostinger API currently has a known issue configuring IPv6 PTR records.")
		fmt.Println("   Please configure manually in Hostinger hPanel:")
		fmt.Println("   1. Go to: https://hpanel.hostinger.com")
		fmt.Println("   2. Navigate to: VPS → Your VPS → IP address")
		fmt.Println("   3. Set PTR record for IPv6:")
		for _, addr := range ipv6Addresses {
			fmt.Printf("      - %s → %s\n", addr, mailHostnameASCII)
		}
		if mailHostname != mailHostnameASCII {
			fmt.Printf("     (Note: use the Punycode value: %s)\n", mailHostnameASCII)
		}
		fmt.Println("   Note: IPv4 PTR is already configured correctly ✓")
	}

	if !ipv4Success && ipv6Failed {
		return fmt.Errorf("failed to configure PTR records for both IPv4 and IPv6")
	}

	return nil
}

// ConfigurePTR configures PTR records for the mail server using the DNS provider
func ConfigurePTR(providerName, apiToken, serverIP, domain string) error {
	if providerName == "" || apiToken == "" {
		// DNS provider not configured, show manual instructions
		fmt.Println("\n⚠️  PTR records must be configured manually:")
		fmt.Printf("    → Set PTR for IPv4 to: mail.%s\n", domain)
		fmt.Printf("    → Set PTR for IPv6 to: mail.%s\n", domain)
		fmt.Println("    → This is required for email deliverability")
		return nil
	}

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	mailHostname := fmt.Sprintf("mail.%s", domain)

	fmt.Println("\n📧 Configuring PTR records for mail server...")

	switch providerName {
	case "hostinger":
		provider := NewHostingerProvider(apiToken)
		if err := provider.ConfigurePTRRecords(serverIP, mailHostname); err != nil {
			fmt.Printf("⚠ Warning: PTR configuration failed: %v\n", err)

			mailHostnameASCII, _ := toPunycode(mailHostname)

			fmt.Println("\n⚠️  Please configure PTR records manually in Hostinger hPanel:")
			fmt.Println("    → VPS Settings → IP address → Set PTR record")
			if mailHostname != mailHostnameASCII {
				fmt.Printf("    → For both IPv4 and IPv6, use the value: %s\n", mailHostnameASCII)
			} else {
				fmt.Printf("    → IPv4: %s\n", mailHostname)
				fmt.Printf("    → IPv6: %s\n", mailHostname)
			}
			return nil // Don't fail the installation
		}
		fmt.Println("✓ PTR records configured successfully")
		return nil
	default:
		fmt.Printf("⚠ PTR configuration not supported for provider: %s\n", providerName)
		fmt.Println("\n⚠️  Please configure PTR records manually:")
		fmt.Printf("    → Set PTR for IPv4 to: %s\n", mailHostname)
		fmt.Printf("    → Set PTR for IPv6 to: %s\n", mailHostname)
		return nil
	}
}

// AddDKIMRecord adds a DKIM DNS record for a domain
func AddDKIMRecord(providerName, apiToken, domain, dkimPublicKey string) error {
	if providerName == "" || apiToken == "" || dkimPublicKey == "" {
		return nil
	}

	providerName = strings.ToLower(strings.TrimSpace(providerName))

	fmt.Printf("\n🔐 Adding DKIM record for %s...\n", domain)

	switch providerName {
	case "hostinger":
		provider := NewHostingerProvider(apiToken)

		// Check if DKIM record already exists
		existingRecords, err := provider.getRecords(domain)
		if err != nil {
			fmt.Printf("  ⚠ Warning: Could not fetch existing records: %v\n", err)
		} else {
			for _, r := range existingRecords {
				if r.Type == "TXT" && r.Name == "default._domainkey" {
					fmt.Printf("  ℹ DKIM record already exists for %s\n", domain)
					return nil
				}
			}
		}

		// Create DKIM record
		dkimRecord := HostingerRecord{
			Type:    "TXT",
			Name:    "default._domainkey",
			Content: fmt.Sprintf("v=DKIM1; h=sha256; k=rsa; p=%s", dkimPublicKey),
			TTL:     14400,
		}

		if err := provider.updateRecords(domain, []HostingerRecord{dkimRecord}); err != nil {
			return fmt.Errorf("failed to create DKIM record: %w", err)
		}

		fmt.Printf("  ✓ DKIM record created: default._domainkey.%s\n", domain)
		return nil

	default:
		fmt.Printf("  ⚠ DKIM record creation not supported for provider: %s\n", providerName)
		fmt.Printf("  Please add the following DKIM record manually:\n")
		fmt.Printf("    Name: default._domainkey.%s\n", domain)
		fmt.Printf("    Type: TXT\n")
		fmt.Printf("    Content: v=DKIM1; h=sha256; k=rsa; p=%s\n", dkimPublicKey)
		return nil
	}
}

// ReadDKIMPublicKey reads the DKIM public key from the OpenDKIM key file
func ReadDKIMPublicKey(domain string) (string, error) {
	// Convert domain to punycode for filesystem path compatibility
	domainASCII, err := toPunycode(domain)
	if err != nil {
		return "", fmt.Errorf("invalid domain name for DKIM key lookup: %w", err)
	}
	keyFile := fmt.Sprintf("/etc/opendkim/keys/%s/default.txt", domainASCII)

	content, err := os.ReadFile(keyFile)
	if err != nil {
		return "", fmt.Errorf("failed to read DKIM key file: %w", err)
	}

	// Parse the DKIM public key from the file
	// The file format is: default._domainkey IN TXT ( "v=DKIM1; h=sha256; k=rsa; " "p=MIIB..." )
	contentStr := string(content)

	// Extract the public key value (everything after "p=")
	startIdx := strings.Index(contentStr, "p=")
	if startIdx == -1 {
		return "", fmt.Errorf("could not find public key in DKIM file")
	}

	// Get everything after "p="
	keyPart := contentStr[startIdx+2:]

	// Remove quotes, parentheses, whitespace, and newlines
	keyPart = strings.ReplaceAll(keyPart, "\"", "")
	keyPart = strings.ReplaceAll(keyPart, "(", "")
	keyPart = strings.ReplaceAll(keyPart, ")", "")
	keyPart = strings.ReplaceAll(keyPart, " ", "")
	keyPart = strings.ReplaceAll(keyPart, "\n", "")
	keyPart = strings.ReplaceAll(keyPart, "\t", "")

	// Remove any trailing content after the key (like semicolon or anything else)
	if idx := strings.Index(keyPart, ";"); idx != -1 {
		keyPart = keyPart[:idx]
	}

	return strings.TrimSpace(keyPart), nil
}

// ============================================================================
// Pre-Install and Post-Install DNS Configuration
// ============================================================================

// UpdateDNSRecordsPreInstall configures DNS records BEFORE Ansible playbook execution
// This includes A records and MX records (needed for SSL certificate generation)
// SPF, DMARC, and DKIM records are added later in UpdateDNSRecordsPostInstall
func UpdateDNSRecordsPreInstall(providerName, providerType, apiToken string, domainCfg DomainConfig, serverIP string) error {
	if providerName == "" || apiToken == "" {
		return nil
	}

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	domain := domainCfg.Name

	fmt.Printf("\n🌐 [PRE-INSTALL] Configuring DNS for %s (type: %s)...\n", domain, providerType)

	switch providerName {
	case "hostinger":
		provider := NewHostingerProvider(apiToken)

		// Fetch and display current configuration
		fmt.Println("→ Fetching current DNS configuration...")
		existingRecords, err := provider.getRecords(domain)
		if err != nil {
			fmt.Printf("  ⚠ Warning: Could not fetch existing records: %v\n", err)
		} else {
			fmt.Println("\n  Current DNS Records:")
			if len(existingRecords) == 0 {
				fmt.Println("    (no records found)")
			} else {
				for _, r := range existingRecords {
					name := r.Name
					if name == "" || name == "@" {
						name = domain
					} else {
						name = name + "." + domain
					}
					fmt.Printf("    %-35s %-6s %5d  %s\n", name, r.Type, r.TTL, r.Content)
				}
			}
		}

		var recordsToCreate []HostingerRecord

		if providerType == "local" {
			// Local DNS (PowerDNS) - create NS records and update nameservers
			fmt.Println("\n→ Configuring NS records for local PowerDNS...")
			if err := provider.EnsureNSRecords(domain, serverIP); err != nil {
				return fmt.Errorf("failed to configure NS records: %w", err)
			}

			fmt.Println("→ Updating domain nameservers...")
			if err := provider.UpdateNameservers(domain); err != nil {
				return fmt.Errorf("failed to update nameservers: %w", err)
			}
		} else if providerType == "external" {
			// External DNS - reset nameservers and add A/MX records only
			fmt.Println("\n→ Resetting nameservers to Hostinger defaults...")
			if err := provider.ResetNameserversToDefault(domain); err != nil {
				fmt.Printf("  ⚠ Warning: Could not reset nameservers: %v\n", err)
			}

			fmt.Println("→ Configuring DNS records (A and MX only)...")

			// Build map of existing records
			existingByNameType := make(map[string]HostingerRecord)
			for _, r := range existingRecords {
				key := fmt.Sprintf("%s:%s", r.Name, r.Type)
				existingByNameType[key] = r
			}

			// Helper function
			shouldSkip := func(name, recordType string) (bool, string) {
				key := fmt.Sprintf("%s:%s", name, recordType)
				if existing, ok := existingByNameType[key]; ok {
					if existing.Content == serverIP || (recordType == "MX" && strings.Contains(existing.Content, fmt.Sprintf("mail.%s", domain))) {
						return true, "already exists with correct value"
					}
					return false, ""
				}
				if recordType == "A" {
					cnameKey := fmt.Sprintf("%s:CNAME", name)
					if _, ok := existingByNameType[cnameKey]; ok {
						return true, "CNAME exists, skipping A record"
					}
				}
				return false, ""
			}

			// A record for root domain
			if skip, reason := shouldSkip("@", "A"); skip {
				fmt.Printf("  ℹ %s A: %s\n", domain, reason)
			} else {
				recordsToCreate = append(recordsToCreate, HostingerRecord{
					Type:    "A",
					Name:    "@",
					Content: serverIP,
					TTL:     300,
				})
			}

			// A records for subdomains
			for _, sub := range domainCfg.Subdomains {
				if skip, reason := shouldSkip(sub.Name, "A"); skip {
					fmt.Printf("  ℹ %s.%s A: %s\n", sub.Name, domain, reason)
				} else {
					recordsToCreate = append(recordsToCreate, HostingerRecord{
						Type:    "A",
						Name:    sub.Name,
						Content: serverIP,
						TTL:     300,
					})
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
				mxContent := fmt.Sprintf("10 mail.%s", domain)
				if skip, reason := shouldSkip("@", "MX"); skip {
					fmt.Printf("  ℹ %s MX: %s\n", domain, reason)
				} else {
					recordsToCreate = append(recordsToCreate, HostingerRecord{
						Type:    "MX",
						Name:    "@",
						Content: mxContent,
						TTL:     14400,
					})
				}
			}

			// Create records
			if len(recordsToCreate) > 0 {
				fmt.Println("\n  Creating/updating DNS records:")
				for _, r := range recordsToCreate {
					name := r.Name
					if name == "@" {
						name = domain
					} else {
						name = name + "." + domain
					}

					var err error
					if r.Type == "A" {
						err = provider.replaceRecords(domain, []HostingerRecord{r})
					} else {
						err = provider.updateRecords(domain, []HostingerRecord{r})
					}

					if err != nil {
						fmt.Printf("    ✗ %s %s %s - %v\n", name, r.Type, r.Content, err)
					} else {
						fmt.Printf("    ✓ %s %s %s\n", name, r.Type, r.Content)
					}
				}
			} else {
				fmt.Println("  ℹ All DNS records already configured correctly")
			}
		}

		fmt.Println("\n✓ [PRE-INSTALL] DNS configured - waiting for propagation...")
		return nil

	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}

// UpdateDNSRecordsPostInstall configures SPF and DMARC records AFTER Ansible playbook execution
// This is called after the mail server is configured
func UpdateDNSRecordsPostInstall(providerName, providerType, apiToken string, domainCfg DomainConfig, serverIP string) error {
	if providerName == "" || apiToken == "" || providerType != "external" {
		return nil
	}

	providerName = strings.ToLower(strings.TrimSpace(providerName))
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

	fmt.Printf("\n🌐 [POST-INSTALL] Configuring mail DNS records for %s...\n", domain)

	switch providerName {
	case "hostinger":
		provider := NewHostingerProvider(apiToken)

		existingRecords, err := provider.getRecords(domain)
		if err != nil {
			fmt.Printf("  ⚠ Warning: Could not fetch existing records: %v\n", err)
			existingRecords = []HostingerRecord{}
		}

		var recordsToCreate []HostingerRecord

		// Check SPF record
		spfContent := fmt.Sprintf("v=spf1 ip4:%s -all", serverIP)
		hasSPF := false
		for _, r := range existingRecords {
			if r.Type == "TXT" && r.Name == "@" && strings.HasPrefix(r.Content, "v=spf1") {
				hasSPF = true
				if strings.Contains(r.Content, serverIP) {
					fmt.Printf("  ℹ %s SPF: already configured\n", domain)
				} else {
					fmt.Printf("  ⚠ %s SPF: exists but doesn't include %s, please update manually\n", domain, serverIP)
				}
				break
			}
		}
		if !hasSPF {
			recordsToCreate = append(recordsToCreate, HostingerRecord{
				Type:    "TXT",
				Name:    "@",
				Content: spfContent,
				TTL:     14400,
			})
		}

		// Check DMARC record
		// Convert domain to Punycode for email address in DMARC record (RFC compliance)
		domainASCII, err := toPunycode(domain)
		if err != nil {
			domainASCII = domain // Fallback to original if conversion fails
		}
		dmarcContent := fmt.Sprintf("v=DMARC1; p=none; rua=mailto:dmarc@%s", domainASCII)
		hasDMARC := false
		for _, r := range existingRecords {
			if r.Type == "TXT" && r.Name == "_dmarc" {
				hasDMARC = true
				fmt.Printf("  ℹ %s DMARC: already configured\n", domain)
				break
			}
		}
		if !hasDMARC {
			recordsToCreate = append(recordsToCreate, HostingerRecord{
				Type:    "TXT",
				Name:    "_dmarc",
				Content: dmarcContent,
				TTL:     3600,
			})
		}

		// Create records
		if len(recordsToCreate) > 0 {
			fmt.Println("\n  Creating mail DNS records:")
			for _, r := range recordsToCreate {
				name := r.Name
				if name == "@" {
					name = domain
				} else {
					name = name + "." + domain
				}

				if err := provider.updateRecords(domain, []HostingerRecord{r}); err != nil {
					fmt.Printf("    ✗ %s %s - %v\n", name, r.Type, err)
				} else {
					fmt.Printf("    ✓ %s %s\n", name, r.Type)
				}
			}
		} else {
			fmt.Println("  ℹ All mail DNS records already configured")
		}

		fmt.Println("\n✓ [POST-INSTALL] Mail DNS records configured")
		return nil

	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}

// VerifyDNSPropagation verifies that all DNS records have propagated for a domain
// It checks the root domain and all subdomains to ensure they resolve to the server IP
// Returns nil if all records are propagated, or an error with details
func VerifyDNSPropagation(domainCfg DomainConfig, serverIP string, maxRetries int, delaySeconds int) error {
	domain := domainCfg.Name

	fmt.Printf("\n🔍 Verifying DNS propagation for %s...\n", domain)
	fmt.Printf("   Server IP: %s\n", serverIP)
	fmt.Printf("   Max attempts: %d (checking every %d seconds)\n\n", maxRetries, delaySeconds)

	// Build list of FQDNs to check
	var fqdnsToCheck []string

	// Add root domain
	fqdnsToCheck = append(fqdnsToCheck, domain)

	// Add all subdomains
	for _, sub := range domainCfg.Subdomains {
		fqdnsToCheck = append(fqdnsToCheck, fmt.Sprintf("%s.%s", sub.Name, domain))
	}

	// Track which domains are not yet propagated
	pendingDomains := make(map[string]bool)
	for _, fqdn := range fqdnsToCheck {
		pendingDomains[fqdn] = true
	}

	// Retry loop
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("\n⏳ Attempt %d/%d - Checking DNS propagation...\n", attempt, maxRetries)
		}

		// Check each pending domain
		for fqdn := range pendingDomains {
			resolved, err := checkDNSRecord(fqdn, serverIP)
			if err != nil {
				fmt.Printf("   ⚠ %s: %v\n", fqdn, err)
				continue
			}

			if resolved {
				fmt.Printf("   ✓ %s → %s\n", fqdn, serverIP)
				delete(pendingDomains, fqdn)
			} else {
				fmt.Printf("   ⏱ %s: waiting for propagation...\n", fqdn)
			}
		}

		// If all domains are resolved, we're done
		if len(pendingDomains) == 0 {
			fmt.Printf("\n✓ DNS propagation complete! All %d records verified.\n", len(fqdnsToCheck))
			return nil
		}

		// If not the last attempt, wait before retrying
		if attempt < maxRetries {
			fmt.Printf("\n   Waiting %d seconds before retry...\n", delaySeconds)
			time.Sleep(time.Duration(delaySeconds) * time.Second)
		}
	}

	// If we get here, some domains didn't propagate in time
	var pendingList []string
	for fqdn := range pendingDomains {
		pendingList = append(pendingList, fqdn)
	}

	return fmt.Errorf("DNS propagation timeout: %d domain(s) did not propagate after %d attempts:\n  - %s\n\nPlease verify your DNS configuration and try again.",
		len(pendingList),
		maxRetries,
		strings.Join(pendingList, "\n  - "))
}

// checkDNSRecord checks if a FQDN resolves to the expected IP address
func checkDNSRecord(fqdn, expectedIP string) (bool, error) {
	// Convert FQDN to punycode for dig compatibility (handles accented domains)
	fqdnASCII, err := toPunycode(fqdn)
	if err != nil {
		return false, fmt.Errorf("invalid domain name: %w", err)
	}

	// Debug: log the conversion
	if fqdn != fqdnASCII {
		fmt.Printf("  [DEBUG] Converted %s to %s\n", fqdn, fqdnASCII)
	}

	// First, get the authoritative nameservers for the domain
	// Extract the base domain from FQDN (e.g., xn--tiuntigre-c4a.fr from adm.xn--tiuntigre-c4a.fr)
	parts := strings.Split(fqdnASCII, ".")
	var baseDomain string
	if len(parts) >= 2 {
		baseDomain = parts[len(parts)-2] + "." + parts[len(parts)-1]
	} else {
		baseDomain = fqdnASCII
	}

	fmt.Printf("  [DEBUG] Base domain: %s, Checking: %s\n", baseDomain, fqdnASCII)

	// Query authoritative nameservers first using Google DNS (8.8.8.8)
	// Local DNS may refuse queries, so we use a public resolver
	nsCmd := exec.Command("dig", "@8.8.8.8", "+short", baseDomain, "NS")
	nsOutput, err := nsCmd.Output()
	fmt.Printf("  [DEBUG] NS query for %s: %s\n", baseDomain, strings.TrimSpace(string(nsOutput)))

	if err == nil && len(strings.TrimSpace(string(nsOutput))) > 0 {
		// Get first nameserver
		nsLines := strings.Split(strings.TrimSpace(string(nsOutput)), "\n")
		if len(nsLines) > 0 && nsLines[0] != "" {
			ns := strings.TrimSpace(nsLines[0])
			fmt.Printf("  [DEBUG] Querying @%s for %s A\n", ns, fqdnASCII)

			// Query using authoritative nameserver
			cmd := exec.Command("dig", "@"+ns, "+short", fqdnASCII, "A")
			output, err := cmd.Output()

			fmt.Printf("  [DEBUG] Response: %s (err: %v)\n", strings.TrimSpace(string(output)), err)

			if err == nil {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				if len(lines) > 0 && lines[0] != "" {
					// Check if any of the returned IPs match our expected IP
					for _, ip := range lines {
						ip = strings.TrimSpace(ip)
						if ip == expectedIP {
							fmt.Printf("  [DEBUG] ✓ Match found: %s\n", ip)
							return true, nil
						}
					}
					// DNS record exists but points to wrong IP
					return false, fmt.Errorf("resolves to %s, expected %s", strings.Join(lines, ", "), expectedIP)
				}
			}
		}
	}

	// Fallback to regular query (using default resolvers)
	cmd := exec.Command("dig", "+short", fqdnASCII, "A")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("dig command failed: %w", err)
	}

	// Parse output - dig returns one IP per line (may include CNAME first)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return false, nil // No DNS record found yet
	}

	// Check if any of the returned IPs match our expected IP
	for _, ip := range lines {
		ip = strings.TrimSpace(ip)
		if ip == expectedIP {
			return true, nil
		}
	}

	// DNS record exists but points to wrong IP
	return false, fmt.Errorf("resolves to %s, expected %s", strings.Join(lines, ", "), expectedIP)
}
