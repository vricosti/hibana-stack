package dnsprovider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
			// External DNS - only add A records for subdomains, MX, SPF, DMARC
			fmt.Println("\n→ Configuring DNS records for external provider...")

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
				dmarcContent := fmt.Sprintf("v=DMARC1; p=none; rua=mailto:dmarc@%s", domain)
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

					if err := provider.updateRecords(domain, []HostingerRecord{r}); err != nil {
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
	url := fmt.Sprintf("%s/virtual-machines/%d/ptr/%d", hostingerVPSAPIURL, vmID, ipAddressID)

	payload := map[string]string{
		"domain": domain,
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

	// Configure PTR for IPv4 addresses
	for _, ip := range vm.IPv4 {
		fmt.Printf("→ Configuring PTR for %s (IPv4)...\n", ip.Address)

		if ip.PTR == mailHostname {
			fmt.Printf("  ℹ PTR already configured: %s → %s\n", ip.Address, mailHostname)
			continue
		}

		if err := h.CreatePTRRecord(vm.ID, ip.ID, mailHostname); err != nil {
			fmt.Printf("  ⚠ Warning: Failed to set PTR for %s: %v\n", ip.Address, err)
			continue
		}

		fmt.Printf("  ✓ PTR record set: %s → %s\n", ip.Address, mailHostname)
	}

	// Configure PTR for IPv6 addresses
	for _, ip := range vm.IPv6 {
		fmt.Printf("→ Configuring PTR for %s (IPv6)...\n", ip.Address)

		if ip.PTR == mailHostname {
			fmt.Printf("  ℹ PTR already configured: %s → %s\n", ip.Address, mailHostname)
			continue
		}

		if err := h.CreatePTRRecord(vm.ID, ip.ID, mailHostname); err != nil {
			fmt.Printf("  ⚠ Warning: Failed to set PTR for %s: %v\n", ip.Address, err)
			continue
		}

		fmt.Printf("  ✓ PTR record set: %s → %s\n", ip.Address, mailHostname)
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
			fmt.Println("\n⚠️  Please configure PTR records manually in Hostinger hPanel:")
			fmt.Println("    → VPS Settings → IP address → Set PTR record")
			fmt.Printf("    → IPv4: %s\n", mailHostname)
			fmt.Printf("    → IPv6: %s\n", mailHostname)
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
	keyFile := fmt.Sprintf("/etc/opendkim/keys/%s/default.txt", domain)

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
