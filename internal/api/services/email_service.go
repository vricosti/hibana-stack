package services

import (
	"database/sql"
	"fmt"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"golang.org/x/crypto/bcrypt"
)

// EmailService handles email account operations
type EmailService struct {
	db *sql.DB
}

// NewEmailService creates a new email service
func NewEmailService(db *sql.DB) *EmailService {
	return &EmailService{db: db}
}

// GetByDomain returns all email accounts for a domain
func (s *EmailService) GetByDomain(domainID int) ([]models.EmailAccount, error) {
	query := `SELECT id, domain_id, username, full_name, created_at
	          FROM email_accounts WHERE domain_id = $1 ORDER BY created_at DESC`

	rows, err := s.db.Query(query, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query email accounts: %w", err)
	}
	defer rows.Close()

	var accounts []models.EmailAccount
	for rows.Next() {
		var acc models.EmailAccount
		if err := rows.Scan(&acc.ID, &acc.DomainID, &acc.Username, &acc.FullName, &acc.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan email account: %w", err)
		}
		accounts = append(accounts, acc)
	}

	return accounts, nil
}

// GetByID returns an email account by ID
func (s *EmailService) GetByID(id int) (*models.EmailAccount, error) {
	query := `SELECT id, domain_id, username, full_name, created_at
	          FROM email_accounts WHERE id = $1`

	var acc models.EmailAccount
	err := s.db.QueryRow(query, id).Scan(&acc.ID, &acc.DomainID, &acc.Username, &acc.FullName, &acc.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("email account not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query email account: %w", err)
	}

	return &acc, nil
}

// GetAll returns all email accounts with domain information
func (s *EmailService) GetAll() ([]models.EmailAccountWithDomain, error) {
	query := `SELECT ea.id, ea.domain_id, ea.username, ea.full_name, ea.created_at, d.name
	          FROM email_accounts ea
	          JOIN domains d ON ea.domain_id = d.id
	          ORDER BY d.name, ea.username`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query email accounts: %w", err)
	}
	defer rows.Close()

	var accounts []models.EmailAccountWithDomain
	for rows.Next() {
		var acc models.EmailAccountWithDomain
		if err := rows.Scan(&acc.ID, &acc.DomainID, &acc.Username, &acc.FullName, &acc.CreatedAt, &acc.DomainName); err != nil {
			return nil, fmt.Errorf("failed to scan email account: %w", err)
		}
		acc.Email = fmt.Sprintf("%s@%s", acc.Username, acc.DomainName)
		accounts = append(accounts, acc)
	}

	return accounts, nil
}

// Create creates a new email account
func (s *EmailService) Create(domainID int, create *models.EmailAccountCreate) (*models.EmailAccount, error) {
	// Check if account already exists
	var exists int
	err := s.db.QueryRow("SELECT COUNT(*) FROM email_accounts WHERE domain_id = $1 AND username = $2",
		domainID, create.Username).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing account: %w", err)
	}
	if exists > 0 {
		return nil, fmt.Errorf("email account %s already exists for this domain", create.Username)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(create.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert account
	var accountID int
	query := `INSERT INTO email_accounts (domain_id, username, password_hash, full_name)
	          VALUES ($1, $2, $3, $4) RETURNING id`
	err = s.db.QueryRow(query, domainID, create.Username, string(hashedPassword), create.FullName).Scan(&accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert email account: %w", err)
	}

	return s.GetByID(accountID)
}

// Update updates an existing email account
func (s *EmailService) Update(id int, update *models.EmailAccountUpdate) (*models.EmailAccount, error) {
	// Check account exists
	account, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Build update query dynamically based on what's provided
	updates := []string{}
	args := []interface{}{}
	argPos := 1

	if update.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(update.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		updates = append(updates, fmt.Sprintf("password_hash = $%d", argPos))
		args = append(args, string(hashedPassword))
		argPos++
	}

	if update.FullName != "" {
		updates = append(updates, fmt.Sprintf("full_name = $%d", argPos))
		args = append(args, update.FullName)
		argPos++
	}

	if len(updates) == 0 {
		// Nothing to update
		return account, nil
	}

	// Add ID to args
	args = append(args, id)

	query := fmt.Sprintf("UPDATE email_accounts SET %s WHERE id = $%d",
		joinStrings(updates, ", "), argPos)

	_, err = s.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update email account: %w", err)
	}

	return s.GetByID(id)
}

// Delete deletes an email account
func (s *EmailService) Delete(id int) error {
	// Check account exists
	_, err := s.GetByID(id)
	if err != nil {
		return err
	}

	query := `DELETE FROM email_accounts WHERE id = $1`
	_, err = s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete email account: %w", err)
	}

	return nil
}

// CheckPassword verifies a password for an email account
func (s *EmailService) CheckPassword(id int, password string) (bool, error) {
	var passwordHash string
	query := `SELECT password_hash FROM email_accounts WHERE id = $1`
	err := s.db.QueryRow(query, id).Scan(&passwordHash)
	if err != nil {
		return false, fmt.Errorf("failed to get password hash: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	return err == nil, nil
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
