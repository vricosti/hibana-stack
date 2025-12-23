package models

import "time"

// Domain represents a domain in the system
type Domain struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`         // Internal name (always stored as entered, may be Unicode or Punycode)
	DisplayName string    `json:"display_name"` // User-friendly display name (Unicode + Punycode for IDN)
	ServerIP    string    `json:"server_ip"`
	Username    string    `json:"username,omitempty"` // Domain user username (from domain_users table)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DomainCreate represents the data needed to create a new domain
type DomainCreate struct {
	Name          string  `json:"name" validate:"required,fqdn"`
	ServerIP      string  `json:"server_ip,omitempty"`
	CreateUser    bool    `json:"create_user"`
	SSHKeyMode    string  `json:"ssh_key_mode,omitempty"`    // "auto" or "manual"
	SSHPublicKey  string  `json:"ssh_public_key,omitempty"`
}

// DomainUpdate represents the data that can be updated for a domain
type DomainUpdate struct {
	ServerIP string `json:"server_ip,omitempty"`
}

// DomainUser represents a domain user
type DomainUser struct {
	ID                      int       `json:"id"`
	DomainID                int       `json:"domain_id"`
	Username                string    `json:"username"`
	SSHPublicKey            string    `json:"ssh_public_key"`
	SSHPrivateKeyEncrypted  string    `json:"-"` // Never expose encrypted key
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}
