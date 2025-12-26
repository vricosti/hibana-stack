package services

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DNSProviderService handles DNS provider operations
type DNSProviderService struct {
	db     *sql.DB
	client *http.Client
}

// NewDNSProviderService creates a new DNS provider service
func NewDNSProviderService(db *sql.DB) *DNSProviderService {
	return &DNSProviderService{
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// DNSProvider represents a DNS provider in the database
type DNSProvider struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`     // "local" or "external"
	Provider          string    `json:"provider"` // "powerdns", "hostinger", "cloudflare", "ovh"
	APIToken          string    `json:"api_token,omitempty"`
	Endpoint          string    `json:"endpoint,omitempty"`
	ApplicationKey    string    `json:"application_key,omitempty"`
	ApplicationSecret string    `json:"application_secret,omitempty"`
	ConsumerKey       string    `json:"consumer_key,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// DNSProviderCreate represents data for creating a DNS provider
type DNSProviderCreate struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	Provider          string `json:"provider"`
	APIToken          string `json:"api_token,omitempty"`
	Endpoint          string `json:"endpoint,omitempty"`
	ApplicationKey    string `json:"application_key,omitempty"`
	ApplicationSecret string `json:"application_secret,omitempty"`
	ConsumerKey       string `json:"consumer_key,omitempty"`
}

// AvailableDomain represents a domain available from a DNS provider
type AvailableDomain struct {
	Domain     string `json:"domain"`
	Status     string `json:"status,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	Target     string `json:"target,omitempty"`
	RecordType string `json:"record_type,omitempty"`
	Available  bool   `json:"available"`
	Managed    bool   `json:"managed"` // true if domain is already managed by the server
}

// GetAll returns all DNS providers
func (s *DNSProviderService) GetAll() ([]DNSProvider, error) {
	query := `SELECT id, name, type, provider, api_token, endpoint, application_key, application_secret, consumer_key, created_at, updated_at FROM dns_providers ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query dns providers: %w", err)
	}
	defer rows.Close()

	var providers []DNSProvider
	for rows.Next() {
		var p DNSProvider
		var apiToken, endpoint, appKey, appSecret, consumerKey sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Provider, &apiToken, &endpoint, &appKey, &appSecret, &consumerKey, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan dns provider: %w", err)
		}
		if apiToken.Valid {
			p.APIToken = apiToken.String
		}
		if endpoint.Valid {
			p.Endpoint = endpoint.String
		}
		if appKey.Valid {
			p.ApplicationKey = appKey.String
		}
		if appSecret.Valid {
			p.ApplicationSecret = appSecret.String
		}
		if consumerKey.Valid {
			p.ConsumerKey = consumerKey.String
		}
		providers = append(providers, p)
	}

	return providers, nil
}

// GetByID returns a DNS provider by ID
func (s *DNSProviderService) GetByID(id int) (*DNSProvider, error) {
	query := `SELECT id, name, type, provider, api_token, endpoint, application_key, application_secret, consumer_key, created_at, updated_at FROM dns_providers WHERE id = $1`

	var p DNSProvider
	var apiToken, endpoint, appKey, appSecret, consumerKey sql.NullString
	err := s.db.QueryRow(query, id).Scan(&p.ID, &p.Name, &p.Type, &p.Provider, &apiToken, &endpoint, &appKey, &appSecret, &consumerKey, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dns provider not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query dns provider: %w", err)
	}

	if apiToken.Valid {
		p.APIToken = apiToken.String
	}
	if endpoint.Valid {
		p.Endpoint = endpoint.String
	}
	if appKey.Valid {
		p.ApplicationKey = appKey.String
	}
	if appSecret.Valid {
		p.ApplicationSecret = appSecret.String
	}
	if consumerKey.Valid {
		p.ConsumerKey = consumerKey.String
	}

	return &p, nil
}

// Create creates a new DNS provider
func (s *DNSProviderService) Create(create *DNSProviderCreate) (*DNSProvider, error) {
	query := `INSERT INTO dns_providers (name, type, provider, api_token, endpoint, application_key, application_secret, consumer_key)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`

	var id int
	err := s.db.QueryRow(query,
		create.Name,
		create.Type,
		create.Provider,
		nullString(create.APIToken),
		nullString(create.Endpoint),
		nullString(create.ApplicationKey),
		nullString(create.ApplicationSecret),
		nullString(create.ConsumerKey),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create dns provider: %w", err)
	}

	// Update the global configuration
	s.updateConfigTable(create.Type, create.Provider)

	return s.GetByID(id)
}

// Update updates an existing DNS provider
func (s *DNSProviderService) Update(id int, update *DNSProviderCreate) (*DNSProvider, error) {
	// Check if provider exists
	_, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	query := `UPDATE dns_providers
	          SET name = $1, type = $2, provider = $3, api_token = $4, endpoint = $5,
	              application_key = $6, application_secret = $7, consumer_key = $8, updated_at = CURRENT_TIMESTAMP
	          WHERE id = $9`

	_, err = s.db.Exec(query,
		update.Name,
		update.Type,
		update.Provider,
		nullString(update.APIToken),
		nullString(update.Endpoint),
		nullString(update.ApplicationKey),
		nullString(update.ApplicationSecret),
		nullString(update.ConsumerKey),
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update dns provider: %w", err)
	}

	// Update the global configuration
	s.updateConfigTable(update.Type, update.Provider)

	return s.GetByID(id)
}

// Delete deletes a DNS provider
func (s *DNSProviderService) Delete(id int) error {
	query := `DELETE FROM dns_providers WHERE id = $1`
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete dns provider: %w", err)
	}
	return nil
}

// GetAvailableDomains fetches domains from a DNS provider
func (s *DNSProviderService) GetAvailableDomains(providerID int) ([]AvailableDomain, error) {
	provider, err := s.GetByID(providerID)
	if err != nil {
		return nil, err
	}

	switch provider.Provider {
	case "hostinger":
		return s.getHostingerDomains(provider.APIToken)
	case "ovh":
		return s.getOVHDomains(provider)
	case "powerdns":
		// For local PowerDNS, return empty - domains are created locally
		return []AvailableDomain{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider.Provider)
	}
}

// getHostingerDomains fetches domains from Hostinger API
func (s *DNSProviderService) getHostingerDomains(apiToken string) ([]AvailableDomain, error) {
	req, err := http.NewRequest("GET", "https://developers.hostinger.com/api/domains/v1/portfolio", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch domains: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed: invalid API token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var hostingerDomains []struct {
		Domain    string `json:"domain"`
		Status    string `json:"status"`
		ExpiresAt string `json:"expires_at"`
	}

	if err := json.Unmarshal(body, &hostingerDomains); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Get already configured domains from our database
	configuredDomains := make(map[string]bool)
	rows, err := s.db.Query("SELECT name FROM domains")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			rows.Scan(&name)
			configuredDomains[name] = true
		}
	}

	// Build list of all domains with DNS info
	var available []AvailableDomain
	for _, d := range hostingerDomains {
		ad := AvailableDomain{
			Domain:    d.Domain,
			Status:    d.Status,
			ExpiresAt: d.ExpiresAt,
			Managed:   configuredDomains[d.Domain], // Mark if already managed
		}

		// Fetch DNS records for this domain to get the target
		target, recordType := s.getHostingerDomainTarget(apiToken, d.Domain)
		ad.Target = target
		ad.RecordType = recordType
		ad.Available = isHostingerParkingIP(target) || configuredDomains[d.Domain]

		available = append(available, ad)
	}

	return available, nil
}

// isHostingerParkingIP checks if the IP is in Hostinger's parking range (84.32.84.0 - 84.32.84.255)
func isHostingerParkingIP(ip string) bool {
	if ip == "" || ip == "-" {
		return true // No IP configured = available
	}
	// Check if IP starts with 84.32.84.
	return strings.HasPrefix(ip, "84.32.84.")
}

// getHostingerDomainTarget fetches the root A or CNAME record for a domain
func (s *DNSProviderService) getHostingerDomainTarget(apiToken, domain string) (string, string) {
	req, err := http.NewRequest("GET", "https://developers.hostinger.com/api/dns/v1/zones/"+domain, nil)
	if err != nil {
		return "-", ""
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "-", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "-", ""
	}

	body, _ := io.ReadAll(resp.Body)

	var records []struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Records []struct {
			Content string `json:"content"`
		} `json:"records"`
	}

	if err := json.Unmarshal(body, &records); err != nil {
		return "-", ""
	}

	// Look for root A or CNAME record (name = "@" or empty)
	for _, r := range records {
		if r.Name == "@" || r.Name == "" {
			if (r.Type == "A" || r.Type == "CNAME") && len(r.Records) > 0 {
				return r.Records[0].Content, r.Type
			}
		}
	}

	return "-", ""
}

// getOVHDomains fetches domains from OVH API
func (s *DNSProviderService) getOVHDomains(provider *DNSProvider) ([]AvailableDomain, error) {
	// Get list of zones from OVH
	zones, err := s.ovhAPICall("GET", "/domain/zone", provider, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OVH zones: %w", err)
	}

	var zoneList []string
	if err := json.Unmarshal(zones, &zoneList); err != nil {
		return nil, fmt.Errorf("failed to parse OVH zones: %w", err)
	}

	// Get already configured domains from our database
	configuredDomains := make(map[string]bool)
	rows, err := s.db.Query("SELECT name FROM domains")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			rows.Scan(&name)
			configuredDomains[name] = true
		}
	}

	// Build list of domains
	var available []AvailableDomain
	for _, zone := range zoneList {
		ad := AvailableDomain{
			Domain:    zone,
			Status:    "active",
			Managed:   configuredDomains[zone],
			Available: true,
		}

		// Get A record for root domain
		target, recordType := s.getOVHDomainTarget(provider, zone)
		ad.Target = target
		ad.RecordType = recordType

		available = append(available, ad)
	}

	return available, nil
}

// getOVHDomainTarget fetches the root A record for a domain from OVH
func (s *DNSProviderService) getOVHDomainTarget(provider *DNSProvider, domain string) (string, string) {
	// Get A records for root
	recordsData, err := s.ovhAPICall("GET", fmt.Sprintf("/domain/zone/%s/record?fieldType=A&subDomain=", domain), provider, nil)
	if err != nil {
		return "-", ""
	}

	var recordIDs []int64
	if err := json.Unmarshal(recordsData, &recordIDs); err != nil || len(recordIDs) == 0 {
		return "-", ""
	}

	// Get first record details
	recordData, err := s.ovhAPICall("GET", fmt.Sprintf("/domain/zone/%s/record/%d", domain, recordIDs[0]), provider, nil)
	if err != nil {
		return "-", ""
	}

	var record struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(recordData, &record); err != nil {
		return "-", ""
	}

	return record.Target, "A"
}

// testOVHConnection tests connection to OVH API
func (s *DNSProviderService) testOVHConnection(create *DNSProviderCreate) error {
	provider := &DNSProvider{
		Endpoint:          create.Endpoint,
		ApplicationKey:    create.ApplicationKey,
		ApplicationSecret: create.ApplicationSecret,
		ConsumerKey:       create.ConsumerKey,
	}

	_, err := s.ovhAPICall("GET", "/domain/zone", provider, nil)
	if err != nil {
		return fmt.Errorf("OVH connection failed: %w", err)
	}
	return nil
}

// ovhAPICall makes an authenticated call to OVH API
func (s *DNSProviderService) ovhAPICall(method, path string, provider *DNSProvider, body []byte) ([]byte, error) {
	endpoint := provider.Endpoint
	if endpoint == "" {
		endpoint = "ovh-eu"
	}

	// Map endpoint to base URL
	baseURLs := map[string]string{
		"ovh-eu":        "https://eu.api.ovh.com/1.0",
		"ovh-ca":        "https://ca.api.ovh.com/1.0",
		"ovh-us":        "https://api.us.ovhcloud.com/1.0",
		"soyoustart-eu": "https://eu.api.soyoustart.com/1.0",
		"soyoustart-ca": "https://ca.api.soyoustart.com/1.0",
		"kimsufi-eu":    "https://eu.api.kimsufi.com/1.0",
		"kimsufi-ca":    "https://ca.api.kimsufi.com/1.0",
	}

	baseURL, ok := baseURLs[endpoint]
	if !ok {
		baseURL = baseURLs["ovh-eu"]
	}

	url := baseURL + path

	// Get current timestamp
	timeResp, err := http.Get(baseURL + "/auth/time")
	if err != nil {
		return nil, fmt.Errorf("failed to get OVH time: %w", err)
	}
	defer timeResp.Body.Close()
	timeBody, _ := io.ReadAll(timeResp.Body)
	timestamp := strings.TrimSpace(string(timeBody))

	// Build signature
	// Signature = "$1$" + SHA1(AS+"+"+CK+"+"+METHOD+"+"+QUERY+"+"+BODY+"+"+TSTAMP)
	bodyStr := ""
	if body != nil {
		bodyStr = string(body)
	}
	toSign := fmt.Sprintf("%s+%s+%s+%s+%s+%s",
		provider.ApplicationSecret,
		provider.ConsumerKey,
		method,
		url,
		bodyStr,
		timestamp,
	)

	h := sha1.New()
	h.Write([]byte(toSign))
	signature := "$1$" + fmt.Sprintf("%x", h.Sum(nil))

	// Create request
	var reqBody io.Reader
	if body != nil {
		reqBody = strings.NewReader(string(body))
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Ovh-Application", provider.ApplicationKey)
	req.Header.Set("X-Ovh-Timestamp", timestamp)
	req.Header.Set("X-Ovh-Signature", signature)
	req.Header.Set("X-Ovh-Consumer", provider.ConsumerKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("OVH API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// TestConnection tests the connection to a DNS provider
func (s *DNSProviderService) TestConnection(create *DNSProviderCreate) error {
	switch create.Provider {
	case "hostinger":
		req, err := http.NewRequest("GET", "https://developers.hostinger.com/api/domains/v1/portfolio", nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+create.APIToken)
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("authentication failed: invalid API token")
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API error: %s", string(body))
		}
		return nil

	case "ovh":
		return s.testOVHConnection(create)

	case "powerdns":
		// Local PowerDNS is always available
		return nil

	default:
		return fmt.Errorf("unsupported provider: %s", create.Provider)
	}
}

// updateConfigTable updates the global configuration table
func (s *DNSProviderService) updateConfigTable(providerType, providerName string) {
	s.db.Exec(`INSERT INTO configuration (key, value, updated_at) VALUES ($1, $2, CURRENT_TIMESTAMP) ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP`, "dns_provider_type", providerType)
	s.db.Exec(`INSERT INTO configuration (key, value, updated_at) VALUES ($1, $2, CURRENT_TIMESTAMP) ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = CURRENT_TIMESTAMP`, "dns_provider_name", providerName)
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
