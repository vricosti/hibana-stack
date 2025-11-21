package services

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/installer"
)

// DomainService handles domain-related operations
type DomainService struct {
	db        *sql.DB
	pdnsDB    *sql.DB
	installer *installer.Installer
}

// NewDomainService creates a new domain service
func NewDomainService(db, pdnsDB *sql.DB, inst *installer.Installer) *DomainService {
	return &DomainService{
		db:        db,
		pdnsDB:    pdnsDB,
		installer: inst,
	}
}

// GetAll returns all domains
func (s *DomainService) GetAll() ([]models.Domain, error) {
	query := `
		SELECT d.id, d.name, d.server_ip, d.created_at, d.updated_at, du.username
		FROM domains d
		LEFT JOIN domain_users du ON d.id = du.domain_id
		ORDER BY d.created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query domains: %w", err)
	}
	defer rows.Close()

	var domains []models.Domain
	for rows.Next() {
		var d models.Domain
		var username sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &d.ServerIP, &d.CreatedAt, &d.UpdatedAt, &username); err != nil {
			return nil, fmt.Errorf("failed to scan domain: %w", err)
		}
		if username.Valid {
			d.Username = username.String
		}
		domains = append(domains, d)
	}

	return domains, nil
}

// GetByID returns a domain by ID
func (s *DomainService) GetByID(id int) (*models.Domain, error) {
	query := `SELECT id, name, server_ip, created_at, updated_at FROM domains WHERE id = $1`

	var d models.Domain
	err := s.db.QueryRow(query, id).Scan(&d.ID, &d.Name, &d.ServerIP, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("domain not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query domain: %w", err)
	}

	return &d, nil
}

// GetByName returns a domain by name
func (s *DomainService) GetByName(name string) (*models.Domain, error) {
	query := `SELECT id, name, server_ip, created_at, updated_at FROM domains WHERE name = $1`

	var d models.Domain
	err := s.db.QueryRow(query, name).Scan(&d.ID, &d.Name, &d.ServerIP, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("domain not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query domain: %w", err)
	}

	return &d, nil
}

// Create creates a new domain
func (s *DomainService) Create(create *models.DomainCreate) (*models.Domain, error) {
	// Validate domain doesn't already exist
	existing, _ := s.GetByName(create.Name)
	if existing != nil {
		return nil, fmt.Errorf("domain %s already exists", create.Name)
	}

	// Use server IP from request or default
	serverIP := create.ServerIP
	if serverIP == "" {
		// Get primary domain's IP as default
		var primaryIP string
		err := s.db.QueryRow("SELECT server_ip FROM domains ORDER BY created_at ASC LIMIT 1").Scan(&primaryIP)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get default server IP: %w", err)
		}
		serverIP = primaryIP
	}

	// Insert domain into Hibana database
	var domainID int
	query := `INSERT INTO domains (name, server_ip) VALUES ($1, $2) RETURNING id`
	err := s.db.QueryRow(query, create.Name, serverIP).Scan(&domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert domain: %w", err)
	}

	// Create domain in PowerDNS
	if err := s.createDomainInPowerDNS(create.Name); err != nil {
		// Rollback Hibana insert
		s.db.Exec("DELETE FROM domains WHERE id = $1", domainID)
		return nil, fmt.Errorf("failed to create domain in PowerDNS: %w", err)
	}

	// Create domain user if requested
	if create.CreateUser && s.installer != nil {
		// Ensure hibana-domains group exists
		if err := s.installer.EnsureHibanaDomainGroup(); err != nil {
			return nil, fmt.Errorf("failed to ensure hibana-domains group: %w", err)
		}

		// Ensure master encryption key exists
		if err := s.installer.EnsureMasterKey(); err != nil {
			return nil, fmt.Errorf("failed to ensure master key: %w", err)
		}

		// Create domain user
		result, err := s.installer.CreateDomainUser(create.Name, create.SSHPublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create domain user: %w", err)
		}

		// Store domain user in database
		if err := s.installer.StoreDomainUser(create.Name, result); err != nil {
			return nil, fmt.Errorf("failed to store domain user: %w", err)
		}
	}

	// Return created domain
	return s.GetByID(domainID)
}

// Update updates an existing domain
func (s *DomainService) Update(id int, update *models.DomainUpdate) (*models.Domain, error) {
	// Check domain exists
	domain, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update server IP if provided
	if update.ServerIP != "" {
		query := `UPDATE domains SET server_ip = $1, updated_at = NOW() WHERE id = $2`
		_, err := s.db.Exec(query, update.ServerIP, id)
		if err != nil {
			return nil, fmt.Errorf("failed to update domain: %w", err)
		}
	}

	return s.GetByID(domain.ID)
}

// Delete deletes a domain
func (s *DomainService) Delete(id int) error {
	// Check domain exists
	domain, err := s.GetByID(id)
	if err != nil {
		return err
	}

	// Delete from PowerDNS first
	if err := s.deleteDomainFromPowerDNS(domain.Name); err != nil {
		return fmt.Errorf("failed to delete domain from PowerDNS: %w", err)
	}

	// Delete from Hibana database (cascade will delete related records)
	query := `DELETE FROM domains WHERE id = $1`
	_, err = s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	return nil
}

// createDomainInPowerDNS creates a domain in PowerDNS database
func (s *DomainService) createDomainInPowerDNS(domainName string) error {
	// Check if domain already exists in PowerDNS
	var existingID int
	err := s.pdnsDB.QueryRow("SELECT id FROM domains WHERE name = $1", domainName).Scan(&existingID)
	if err == nil {
		return fmt.Errorf("domain already exists in PowerDNS")
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing domain: %w", err)
	}

	// Create domain with MASTER type
	query := `INSERT INTO domains (name, type) VALUES ($1, 'MASTER')`
	_, err = s.pdnsDB.Exec(query, domainName)
	if err != nil {
		return fmt.Errorf("failed to insert domain: %w", err)
	}

	return nil
}

// deleteDomainFromPowerDNS deletes a domain from PowerDNS database
func (s *DomainService) deleteDomainFromPowerDNS(domainName string) error {
	// Get PowerDNS domain ID
	var pdnsID int
	err := s.pdnsDB.QueryRow("SELECT id FROM domains WHERE name = $1", domainName).Scan(&pdnsID)
	if err == sql.ErrNoRows {
		// Domain doesn't exist in PowerDNS, that's okay
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get PowerDNS domain ID: %w", err)
	}

	// Delete records first (cascade should handle this, but be explicit)
	_, err = s.pdnsDB.Exec("DELETE FROM records WHERE domain_id = $1", pdnsID)
	if err != nil {
		return fmt.Errorf("failed to delete DNS records: %w", err)
	}

	// Delete domain
	_, err = s.pdnsDB.Exec("DELETE FROM domains WHERE id = $1", pdnsID)
	if err != nil {
		return fmt.Errorf("failed to delete PowerDNS domain: %w", err)
	}

	return nil
}

// GetDomainUser returns the domain user for a domain
func (s *DomainService) GetDomainUser(domainID int) (*models.DomainUser, error) {
	query := `SELECT id, domain_id, username, ssh_public_key, ssh_private_key_encrypted, created_at, updated_at
	          FROM domain_users WHERE domain_id = $1`

	var du models.DomainUser
	err := s.db.QueryRow(query, domainID).Scan(
		&du.ID, &du.DomainID, &du.Username, &du.SSHPublicKey,
		&du.SSHPrivateKeyEncrypted, &du.CreatedAt, &du.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("domain user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query domain user: %w", err)
	}

	return &du, nil
}

// GetStats returns statistics about domains
func (s *DomainService) GetStats() (*models.StatsResponse, error) {
	stats := &models.StatsResponse{}

	// Total domains
	err := s.db.QueryRow("SELECT COUNT(*) FROM domains").Scan(&stats.TotalDomains)
	if err != nil {
		return nil, fmt.Errorf("failed to count domains: %w", err)
	}

	// Total email accounts
	err = s.db.QueryRow("SELECT COUNT(*) FROM email_accounts").Scan(&stats.TotalEmailAccounts)
	if err != nil {
		return nil, fmt.Errorf("failed to count email accounts: %w", err)
	}

	// Total DNS records (from PowerDNS)
	err = s.pdnsDB.QueryRow("SELECT COUNT(*) FROM records").Scan(&stats.TotalDNSRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to count DNS records: %w", err)
	}

	// Per-domain stats
	query := `
		SELECT d.name,
		       COUNT(DISTINCT ea.id) as email_count
		FROM domains d
		LEFT JOIN email_accounts ea ON d.id = ea.domain_id
		GROUP BY d.id, d.name
		ORDER BY d.created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query domain stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ds models.DomainStat
		if err := rows.Scan(&ds.DomainName, &ds.EmailCount); err != nil {
			return nil, fmt.Errorf("failed to scan domain stat: %w", err)
		}

		// Get DNS record count from PowerDNS
		var pdnsID int
		err := s.pdnsDB.QueryRow("SELECT id FROM domains WHERE name = $1", ds.DomainName).Scan(&pdnsID)
		if err == nil {
			s.pdnsDB.QueryRow("SELECT COUNT(*) FROM records WHERE domain_id = $1", pdnsID).Scan(&ds.DNSRecordCount)
		}

		stats.DomainStats = append(stats.DomainStats, ds)
	}

	return stats, nil
}

// sanitizeDomainName removes invalid characters from domain name
func sanitizeDomainName(domain string) string {
	domain = strings.ToLower(domain)
	domain = strings.TrimSpace(domain)
	return domain
}
