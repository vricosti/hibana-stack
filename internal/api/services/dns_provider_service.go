package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	Domain    string `json:"domain"`
	Status    string `json:"status,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
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

	// Filter out already configured domains
	var available []AvailableDomain
	for _, d := range hostingerDomains {
		if !configuredDomains[d.Domain] {
			available = append(available, AvailableDomain{
				Domain:    d.Domain,
				Status:    d.Status,
				ExpiresAt: d.ExpiresAt,
			})
		}
	}

	return available, nil
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
