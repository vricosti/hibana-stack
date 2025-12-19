package models

import "time"

// SSHKey represents an SSH public key for a domain user
type SSHKey struct {
	ID          int       `json:"id"`
	DomainID    int       `json:"domain_id"`
	DomainName  string    `json:"domain_name"`
	Username    string    `json:"username"` // Domain username
	PublicKey   string    `json:"public_key"`
	Fingerprint string    `json:"fingerprint"` // SSH key fingerprint for display
	Label       string    `json:"label"`       // User-provided label (e.g., "John's laptop")
	CreatedAt   time.Time `json:"created_at"`
}

// SSHKeyCreateRequest represents the request to add a new SSH key
type SSHKeyCreateRequest struct {
	PublicKey string `json:"public_key" binding:"required"`
	Label     string `json:"label"`
}

// SSHKeyListResponse represents the response for listing SSH keys
type SSHKeyListResponse struct {
	Keys []SSHKey `json:"keys"`
}

// ExternalSSHKeyRequest represents the request to add an external service SSH key
type ExternalSSHKeyRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
	Label      string `json:"label" binding:"required"`
	Host       string `json:"host" binding:"required"`
	Hostname   string `json:"hostname"`
	User       string `json:"user" binding:"required"`
}
