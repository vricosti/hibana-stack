package models

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
	Total   int         `json:"total"`
}

// StatsResponse represents system statistics
type StatsResponse struct {
	TotalDomains       int              `json:"total_domains"`
	TotalEmailAccounts int              `json:"total_email_accounts"`
	TotalDNSRecords    int              `json:"total_dns_records"`
	DomainStats        []DomainStat     `json:"domain_stats,omitempty"`
}

// DomainStat represents statistics for a single domain
type DomainStat struct {
	DomainName     string `json:"domain_name"`
	EmailCount     int    `json:"email_count"`
	DNSRecordCount int    `json:"dns_record_count"`
}
