package dnsprovider

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// DNSBackupRecord represents a backed up DNS record
type DNSBackupRecord struct {
	ID            int64
	DomainName    string
	Provider      string
	RecordType    string
	RecordName    string
	RecordContent string
	RecordTTL     int
	RecordPriority int
	BackupReason  string
	CreatedAt     time.Time
}

// DNSBackupService handles DNS record backup operations
type DNSBackupService struct {
	db *sql.DB
}

// NewDNSBackupService creates a new DNS backup service
func NewDNSBackupService(dbHost, dbPort, dbUser, dbPassword, dbName string) (*DNSBackupService, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DNSBackupService{db: db}, nil
}

// Close closes the database connection
func (s *DNSBackupService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// BackupRecord saves a single DNS record to the backup table
func (s *DNSBackupService) BackupRecord(record DNSBackupRecord) error {
	query := `
		INSERT INTO dns_backup (domain_name, provider, record_type, record_name, record_content, record_ttl, record_priority, backup_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.db.Exec(query,
		record.DomainName,
		record.Provider,
		record.RecordType,
		record.RecordName,
		record.RecordContent,
		record.RecordTTL,
		record.RecordPriority,
		record.BackupReason,
	)

	if err != nil {
		return fmt.Errorf("failed to backup DNS record: %w", err)
	}

	return nil
}

// BackupRecords saves multiple DNS records to the backup table
func (s *DNSBackupService) BackupRecords(records []DNSBackupRecord) error {
	for _, record := range records {
		if err := s.BackupRecord(record); err != nil {
			return err
		}
	}
	return nil
}

// GetBackups retrieves all backup records for a domain
func (s *DNSBackupService) GetBackups(domainName string) ([]DNSBackupRecord, error) {
	query := `
		SELECT id, domain_name, provider, record_type, record_name, record_content, record_ttl, record_priority, backup_reason, created_at
		FROM dns_backup
		WHERE domain_name = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, domainName)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS backups: %w", err)
	}
	defer rows.Close()

	var records []DNSBackupRecord
	for rows.Next() {
		var r DNSBackupRecord
		var priority sql.NullInt64
		var ttl sql.NullInt64

		err := rows.Scan(&r.ID, &r.DomainName, &r.Provider, &r.RecordType, &r.RecordName,
			&r.RecordContent, &ttl, &priority, &r.BackupReason, &r.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan DNS backup record: %w", err)
		}

		if priority.Valid {
			r.RecordPriority = int(priority.Int64)
		}
		if ttl.Valid {
			r.RecordTTL = int(ttl.Int64)
		}

		records = append(records, r)
	}

	return records, nil
}

// BackupOVHRecords backs up OVH DNS records before modification
func BackupOVHRecords(provider *OVHCloudProvider, domain string, dbHost, dbPort, dbUser, dbPassword, dbName string) error {
	// Create backup service
	backupService, err := NewDNSBackupService(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		return fmt.Errorf("failed to create backup service: %w", err)
	}
	defer backupService.Close()

	// Get all records from OVH
	records, err := provider.GetRecords(domain)
	if err != nil {
		return fmt.Errorf("failed to get OVH records: %w", err)
	}

	fmt.Printf("  Backing up %d DNS records for %s...\n", len(records), domain)

	// Convert and backup each record
	for _, r := range records {
		backupRecord := DNSBackupRecord{
			DomainName:    domain,
			Provider:      "ovh",
			RecordType:    r.FieldType,
			RecordName:    r.SubDomain,
			RecordContent: r.Target,
			RecordTTL:     r.TTL,
			BackupReason:  "pre-installation backup",
		}

		// Parse priority from MX records
		if r.FieldType == "MX" && strings.Contains(r.Target, " ") {
			parts := strings.SplitN(r.Target, " ", 2)
			if len(parts) == 2 {
				var prio int
				fmt.Sscanf(parts[0], "%d", &prio)
				backupRecord.RecordPriority = prio
			}
		}

		if err := backupService.BackupRecord(backupRecord); err != nil {
			fmt.Printf("    Warning: Failed to backup record %s %s: %v\n", r.SubDomain, r.FieldType, err)
		}
	}

	fmt.Printf("  %d DNS records backed up successfully\n", len(records))
	return nil
}

// BackupHostingerRecords backs up Hostinger DNS records before modification
func BackupHostingerRecords(provider *HostingerProvider, domain string, dbHost, dbPort, dbUser, dbPassword, dbName string) error {
	// Create backup service
	backupService, err := NewDNSBackupService(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		return fmt.Errorf("failed to create backup service: %w", err)
	}
	defer backupService.Close()

	// Get all records from Hostinger
	records, err := provider.GetRecordsPublic(domain)
	if err != nil {
		return fmt.Errorf("failed to get Hostinger records: %w", err)
	}

	fmt.Printf("  Backing up %d DNS records for %s...\n", len(records), domain)

	// Convert and backup each record
	for _, r := range records {
		backupRecord := DNSBackupRecord{
			DomainName:    domain,
			Provider:      "hostinger",
			RecordType:    r.Type,
			RecordName:    r.Name,
			RecordContent: r.Content,
			RecordTTL:     r.TTL,
			BackupReason:  "pre-installation backup",
		}

		// Parse priority from MX records
		if r.Type == "MX" && strings.Contains(r.Content, " ") {
			parts := strings.SplitN(r.Content, " ", 2)
			if len(parts) == 2 {
				var prio int
				fmt.Sscanf(parts[0], "%d", &prio)
				backupRecord.RecordPriority = prio
			}
		}

		if err := backupService.BackupRecord(backupRecord); err != nil {
			fmt.Printf("    Warning: Failed to backup record %s %s: %v\n", r.Name, r.Type, err)
		}
	}

	fmt.Printf("  %d DNS records backed up successfully\n", len(records))
	return nil
}
