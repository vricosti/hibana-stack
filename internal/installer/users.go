package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vricosti/hibana-stack/internal/config"
)

// SetupSystemUsers creates system users if they don't exist
func (i *Installer) SetupSystemUsers() error {
	if len(i.config.SystemUsers) == 0 {
		return nil // No users to create
	}

	for _, user := range i.config.SystemUsers {
		if err := i.createSystemUser(user); err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Username, err)
		}
	}

	return nil
}

// createSystemUser creates a single system user with the specified configuration
func (i *Installer) createSystemUser(user config.SystemUser) error {
	// Check if user already exists
	if _, err := exec.Command("id", user.Username).Output(); err == nil {
		fmt.Printf("  • User %s already exists, updating configuration...\n", user.Username)
		return i.updateSystemUser(user)
	}

	fmt.Printf("  • Creating user: %s\n", user.Username)

	// Create user with home directory
	createCmd := exec.Command("useradd",
		"-m",                    // Create home directory
		"-s", "/bin/bash",       // Set shell
		"-c", user.Name,         // Set full name/comment
		user.Username,
	)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create user: %w, output: %s", err, string(output))
	}

	// Set password
	if err := i.setUserPassword(user.Username, user.Password); err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}

	// Add to sudoers if requested
	if user.Sudoers {
		if err := i.addUserToSudoers(user.Username); err != nil {
			return fmt.Errorf("failed to add to sudoers: %w", err)
		}
		fmt.Printf("    ✓ Added to sudoers\n")
	}

	// Setup SSH key if provided
	if user.SSHPubKey != "" {
		if err := i.setupSSHKey(user.Username, user.SSHPubKey); err != nil {
			return fmt.Errorf("failed to setup SSH key: %w", err)
		}
		fmt.Printf("    ✓ SSH key configured\n")
	} else {
		fmt.Printf("    ⚠ No SSH key provided, password authentication only\n")
	}

	return nil
}

// updateSystemUser updates an existing user's configuration
func (i *Installer) updateSystemUser(user config.SystemUser) error {
	// Update password
	if err := i.setUserPassword(user.Username, user.Password); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Update sudoers status
	isSudoer, err := i.isUserInSudoers(user.Username)
	if err != nil {
		return fmt.Errorf("failed to check sudoers status: %w", err)
	}

	if user.Sudoers && !isSudoer {
		if err := i.addUserToSudoers(user.Username); err != nil {
			return fmt.Errorf("failed to add to sudoers: %w", err)
		}
		fmt.Printf("    ✓ Added to sudoers\n")
	} else if !user.Sudoers && isSudoer {
		if err := i.removeUserFromSudoers(user.Username); err != nil {
			return fmt.Errorf("failed to remove from sudoers: %w", err)
		}
		fmt.Printf("    ✓ Removed from sudoers\n")
	}

	// Update SSH key if provided
	if user.SSHPubKey != "" {
		if err := i.setupSSHKey(user.Username, user.SSHPubKey); err != nil {
			return fmt.Errorf("failed to update SSH key: %w", err)
		}
		fmt.Printf("    ✓ SSH key updated\n")
	}

	return nil
}

// setUserPassword sets the password for a user using chpasswd
func (i *Installer) setUserPassword(username, password string) error {
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, password))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chpasswd failed: %w, output: %s", err, string(output))
	}
	return nil
}

// addUserToSudoers adds a user to the sudo group
func (i *Installer) addUserToSudoers(username string) error {
	cmd := exec.Command("usermod", "-aG", "sudo", username)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("usermod failed: %w, output: %s", err, string(output))
	}
	return nil
}

// removeUserFromSudoers removes a user from the sudo group
func (i *Installer) removeUserFromSudoers(username string) error {
	cmd := exec.Command("gpasswd", "-d", username, "sudo")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gpasswd failed: %w, output: %s", err, string(output))
	}
	return nil
}

// isUserInSudoers checks if a user is in the sudo group
func (i *Installer) isUserInSudoers(username string) (bool, error) {
	output, err := exec.Command("groups", username).Output()
	if err != nil {
		return false, fmt.Errorf("groups command failed: %w", err)
	}
	return strings.Contains(string(output), "sudo"), nil
}

// setupSSHKey configures SSH public key authentication for a user
func (i *Installer) setupSSHKey(username, pubKey string) error {
	// Get user's home directory
	homeDir, err := i.getUserHomeDir(username)
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	sshDir := homeDir + "/.ssh"
	authorizedKeysFile := sshDir + "/authorized_keys"

	// Create .ssh directory if it doesn't exist
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Read existing authorized_keys if it exists
	existingKeys := ""
	if data, err := os.ReadFile(authorizedKeysFile); err == nil {
		existingKeys = string(data)
	}

	// Check if key already exists
	if strings.Contains(existingKeys, pubKey) {
		return nil // Key already present
	}

	// Append the new key
	f, err := os.OpenFile(authorizedKeysFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer f.Close()

	if existingKeys != "" && !strings.HasSuffix(existingKeys, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	if _, err := f.WriteString(pubKey + "\n"); err != nil {
		return fmt.Errorf("failed to write SSH key: %w", err)
	}

	// Set proper ownership
	if err := i.setSSHDirOwnership(username, sshDir); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	return nil
}

// getUserHomeDir gets the home directory for a user
func (i *Installer) getUserHomeDir(username string) (string, error) {
	output, err := exec.Command("getent", "passwd", username).Output()
	if err != nil {
		return "", fmt.Errorf("getent failed: %w", err)
	}

	// Output format: username:x:uid:gid:comment:home:shell
	parts := strings.Split(strings.TrimSpace(string(output)), ":")
	if len(parts) < 6 {
		return "", fmt.Errorf("unexpected getent output format")
	}

	return parts[5], nil
}

// setSSHDirOwnership sets the correct ownership for SSH directory and files
func (i *Installer) setSSHDirOwnership(username, sshDir string) error {
	// Change ownership of .ssh directory
	cmd := exec.Command("chown", "-R", username+":"+username, sshDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown failed: %w, output: %s", err, string(output))
	}

	// Ensure correct permissions
	if err := os.Chmod(sshDir, 0700); err != nil {
		return fmt.Errorf("chmod .ssh failed: %w", err)
	}

	authorizedKeysFile := sshDir + "/authorized_keys"
	if _, err := os.Stat(authorizedKeysFile); err == nil {
		if err := os.Chmod(authorizedKeysFile, 0600); err != nil {
			return fmt.Errorf("chmod authorized_keys failed: %w", err)
		}
	}

	return nil
}
