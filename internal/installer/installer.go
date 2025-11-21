package installer

import (
	"database/sql"

	"github.com/vricosti/hibana-stack/internal/config"
	_ "github.com/lib/pq"
)

// Installer handles the installation and configuration of Hibana Stack
type Installer struct {
	config *config.Config
	db     *sql.DB
}

// New creates a new Installer instance
func New(cfg *config.Config) *Installer {
	return &Installer{
		config: cfg,
	}
}

// NewInstaller creates a new Installer instance with database connection
func NewInstaller(cfg *config.Config, db *sql.DB) *Installer {
	return &Installer{
		config: cfg,
		db:     db,
	}
}

// Close closes database connections
func (i *Installer) Close() error {
	if i.db != nil {
		return i.db.Close()
	}
	return nil
}

// SetDatabase sets the database connection (used for reset)
func (i *Installer) SetDatabase(db *sql.DB) {
	i.db = db
}

// RestoreFirewallState is a public wrapper for restoreFirewallState
func (i *Installer) RestoreFirewallState() error {
	return i.restoreFirewallState()
}
