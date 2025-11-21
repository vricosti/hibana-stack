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
	hostingerAPIURL = "https://api.hostinger.com/dns/v1"
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

// HostingerZone represents a DNS zone in Hostinger
type HostingerZone struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

// HostingerRecord represents a DNS record in Hostinger
type HostingerRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

// EnsureNSRecords ensures that ns1 and ns2 A records exist for the domain
func (h *HostingerProvider) EnsureNSRecords(domain, serverIP string) error {
	// Get zone ID for the domain
	zoneID, err := h.getZoneID(domain)
	if err != nil {
		return fmt.Errorf("failed to get zone ID for %s: %w", domain, err)
	}

	// Get existing records
	existingRecords, err := h.getRecords(zoneID)
	if err != nil {
		return fmt.Errorf("failed to get existing records: %w", err)
	}

	// Check if ns1 and ns2 records exist
	hasNS1 := false
	hasNS2 := false

	for _, record := range existingRecords {
		if record.Type == "A" && record.Name == "ns1" {
			hasNS1 = true
		}
		if record.Type == "A" && record.Name == "ns2" {
			hasNS2 = true
		}
	}

	// Create missing records
	if !hasNS1 {
		if err := h.createRecord(zoneID, HostingerRecord{
			Type:    "A",
			Name:    "ns1",
			Content: serverIP,
			TTL:     14400,
		}); err != nil {
			return fmt.Errorf("failed to create ns1 record: %w", err)
		}
		fmt.Println("✓ Created DNS record: ns1 A " + serverIP)
	} else {
		fmt.Println("✓ DNS record already exists: ns1 A")
	}

	if !hasNS2 {
		if err := h.createRecord(zoneID, HostingerRecord{
			Type:    "A",
			Name:    "ns2",
			Content: serverIP,
			TTL:     14400,
		}); err != nil {
			return fmt.Errorf("failed to create ns2 record: %w", err)
		}
		fmt.Println("✓ Created DNS record: ns2 A " + serverIP)
	} else {
		fmt.Println("✓ DNS record already exists: ns2 A")
	}

	return nil
}

// getZoneID retrieves the zone ID for a given domain
func (h *HostingerProvider) getZoneID(domain string) (string, error) {
	req, err := http.NewRequest("GET", hostingerAPIURL+"/zones", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var zones []HostingerZone
	if err := json.Unmarshal(body, &zones); err != nil {
		return "", fmt.Errorf("failed to parse zones response: %w", err)
	}

	// Find the zone for the domain
	for _, zone := range zones {
		if zone.Domain == domain {
			return zone.ID, nil
		}
	}

	return "", fmt.Errorf("zone not found for domain %s", domain)
}

// getRecords retrieves all DNS records for a zone
func (h *HostingerProvider) getRecords(zoneID string) ([]HostingerRecord, error) {
	req, err := http.NewRequest("GET", hostingerAPIURL+"/zones/"+zoneID+"/records", nil)
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

	var records []HostingerRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("failed to parse records response: %w", err)
	}

	return records, nil
}

// createRecord creates a new DNS record in the zone
func (h *HostingerProvider) createRecord(zoneID string, record HostingerRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", hostingerAPIURL+"/zones/"+zoneID+"/records", bytes.NewBuffer(payload))
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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UpdateDNSRecords updates DNS records for the given domain and server IP
func UpdateDNSRecords(providerName, apiToken, domain, serverIP string) error {
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
		if err := provider.EnsureNSRecords(domain, serverIP); err != nil {
			return err
		}
		fmt.Println("✓ DNS records configured successfully")
		return nil
	default:
		return fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}
