package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add resources to Hibana Stack",
	Long:  `Add domains, users, or other resources to your Hibana Stack installation`,
}

var addDomainCmd = &cobra.Command{
	Use:   "domain <domain-name>",
	Short: "Prepare a domain for use with Hibana Stack",
	Long: `Creates the system user and directory structure for a domain.
This must be run on the host before adding the domain via the web interface.

Example:
  sudo hibana add domain vricosti.com`,
	Args: cobra.ExactArgs(1),
	RunE: runAddDomain,
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.AddCommand(addDomainCmd)
}

func runAddDomain(cmd *cobra.Command, args []string) error {
	domainName := strings.ToLower(strings.TrimSpace(args[0]))

	// Validate domain name format
	if !isValidDomainName(domainName) {
		return fmt.Errorf("invalid domain name: %s", domainName)
	}

	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root (use sudo)")
	}

	// Convert domain to system name (e.g., vricosti.com -> vricosti-com)
	systemName := domainToSystemName(domainName)
	domainDir := filepath.Join("/srv", systemName)

	fmt.Printf("Preparing domain: %s\n", domainName)
	fmt.Printf("  System name: %s\n", systemName)
	fmt.Printf("  Directory: %s\n", domainDir)
	fmt.Println()

	// Step 1: Check if already exists
	if _, err := os.Stat(domainDir); err == nil {
		fmt.Printf("Directory %s already exists.\n", domainDir)
		// Check if user exists
		if err := exec.Command("id", systemName).Run(); err == nil {
			fmt.Printf("User %s already exists.\n", systemName)
			fmt.Println("\nDomain is already prepared. You can add it via the web interface.")
			return nil
		}
	}

	// Step 2: Ensure hibana-domains group exists
	fmt.Println("1. Ensuring hibana-domains group exists...")
	if err := exec.Command("getent", "group", "hibana-domains").Run(); err != nil {
		output, err := exec.Command("groupadd", "hibana-domains").CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create group: %w, output: %s", err, string(output))
		}
		fmt.Println("   Created hibana-domains group")
	} else {
		fmt.Println("   Group already exists")
	}

	// Step 3: Create directory
	fmt.Println("2. Creating domain directory...")
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	fmt.Printf("   Created %s\n", domainDir)

	// Step 4: Create user
	fmt.Println("3. Creating domain user...")
	if err := exec.Command("id", systemName).Run(); err == nil {
		fmt.Printf("   User %s already exists\n", systemName)
	} else {
		output, err := exec.Command("useradd",
			"-d", domainDir,
			"-s", "/bin/bash",
			"-g", "hibana-domains",
			"-c", fmt.Sprintf("Domain user for %s", domainName),
			systemName,
		).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create user: %w, output: %s", err, string(output))
		}
		fmt.Printf("   Created user %s\n", systemName)
	}

	// Step 5: Set directory ownership
	fmt.Println("4. Setting directory ownership...")
	output, err := exec.Command("chown", "-R", systemName+":hibana-domains", domainDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set ownership: %w, output: %s", err, string(output))
	}
	if err := os.Chmod(domainDir, 0700); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	fmt.Println("   Ownership set")

	// Step 6: Disable password authentication
	fmt.Println("5. Disabling password authentication...")
	output, err = exec.Command("usermod", "-L", systemName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to lock user: %w, output: %s", err, string(output))
	}
	fmt.Println("   Password authentication disabled")

	// Step 7: Add to docker group
	fmt.Println("6. Adding user to docker group...")
	output, err = exec.Command("usermod", "-aG", "docker", systemName).CombinedOutput()
	if err != nil {
		fmt.Printf("   Warning: Could not add to docker group: %s\n", string(output))
	} else {
		fmt.Println("   Added to docker group")
	}

	// Step 8: Generate SSH key
	fmt.Println("7. Generating SSH key pair...")
	sshDir := filepath.Join(domainDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	keyPath := filepath.Join(sshDir, "id_ed25519")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		output, err = exec.Command("ssh-keygen",
			"-t", "ed25519",
			"-f", keyPath,
			"-N", "",
			"-C", fmt.Sprintf("%s@%s", systemName, domainName),
		).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to generate SSH key: %w, output: %s", err, string(output))
		}

		// Copy public key to authorized_keys
		pubKey, err := os.ReadFile(keyPath + ".pub")
		if err != nil {
			return fmt.Errorf("failed to read public key: %w", err)
		}
		authKeysPath := filepath.Join(sshDir, "authorized_keys")
		if err := os.WriteFile(authKeysPath, pubKey, 0600); err != nil {
			return fmt.Errorf("failed to write authorized_keys: %w", err)
		}

		fmt.Println("   SSH key pair generated")
	} else {
		fmt.Println("   SSH key already exists")
	}

	// Set SSH directory ownership (use hibana-domains group)
	output, err = exec.Command("chown", "-R", systemName+":hibana-domains", sshDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set SSH ownership: %w, output: %s", err, string(output))
	}
	os.Chmod(sshDir, 0700)
	os.Chmod(filepath.Join(sshDir, "authorized_keys"), 0600)

	// Step 9: Configure sudoers for docker commands
	fmt.Println("8. Configuring sudoers for docker commands...")
	sudoersFile := fmt.Sprintf("/etc/sudoers.d/hibana-domain-%s", systemName)
	sudoersContent := fmt.Sprintf(`# Hibana domain user sudoers for %s
%s ALL=(ALL) NOPASSWD: /usr/bin/docker ps
%s ALL=(ALL) NOPASSWD: /usr/bin/docker ps -a
%s ALL=(ALL) NOPASSWD: /usr/bin/docker logs *
%s ALL=(ALL) NOPASSWD: /usr/bin/docker inspect *
%s ALL=(ALL) NOPASSWD: /usr/bin/docker restart *%s*
%s ALL=(ALL) NOPASSWD: /usr/bin/docker stop *%s*
%s ALL=(ALL) NOPASSWD: /usr/bin/docker start *%s*
%s ALL=(ALL) NOPASSWD: /usr/bin/docker exec -it *%s* *
%s ALL=(ALL) NOPASSWD: /usr/bin/docker-compose -f /srv/%s/* up -d
%s ALL=(ALL) NOPASSWD: /usr/bin/docker-compose -f /srv/%s/* down
%s ALL=(ALL) NOPASSWD: /usr/bin/docker-compose -f /srv/%s/* restart
%s ALL=(ALL) NOPASSWD: /usr/bin/docker-compose -f /srv/%s/* logs
%s ALL=(ALL) NOPASSWD: /usr/bin/docker-compose -f /srv/%s/* ps
`,
		domainName,
		systemName, systemName, systemName, systemName,
		systemName, systemName,
		systemName, systemName,
		systemName, systemName,
		systemName, systemName,
		systemName, systemName,
		systemName, systemName,
		systemName, systemName,
		systemName, systemName,
		systemName, systemName,
	)

	if err := os.WriteFile(sudoersFile, []byte(sudoersContent), 0440); err != nil {
		return fmt.Errorf("failed to write sudoers file: %w", err)
	}

	// Validate sudoers file
	output, err = exec.Command("visudo", "-cf", sudoersFile).CombinedOutput()
	if err != nil {
		os.Remove(sudoersFile)
		return fmt.Errorf("invalid sudoers syntax: %w, output: %s", err, string(output))
	}
	fmt.Println("   Sudoers configured")

	// Done
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Domain %s is now prepared!\n", domainName)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("You can now add this domain via the web interface at:")
	fmt.Println("  https://adm.<your-primary-domain>/")
	fmt.Println()
	fmt.Println("SSH private key location:")
	fmt.Printf("  %s\n", filepath.Join(sshDir, "id_ed25519"))
	fmt.Println()

	return nil
}

// domainToSystemName converts a domain name to a valid system name
// e.g., vricosti.com -> vricosti-com
func domainToSystemName(domain string) string {
	return strings.ReplaceAll(domain, ".", "-")
}

// isValidDomainName checks if a domain name is valid
func isValidDomainName(domain string) bool {
	if len(domain) < 4 || len(domain) > 253 {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	// Basic validation - alphanumeric, hyphens, and dots
	for _, c := range domain {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
			return false
		}
	}
	return true
}
