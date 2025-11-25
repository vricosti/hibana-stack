package dnsprovider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	// Check if domain is in the portfolio
	for _, d := range domains {
		if d.Domain == domain {
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
	req, err := http.NewRequest("GET", hostingerDNSAPIURL+"/zones/"+domain, nil)
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
		Overwrite: false, // Append/update, don't replace all records
		Zone:      zoneRecords,
	}

	payload, err := json.Marshal(updateReq)
	if err != nil {
		return err
	}

	fmt.Printf("  DEBUG: Sending to API: %s\n", string(payload))

	req, err := http.NewRequest("PUT", hostingerDNSAPIURL+"/zones/"+domain, bytes.NewBuffer(payload))
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

// UpdateNameservers updates the domain's nameservers to use ns1 and ns2
// It first checks if they are already correctly configured
func (h *HostingerProvider) UpdateNameservers(domain string) error {
	desiredNS1 := fmt.Sprintf("ns1.%s", domain)
	desiredNS2 := fmt.Sprintf("ns2.%s", domain)

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

	req, err := http.NewRequest("PUT", hostingerDomainsAPIURL+"/portfolio/"+domain+"/nameservers", bytes.NewBuffer(payloadBytes))
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
	// Use the domain details endpoint which includes nameservers
	req, err := http.NewRequest("GET", hostingerDomainsAPIURL+"/portfolio/"+domain, nil)
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

// UpdateDNSRecords updates DNS records for the given domain and server IP
// If simulate is true, it will only log what would be done without making actual API calls
func UpdateDNSRecords(providerName, apiToken, domain, serverIP string, simulate bool) error {
	if providerName == "" || apiToken == "" {
		// DNS provider not configured, skip
		return nil
	}

	// Normalize provider name (case insensitive)
	providerName = strings.ToLower(strings.TrimSpace(providerName))

	fmt.Println("\n🌐 Configuring DNS provider...")

	switch providerName {
	case "hostinger":
		provider := NewHostingerProvider(apiToken)

		if simulate {
			// Simulation mode - fetch existing data and show what would be done
			fmt.Println("→ [SIMULATION] Fetching current DNS configuration...")

			// Fetch existing DNS records
			existingRecords, err := provider.getRecords(domain)
			if err != nil {
				fmt.Printf("  ⚠ Warning: Could not fetch existing records: %v\n", err)
			}

			// Fetch current nameservers
			currentNameservers, err := provider.GetNameservers(domain)
			if err != nil {
				fmt.Printf("  ⚠ Warning: Could not fetch current nameservers: %v\n", err)
			}

			// Display current state
			fmt.Println("\n→ [SIMULATION] Current DNS configuration:")

			// Show current NS records
			if existingRecords != nil {
				var ns1Record, ns2Record *HostingerRecord
				for i := range existingRecords {
					if existingRecords[i].Type == "A" && existingRecords[i].Name == "ns1" {
						ns1Record = &existingRecords[i]
					}
					if existingRecords[i].Type == "A" && existingRecords[i].Name == "ns2" {
						ns2Record = &existingRecords[i]
					}
				}

				fmt.Println("  NS Records:")
				if ns1Record != nil {
					fmt.Printf("    • ns1.%s A %s (TTL: %d)\n", domain, ns1Record.Content, ns1Record.TTL)
				} else {
					fmt.Printf("    • ns1.%s - not configured\n", domain)
				}
				if ns2Record != nil {
					fmt.Printf("    • ns2.%s A %s (TTL: %d)\n", domain, ns2Record.Content, ns2Record.TTL)
				} else {
					fmt.Printf("    • ns2.%s - not configured\n", domain)
				}
			}

			// Show current nameservers
			if currentNameservers != nil && len(currentNameservers) > 0 {
				fmt.Println("  Domain Nameservers:")
				for _, ns := range currentNameservers {
					fmt.Printf("    • %s\n", ns)
				}
			} else {
				fmt.Println("  Domain Nameservers: none configured")
			}

			// Display what would be changed
			fmt.Println("\n→ [SIMULATION] Planned changes:")

			if existingRecords != nil {
				var ns1Record, ns2Record *HostingerRecord
				for i := range existingRecords {
					if existingRecords[i].Type == "A" && existingRecords[i].Name == "ns1" {
						ns1Record = &existingRecords[i]
					}
					if existingRecords[i].Type == "A" && existingRecords[i].Name == "ns2" {
						ns2Record = &existingRecords[i]
					}
				}

				hasChanges := false
				if ns1Record == nil {
					fmt.Printf("  • Would create: ns1.%s A %s (TTL: 14400)\n", domain, serverIP)
					hasChanges = true
				}
				if ns2Record == nil {
					fmt.Printf("  • Would create: ns2.%s A %s (TTL: 14400)\n", domain, serverIP)
					hasChanges = true
				}

				// Check if nameservers need update
				desiredNS := []string{fmt.Sprintf("ns1.%s", domain), fmt.Sprintf("ns2.%s", domain)}
				needsNSUpdate := false
				if len(currentNameservers) != len(desiredNS) {
					needsNSUpdate = true
				} else {
					for i, ns := range currentNameservers {
						if i >= len(desiredNS) || ns != desiredNS[i] {
							needsNSUpdate = true
							break
						}
					}
				}

				if needsNSUpdate {
					fmt.Printf("  • Would update domain nameservers to: ns1.%s, ns2.%s\n", domain, domain)
					hasChanges = true
				}

				if !hasChanges {
					fmt.Println("  • No changes needed - DNS is already configured correctly")
				}
			}

			fmt.Println("\n✓ DNS provider configuration simulated successfully")
			return nil
		}

		// Real mode - make actual API calls
		// First, display current configuration
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

		currentNameservers, err := provider.GetNameservers(domain)
		if err != nil {
			fmt.Printf("  ⚠ Warning: Could not fetch current nameservers: %v\n", err)
		} else {
			fmt.Println("\n  Current Nameservers:")
			if len(currentNameservers) == 0 {
				fmt.Println("    (no nameservers configured)")
			} else {
				for i, ns := range currentNameservers {
					fmt.Printf("    %d. %s\n", i+1, ns)
				}
			}
		}

		fmt.Println()

		// Step 1: Create ns1 and ns2 A records
		fmt.Println("→ Configuring NS records...")
		if err := provider.EnsureNSRecords(domain, serverIP); err != nil {
			return fmt.Errorf("failed to configure NS records: %w", err)
		}

		// Step 2: Update domain nameservers
		fmt.Println("→ Updating domain nameservers...")
		if err := provider.UpdateNameservers(domain); err != nil {
			return fmt.Errorf("failed to update nameservers: %w", err)
		}

		fmt.Println("✓ DNS provider configured successfully")
		return nil
	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}
