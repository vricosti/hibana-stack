package models

import "time"

// EmailAccount represents an email account
type EmailAccount struct {
	ID           int       `json:"id"`
	DomainID     int       `json:"domain_id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name,omitempty"`
	PasswordHash string    `json:"-"` // Never expose password hash
	CreatedAt    time.Time `json:"created_at"`
}

// EmailAccountCreate represents the data needed to create a new email account
type EmailAccountCreate struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
	FullName string `json:"full_name,omitempty"`
}

// EmailAccountUpdate represents the data that can be updated for an email account
type EmailAccountUpdate struct {
	Password string `json:"password,omitempty" validate:"omitempty,min=8"`
	FullName string `json:"full_name,omitempty"`
}

// EmailAccountWithDomain includes domain information
type EmailAccountWithDomain struct {
	EmailAccount
	DomainName string `json:"domain_name"`
	Email      string `json:"email"` // username@domain
}
