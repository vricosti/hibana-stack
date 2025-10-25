package installer

import (
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// saveFirewallStateBefore captures the current firewall state before making changes
func (i *Installer) saveFirewallStateBefore(step string) error {
	if i.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Get current firewall state
	firewallState, err := getFirewallState()
	if err != nil {
		// If UFW is not installed or not active, save empty state
		firewallState = "UFW not installed or not active"
	}

	// Insert initial state record
	_, err = i.db.Exec(`
		INSERT INTO installation_state (step, status, firewall_rules_before, started_at)
		VALUES ($1, $2, $3, $4)
	`, step, "started", firewallState, time.Now())

	return err
}

// saveFirewallStateAfter updates the installation state with firewall rules after changes
func (i *Installer) saveFirewallStateAfter(step string, portsOpened []string) error {
	if i.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Get current firewall state after changes
	firewallState, err := getFirewallState()
	if err != nil {
		firewallState = "UFW not installed or not active"
	}

	// Update the most recent record for this step
	_, err = i.db.Exec(`
		UPDATE installation_state
		SET firewall_rules_after = $1,
		    ports_opened = $2,
		    status = $3,
		    completed_at = $4
		WHERE step = $5 AND completed_at IS NULL
	`, firewallState, strings.Join(portsOpened, ","), "completed", time.Now(), step)

	return err
}

// recordInstallationError records an error during installation
func (i *Installer) recordInstallationError(step string, errorMsg string) error {
	if i.db == nil {
		return nil // Don't fail if we can't record the error
	}

	_, err := i.db.Exec(`
		UPDATE installation_state
		SET status = $1,
		    error_message = $2,
		    completed_at = $3
		WHERE step = $4 AND completed_at IS NULL
	`, "failed", errorMsg, time.Now(), step)

	return err
}

// getFirewallState retrieves the current UFW firewall state
func getFirewallState() (string, error) {
	cmd := exec.Command("ufw", "status", "numbered")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// restoreFirewallState restores the firewall to its initial state
func (i *Installer) restoreFirewallState() error {
	if i.db == nil {
		fmt.Println("  ⚠️  Database not available, cannot restore firewall state")
		return nil
	}

	// Get all ports that were opened during installation
	rows, err := i.db.Query(`
		SELECT DISTINCT ports_opened
		FROM installation_state
		WHERE ports_opened IS NOT NULL AND ports_opened != ''
		ORDER BY id
	`)
	if err != nil {
		return fmt.Errorf("failed to query installation state: %w", err)
	}
	defer rows.Close()

	var allPortsOpened []string
	for rows.Next() {
		var portsStr string
		if err := rows.Scan(&portsStr); err != nil {
			continue
		}
		if portsStr != "" {
			ports := strings.Split(portsStr, ",")
			allPortsOpened = append(allPortsOpened, ports...)
		}
	}

	// Check if UFW is active
	cmd := exec.Command("ufw", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// UFW not installed or not available
		fmt.Println("  ℹ️  UFW not available, skipping firewall restoration")
		return nil
	}

	if !strings.Contains(string(output), "Status: active") {
		fmt.Println("  ℹ️  UFW is not active, skipping firewall restoration")
		return nil
	}

	// Remove all ports that were opened during installation
	if len(allPortsOpened) > 0 {
		fmt.Printf("  Removing firewall rules that were added during installation...\n")
		for _, port := range allPortsOpened {
			port = strings.TrimSpace(port)
			if port == "" {
				continue
			}
			// Delete the rule
			cmd := exec.Command("ufw", "delete", "allow", port)
			if err := cmd.Run(); err != nil {
				fmt.Printf("  ⚠️  Failed to remove firewall rule for port %s: %v\n", port, err)
			} else {
				fmt.Printf("  ✓ Removed firewall rule for port %s\n", port)
			}
		}
	}

	return nil
}

// openPortsWithTracking opens firewall ports and records them in the database
func (i *Installer) openPortsWithTracking(step string, ports []string, portDescriptions map[string]string) error {
	// Save firewall state before changes
	if err := i.saveFirewallStateBefore(step); err != nil {
		fmt.Printf("  ⚠️  Warning: failed to save firewall state: %v\n", err)
	}

	// Check if ufw is installed and active
	cmd := exec.Command("ufw", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// ufw might not be installed or not enabled, that's okay
		fmt.Println("  ℹ️  UFW not installed or not active")
		return nil
	}

	// Check if ufw is active
	if !strings.Contains(string(output), "Status: active") {
		// Firewall is not active, no need to open ports
		fmt.Println("  ℹ️  UFW firewall is not active")
		return nil
	}

	// Open ports
	var openedPorts []string
	for _, port := range ports {
		cmd := exec.Command("ufw", "allow", port)
		if err := cmd.Run(); err != nil {
			// Record error
			i.recordInstallationError(step, fmt.Sprintf("failed to open port %s: %v", port, err))
			return fmt.Errorf("failed to open port %s: %w", port, err)
		}
		openedPorts = append(openedPorts, port)
	}

	// Save firewall state after changes
	if err := i.saveFirewallStateAfter(step, openedPorts); err != nil {
		fmt.Printf("  ⚠️  Warning: failed to save firewall state after changes: %v\n", err)
	}

	// Display success message
	portList := strings.Join(ports, ", ")
	fmt.Printf("  ✓ Firewall ports opened: %s\n", portList)

	return nil
}

// getInstallationHistory retrieves the installation history from the database
func (i *Installer) getInstallationHistory() ([]InstallationStateRecord, error) {
	if i.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	rows, err := i.db.Query(`
		SELECT id, step, status, firewall_rules_before, firewall_rules_after,
		       ports_opened, started_at, completed_at, error_message
		FROM installation_state
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []InstallationStateRecord
	for rows.Next() {
		var r InstallationStateRecord
		var completedAt sql.NullTime
		var errorMsg sql.NullString
		var firewallBefore sql.NullString
		var firewallAfter sql.NullString
		var portsOpened sql.NullString

		err := rows.Scan(&r.ID, &r.Step, &r.Status, &firewallBefore, &firewallAfter,
			&portsOpened, &r.StartedAt, &completedAt, &errorMsg)
		if err != nil {
			continue
		}

		if completedAt.Valid {
			r.CompletedAt = &completedAt.Time
		}
		if errorMsg.Valid {
			r.ErrorMessage = errorMsg.String
		}
		if firewallBefore.Valid {
			r.FirewallBefore = firewallBefore.String
		}
		if firewallAfter.Valid {
			r.FirewallAfter = firewallAfter.String
		}
		if portsOpened.Valid {
			r.PortsOpened = portsOpened.String
		}

		records = append(records, r)
	}

	return records, nil
}

// InstallationStateRecord represents a record from the installation_state table
type InstallationStateRecord struct {
	ID              int
	Step            string
	Status          string
	FirewallBefore  string
	FirewallAfter   string
	PortsOpened     string
	StartedAt       time.Time
	CompletedAt     *time.Time
	ErrorMessage    string
}
