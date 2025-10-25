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

// Close closes database connections
func (i *Installer) Close() error {
	if i.db != nil {
		return i.db.Close()
	}
	return nil
}
