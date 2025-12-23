package services

import (
	"database/sql"
	"fmt"

	"github.com/vricosti/hibana-stack/internal/api/models"
)

// AliasService handles email alias operations
type AliasService struct {
	db            *sql.DB
	domainService *DomainService
}

// NewAliasService creates a new alias service
func NewAliasService(db *sql.DB, domainService *DomainService) *AliasService {
	return &AliasService{db: db, domainService: domainService}
}

// GetByDomain returns all email aliases for a domain
func (s *AliasService) GetByDomain(domainID int) ([]models.EmailAlias, error) {
	query := `SELECT id, domain_id, source_address, destination_addresses, created_at
	          FROM email_aliases WHERE domain_id = $1 ORDER BY created_at DESC`

	rows, err := s.db.Query(query, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query email aliases: %w", err)
	}
	defer rows.Close()

	// Initialize as empty slice instead of nil to ensure JSON marshaling returns []
	aliases := make([]models.EmailAlias, 0)
	for rows.Next() {
		var alias models.EmailAlias
		if err := rows.Scan(&alias.ID, &alias.DomainID, &alias.SourceAddress, &alias.DestinationAddresses, &alias.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan email alias: %w", err)
		}
		aliases = append(aliases, alias)
	}

	return aliases, nil
}

// GetByID returns an email alias by ID
func (s *AliasService) GetByID(id int) (*models.EmailAlias, error) {
	query := `SELECT id, domain_id, source_address, destination_addresses, created_at
	          FROM email_aliases WHERE id = $1`

	var alias models.EmailAlias
	err := s.db.QueryRow(query, id).Scan(&alias.ID, &alias.DomainID, &alias.SourceAddress, &alias.DestinationAddresses, &alias.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email alias not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query email alias: %w", err)
	}

	return &alias, nil
}

// GetAll returns all email aliases with domain information
func (s *AliasService) GetAll() ([]models.EmailAliasWithDomain, error) {
	query := `SELECT ea.id, ea.domain_id, ea.source_address, ea.destination_addresses, ea.created_at, d.name
	          FROM email_aliases ea
	          JOIN domains d ON ea.domain_id = d.id
	          ORDER BY d.name, ea.source_address`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query email aliases: %w", err)
	}
	defer rows.Close()

	// Initialize as empty slice instead of nil to ensure JSON marshaling returns []
	aliases := make([]models.EmailAliasWithDomain, 0)
	for rows.Next() {
		var alias models.EmailAliasWithDomain
		if err := rows.Scan(&alias.ID, &alias.DomainID, &alias.SourceAddress, &alias.DestinationAddresses, &alias.CreatedAt, &alias.DomainName); err != nil {
			return nil, fmt.Errorf("failed to scan email alias: %w", err)
		}
		alias.FullAddress = fmt.Sprintf("%s@%s", alias.SourceAddress, alias.DomainName)
		aliases = append(aliases, alias)
	}

	return aliases, nil
}

// Create creates a new email alias
// Postfix reads aliases directly from the database, no file updates needed
func (s *AliasService) Create(domainID int, create *models.EmailAliasCreate) (*models.EmailAlias, error) {
	// Check if alias already exists
	var exists int
	err := s.db.QueryRow("SELECT COUNT(*) FROM email_aliases WHERE domain_id = $1 AND source_address = $2",
		domainID, create.SourceAddress).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing alias: %w", err)
	}
	if exists > 0 {
		return nil, fmt.Errorf("email alias %s already exists for this domain", create.SourceAddress)
	}

	// Insert alias
	var aliasID int
	query := `INSERT INTO email_aliases (domain_id, source_address, destination_addresses)
	          VALUES ($1, $2, $3) RETURNING id`
	err = s.db.QueryRow(query, domainID, create.SourceAddress, create.DestinationAddresses).Scan(&aliasID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert email alias: %w", err)
	}

	return s.GetByID(aliasID)
}

// Update updates an existing email alias
func (s *AliasService) Update(id int, update *models.EmailAliasUpdate) (*models.EmailAlias, error) {
	// Check alias exists
	_, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	query := `UPDATE email_aliases SET destination_addresses = $1 WHERE id = $2`
	_, err = s.db.Exec(query, update.DestinationAddresses, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update email alias: %w", err)
	}

	return s.GetByID(id)
}

// Delete deletes an email alias
func (s *AliasService) Delete(id int) error {
	// Check alias exists
	_, err := s.GetByID(id)
	if err != nil {
		return err
	}

	query := `DELETE FROM email_aliases WHERE id = $1`
	_, err = s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete email alias: %w", err)
	}

	return nil
}
