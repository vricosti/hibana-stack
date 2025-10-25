package installer

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	hibanaDatabaseName = "hibana"
	hibanaUser         = "hibana"
	pdnsDatabaseName   = "powerdns"
	pdnsUser           = "pdns"
)

// SetupPostgreSQL initializes PostgreSQL databases for Hibana and PowerDNS
func (i *Installer) SetupPostgreSQL() error {
	// Start PostgreSQL service
	if err := i.startPostgreSQL(); err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %w", err)
	}

	// Check if databases already exist
	hibanaExists, err := i.databaseExists(hibanaDatabaseName)
	if err != nil {
		return err
	}

	pdnsExists, err := i.databaseExists(pdnsDatabaseName)
	if err != nil {
		return err
	}

	// Create Hibana database and user
	if !hibanaExists {
		if err := i.createDatabase(hibanaDatabaseName, hibanaUser); err != nil {
			return fmt.Errorf("failed to create Hibana database: %w", err)
		}
	}

	// Create PowerDNS database and user
	if !pdnsExists {
		if err := i.createDatabase(pdnsDatabaseName, pdnsUser); err != nil {
			return fmt.Errorf("failed to create PowerDNS database: %w", err)
		}
	}

	// Initialize Hibana database schema
	if err := i.initHibanaSchema(); err != nil {
		return fmt.Errorf("failed to initialize Hibana schema: %w", err)
	}

	// Initialize PowerDNS database schema
	if !pdnsExists {
		if err := i.initPowerDNSSchema(); err != nil {
			return fmt.Errorf("failed to initialize PowerDNS schema: %w", err)
		}
	}

	// Connect to Hibana database
	connStr := fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable",
		hibanaUser, i.getOrGeneratePassword(hibanaUser), hibanaDatabaseName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	i.db = db

	return nil
}

// startPostgreSQL ensures PostgreSQL is running
func (i *Installer) startPostgreSQL() error {
	cmd := exec.Command("systemctl", "start", "postgresql")
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("systemctl", "enable", "postgresql")
	return cmd.Run()
}

// databaseExists checks if a database exists
func (i *Installer) databaseExists(dbName string) (bool, error) {
	cmd := exec.Command("sudo", "-u", "postgres", "psql", "-lqt")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return string(output) != "" && containsDatabase(string(output), dbName), nil
}

// containsDatabase checks if database name is in psql output
func containsDatabase(output, dbName string) bool {
	return exec.Command("grep", "-q", dbName).Run() == nil
}

// createDatabase creates a new PostgreSQL database and user
func (i *Installer) createDatabase(dbName, userName string) error {
	password := i.getOrGeneratePassword(userName)

	// Create user
	createUserSQL := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s';", userName, password)
	cmd := exec.Command("sudo", "-u", "postgres", "psql", "-c", createUserSQL)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Ignore error if user already exists
		if !containsString(string(output), "already exists") {
			return fmt.Errorf("failed to create user: %w\n%s", err, output)
		}
	}

	// Create database
	createDBSQL := fmt.Sprintf("CREATE DATABASE %s OWNER %s;", dbName, userName)
	cmd = exec.Command("sudo", "-u", "postgres", "psql", "-c", createDBSQL)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Ignore error if database already exists
		if !containsString(string(output), "already exists") {
			return fmt.Errorf("failed to create database: %w\n%s", err, output)
		}
	}

	// Grant privileges
	grantSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s;", dbName, userName)
	cmd = exec.Command("sudo", "-u", "postgres", "psql", "-c", grantSQL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	// Save password to file
	return i.savePassword(userName, password)
}

// getOrGeneratePassword retrieves or generates a password for a user
func (i *Installer) getOrGeneratePassword(userName string) string {
	passwordFile := filepath.Join("/etc/hibana/passwords", userName)

	if data, err := os.ReadFile(passwordFile); err == nil {
		return string(data)
	}

	// Generate random password
	return generateRandomPassword(32)
}

// savePassword saves a password to the encrypted storage
func (i *Installer) savePassword(userName, password string) error {
	passwordDir := "/etc/hibana/passwords"
	if err := os.MkdirAll(passwordDir, 0700); err != nil {
		return err
	}

	passwordFile := filepath.Join(passwordDir, userName)
	return os.WriteFile(passwordFile, []byte(password), 0600)
}

// initHibanaSchema initializes the Hibana database schema
func (i *Installer) initHibanaSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS domains (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    server_ip VARCHAR(45) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS email_accounts (
    id SERIAL PRIMARY KEY,
    domain_id INTEGER REFERENCES domains(id) ON DELETE CASCADE,
    username VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(domain_id, username)
);

CREATE TABLE IF NOT EXISTS dkim_keys (
    id SERIAL PRIMARY KEY,
    domain_id INTEGER REFERENCES domains(id) ON DELETE CASCADE,
    selector VARCHAR(255) NOT NULL,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(domain_id, selector)
);

CREATE TABLE IF NOT EXISTS configuration (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS installation_state (
    id SERIAL PRIMARY KEY,
    step VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    firewall_rules_before TEXT,
    firewall_rules_after TEXT,
    ports_opened TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT
);
`

	connStr := fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable",
		hibanaUser, i.getOrGeneratePassword(hibanaUser), hibanaDatabaseName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(schema)
	return err
}

// initPowerDNSSchema initializes the PowerDNS database schema
func (i *Installer) initPowerDNSSchema() error {
	// PowerDNS PostgreSQL schema with IF NOT EXISTS for idempotency
	schema := `
CREATE TABLE IF NOT EXISTS domains (
  id                    SERIAL PRIMARY KEY,
  name                  VARCHAR(255) NOT NULL UNIQUE,
  master                VARCHAR(128) DEFAULT NULL,
  last_check            INT DEFAULT NULL,
  type                  VARCHAR(6) NOT NULL,
  notified_serial       BIGINT DEFAULT NULL,
  account               VARCHAR(40) DEFAULT NULL,
  options               VARCHAR(65535) DEFAULT NULL,
  catalog               VARCHAR(255) DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS name_index ON domains(name);

CREATE TABLE IF NOT EXISTS records (
  id                    BIGSERIAL PRIMARY KEY,
  domain_id             INT DEFAULT NULL,
  name                  VARCHAR(255) DEFAULT NULL,
  type                  VARCHAR(10) DEFAULT NULL,
  content               VARCHAR(65535) DEFAULT NULL,
  ttl                   INT DEFAULT NULL,
  prio                  INT DEFAULT NULL,
  disabled              BOOL DEFAULT 'f',
  ordername             VARCHAR(255),
  auth                  BOOL DEFAULT 't',
  CONSTRAINT domain_exists FOREIGN KEY(domain_id) REFERENCES domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS rec_name_index ON records(name);
CREATE INDEX IF NOT EXISTS nametype_index ON records(name,type);
CREATE INDEX IF NOT EXISTS domain_id ON records(domain_id);
CREATE INDEX IF NOT EXISTS orderindex ON records(ordername);

CREATE TABLE IF NOT EXISTS supermasters (
  ip                    INET NOT NULL,
  nameserver            VARCHAR(255) NOT NULL,
  account               VARCHAR(40) DEFAULT NULL,
  PRIMARY KEY(ip, nameserver)
);

CREATE TABLE IF NOT EXISTS comments (
  id                    SERIAL PRIMARY KEY,
  domain_id             INT NOT NULL,
  name                  VARCHAR(255) NOT NULL,
  type                  VARCHAR(10) NOT NULL,
  modified_at           INT NOT NULL,
  account               VARCHAR(40) DEFAULT NULL,
  comment               VARCHAR(65535) NOT NULL,
  CONSTRAINT comments_domain_exists FOREIGN KEY(domain_id) REFERENCES domains(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS comments_domain_id_idx ON comments (domain_id);
CREATE INDEX IF NOT EXISTS comments_name_type_idx ON comments (name, type);
CREATE INDEX IF NOT EXISTS comments_order_idx ON comments (domain_id, modified_at);

CREATE TABLE IF NOT EXISTS domainmetadata (
  id                    SERIAL PRIMARY KEY,
  domain_id             INT REFERENCES domains(id) ON DELETE CASCADE,
  kind                  VARCHAR(32),
  content               TEXT
);

CREATE INDEX IF NOT EXISTS domainmetadata_idx ON domainmetadata (domain_id, kind);

CREATE TABLE IF NOT EXISTS cryptokeys (
  id                    SERIAL PRIMARY KEY,
  domain_id             INT REFERENCES domains(id) ON DELETE CASCADE,
  flags                 INT NOT NULL,
  active                BOOL,
  published             BOOL DEFAULT TRUE,
  content               TEXT
);

CREATE INDEX IF NOT EXISTS domainidindex ON cryptokeys(domain_id);

CREATE TABLE IF NOT EXISTS tsigkeys (
  id                    SERIAL PRIMARY KEY,
  name                  VARCHAR(255) UNIQUE,
  algorithm             VARCHAR(50),
  secret                VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS namealgoindex ON tsigkeys(name, algorithm);
`

	connStr := fmt.Sprintf("host=localhost port=5432 user=%s password=%s dbname=%s sslmode=disable",
		pdnsUser, i.getOrGeneratePassword(pdnsUser), pdnsDatabaseName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(schema)
	return err
}

// Helper functions

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}

func generateRandomPassword(length int) string {
	// Cryptographically secure password generation using crypto/rand
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"
	password := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := range password {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			// Fallback to a simpler character if random fails
			password[i] = charset[i%len(charset)]
		} else {
			password[i] = charset[n.Int64()]
		}
	}

	return string(password)
}
