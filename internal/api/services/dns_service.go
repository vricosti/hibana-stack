package services

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/dnsprovider"
)

// DNSService handles DNS record operations
type DNSService struct {
	hibanaDB *sql.DB
	pdnsDB   *sql.DB
}

// NewDNSService creates a new DNS service
func NewDNSService(hibanaDB, pdnsDB *sql.DB) *DNSService {
	return &DNSService{
		hibanaDB: hibanaDB,
		pdnsDB:   pdnsDB,
	}
}

// GetByDomain returns all DNS records for a domain
func (s *DNSService) GetByDomain(domainID int) ([]models.DNSRecord, error) {
	// Get domain name from Hibana DB
	var domainName string
	err := s.hibanaDB.QueryRow("SELECT name FROM domains WHERE id = $1", domainID).Scan(&domainName)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	// Check if using external DNS provider
	providerType, providerName, apiToken := s.getDNSProviderConfig()
	if providerType == "external" {
		return s.getRecordsFromExternalProvider(domainName, providerName, apiToken)
	}

	// Get PowerDNS domain ID
	var pdnsID int
	err = s.pdnsDB.QueryRow("SELECT id FROM domains WHERE name = $1", domainName).Scan(&pdnsID)
	if err == sql.ErrNoRows {
		return []models.DNSRecord{}, nil // Domain exists but has no DNS records yet
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get PowerDNS domain: %w", err)
	}

	// Get DNS records
	query := `SELECT id, domain_id, name, type, content, ttl, prio
	          FROM records WHERE domain_id = $1 ORDER BY type, name`

	rows, err := s.pdnsDB.Query(query, pdnsID)
	if err != nil {
		return nil, fmt.Errorf("failed to query DNS records: %w", err)
	}
	defer rows.Close()

	var records []models.DNSRecord
	for rows.Next() {
		var rec models.DNSRecord
		var prio sql.NullInt64
		if err := rows.Scan(&rec.ID, &rec.DomainID, &rec.Name, &rec.Type, &rec.Content, &rec.TTL, &prio); err != nil {
			return nil, fmt.Errorf("failed to scan DNS record: %w", err)
		}
		if prio.Valid {
			rec.Priority = int(prio.Int64)
		}
		records = append(records, rec)
	}

	return records, nil
}

// getDNSProviderConfig retrieves the DNS provider configuration from the database
func (s *DNSService) getDNSProviderConfig() (providerType, providerName, apiToken string) {
	// Get DNS provider type and name from configuration
	err := s.hibanaDB.QueryRow(`SELECT value FROM configuration WHERE key = 'dns_provider_type'`).Scan(&providerType)
	if err != nil {
		return "", "", ""
	}

	err = s.hibanaDB.QueryRow(`SELECT value FROM configuration WHERE key = 'dns_provider_name'`).Scan(&providerName)
	if err != nil {
		return providerType, "", ""
	}

	// Get API token from dns_providers table
	_ = s.hibanaDB.QueryRow(`SELECT api_token FROM dns_providers WHERE provider = $1`, providerName).Scan(&apiToken)

	return providerType, providerName, apiToken
}

// getRecordsFromExternalProvider fetches DNS records from an external provider
func (s *DNSService) getRecordsFromExternalProvider(domainName, providerName, apiToken string) ([]models.DNSRecord, error) {
	switch providerName {
	case "hostinger":
		if apiToken == "" {
			return nil, fmt.Errorf("Hostinger API token not configured")
		}
		provider := dnsprovider.NewHostingerProvider(apiToken)
		hostingerRecords, err := provider.GetRecordsPublic(domainName)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch records from Hostinger: %w", err)
		}

		// Convert Hostinger records to our model
		var records []models.DNSRecord
		id := 1
		for _, hr := range hostingerRecords {
			rec := models.DNSRecord{
				ID:       id,
				DomainID: 0, // External records don't have a local domain ID
				Name:     hr.Name,
				Type:     hr.Type,
				Content:  hr.Content,
				TTL:      hr.TTL,
			}
			// Parse priority from MX records
			if hr.Type == "MX" {
				// MX content format: "10 mail.example.com"
				parts := strings.SplitN(hr.Content, " ", 2)
				if len(parts) == 2 {
					fmt.Sscanf(parts[0], "%d", &rec.Priority)
					rec.Content = parts[1]
				}
			}
			records = append(records, rec)
			id++
		}
		return records, nil

	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", providerName)
	}
}

// GetByID returns a DNS record by ID
func (s *DNSService) GetByID(id int) (*models.DNSRecord, error) {
	query := `SELECT id, domain_id, name, type, content, ttl, prio FROM records WHERE id = $1`

	var rec models.DNSRecord
	var prio sql.NullInt64
	err := s.pdnsDB.QueryRow(query, id).Scan(&rec.ID, &rec.DomainID, &rec.Name, &rec.Type, &rec.Content, &rec.TTL, &prio)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("DNS record not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query DNS record: %w", err)
	}

	if prio.Valid {
		rec.Priority = int(prio.Int64)
	}

	return &rec, nil
}

// Create creates a new DNS record
func (s *DNSService) Create(domainID int, create *models.DNSRecordCreate) (*models.DNSRecord, error) {
	// Get domain name from Hibana DB
	var domainName string
	err := s.hibanaDB.QueryRow("SELECT name FROM domains WHERE id = $1", domainID).Scan(&domainName)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	// Get or create PowerDNS domain
	var pdnsID int
	err = s.pdnsDB.QueryRow("SELECT id FROM domains WHERE name = $1", domainName).Scan(&pdnsID)
	if err == sql.ErrNoRows {
		// Create domain in PowerDNS if it doesn't exist
		err = s.pdnsDB.QueryRow("INSERT INTO domains (name, type) VALUES ($1, 'MASTER') RETURNING id",
			domainName).Scan(&pdnsID)
		if err != nil {
			return nil, fmt.Errorf("failed to create PowerDNS domain: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get PowerDNS domain: %w", err)
	}

	// Format record name
	recordName := s.formatRecordName(create.Name, domainName)

	// Format content for CNAME records
	content := create.Content
	if create.Type == "CNAME" && !strings.HasSuffix(content, ".") {
		content = content + "."
	}

	// Insert DNS record
	var recordID int
	query := `INSERT INTO records (domain_id, name, type, content, ttl, prio)
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	var prio interface{}
	if create.Priority > 0 {
		prio = create.Priority
	} else {
		prio = nil
	}

	err = s.pdnsDB.QueryRow(query, pdnsID, recordName, create.Type, content, create.TTL, prio).Scan(&recordID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert DNS record: %w", err)
	}

	return s.GetByID(recordID)
}

// Update updates an existing DNS record
func (s *DNSService) Update(id int, update *models.DNSRecordUpdate) (*models.DNSRecord, error) {
	// Check record exists
	record, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Build update query
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if update.Content != "" {
		content := update.Content
		if record.Type == "CNAME" && !strings.HasSuffix(content, ".") {
			content = content + "."
		}
		updates = append(updates, fmt.Sprintf("content = $%d", argPos))
		args = append(args, content)
		argPos++
	}

	if update.TTL > 0 {
		updates = append(updates, fmt.Sprintf("ttl = $%d", argPos))
		args = append(args, update.TTL)
		argPos++
	}

	if update.Priority > 0 {
		updates = append(updates, fmt.Sprintf("prio = $%d", argPos))
		args = append(args, update.Priority)
		argPos++
	}

	if len(updates) == 0 {
		return record, nil
	}

	// Add ID to args
	args = append(args, id)

	query := fmt.Sprintf("UPDATE records SET %s WHERE id = $%d", strings.Join(updates, ", "), argPos)

	_, err = s.pdnsDB.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update DNS record: %w", err)
	}

	return s.GetByID(id)
}

// Delete deletes a DNS record
func (s *DNSService) Delete(id int) error {
	// Check record exists
	_, err := s.GetByID(id)
	if err != nil {
		return err
	}

	query := `DELETE FROM records WHERE id = $1`
	_, err = s.pdnsDB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	return nil
}

// formatRecordName formats the record name for PowerDNS
func (s *DNSService) formatRecordName(name, domainName string) string {
	if name == "@" || name == "" {
		return domainName
	}

	// If name already contains the domain, return as is
	if strings.HasSuffix(name, "."+domainName) || name == domainName {
		return name
	}

	// Otherwise, append domain name
	return fmt.Sprintf("%s.%s", name, domainName)
}

// CreateDefaultRecords creates default DNS records for a new domain
func (s *DNSService) CreateDefaultRecords(domainID int, serverIP string) error {
	// Get domain name
	var domainName string
	err := s.hibanaDB.QueryRow("SELECT name FROM domains WHERE id = $1", domainID).Scan(&domainName)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	// Default records to create
	defaultRecords := []models.DNSRecordCreate{
		{Name: "@", Type: "A", Content: serverIP, TTL: 300},
		{Name: "www", Type: "A", Content: serverIP, TTL: 300},
		{Name: "mail", Type: "A", Content: serverIP, TTL: 3600},
		{Name: "@", Type: "MX", Content: fmt.Sprintf("mail.%s", domainName), TTL: 14400, Priority: 10},
		{Name: "@", Type: "TXT", Content: fmt.Sprintf(`"v=spf1 ip4:%s -all"`, serverIP), TTL: 14400},
	}

	// Create each record
	for _, record := range defaultRecords {
		_, err := s.Create(domainID, &record)
		if err != nil {
			return fmt.Errorf("failed to create default record %s: %w", record.Type, err)
		}
	}

	return nil
}
