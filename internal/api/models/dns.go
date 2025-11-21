package models

// DNSRecord represents a DNS record
type DNSRecord struct {
	ID       int    `json:"id"`
	DomainID int    `json:"domain_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

// DNSRecordCreate represents the data needed to create a new DNS record
type DNSRecordCreate struct {
	Name     string `json:"name" validate:"required"`
	Type     string `json:"type" validate:"required,oneof=A AAAA CNAME MX TXT NS SOA SRV CAA"`
	Content  string `json:"content" validate:"required"`
	TTL      int    `json:"ttl" validate:"required,min=60"`
	Priority int    `json:"priority,omitempty"`
}

// DNSRecordUpdate represents the data that can be updated for a DNS record
type DNSRecordUpdate struct {
	Content  string `json:"content,omitempty"`
	TTL      int    `json:"ttl,omitempty" validate:"omitempty,min=60"`
	Priority int    `json:"priority,omitempty"`
}
