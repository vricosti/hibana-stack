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

// EmailAlias represents an email redirect/alias
type EmailAlias struct {
	ID            int       `json:"id"`
	DomainID      int       `json:"domain_id"`
	SourceAddress string    `json:"source_address"`
	Destination   string    `json:"destination"`
	CreatedAt     time.Time `json:"created_at"`
}

// EmailAliasCreate represents the data needed to create a new email alias
type EmailAliasCreate struct {
	SourceAddress string `json:"source_address" validate:"required"`
	Destination   string `json:"destination" validate:"required,email"`
}

// EmailAliasUpdate represents the data that can be updated for an email alias
type EmailAliasUpdate struct {
	Destination string `json:"destination" validate:"required,email"`
}

// EmailAliasWithDomain includes domain information
type EmailAliasWithDomain struct {
	EmailAlias
	DomainName  string `json:"domain_name"`
	FullAddress string `json:"full_address"` // source@domain
}
