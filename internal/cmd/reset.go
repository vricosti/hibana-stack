package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vricosti/hibana-stack/internal/ansible"
	"github.com/vricosti/hibana-stack/internal/config"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Remove all Hibana Stack packages and configuration",
	Long: `⚠️  DANGER: This will completely remove all installed packages and configurations.
This includes:
- PostgreSQL (and ALL databases)
- PowerDNS
- Postfix and Dovecot (mail server)
- Docker containers
- Traefik
- SpamAssassin
- All configuration files

This action CANNOT be undone!`,
	RunE: runReset,
}

var forceReset bool

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().BoolVarP(&forceReset, "force", "f", false, "Skip confirmation prompt")
}

func runReset(cmd *cobra.Command, args []string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root (use sudo)")
	}

	fmt.Println("⚠️  ⚠️  ⚠️  DANGER ZONE ⚠️  ⚠️  ⚠️")
	fmt.Println()
	fmt.Println("This will COMPLETELY REMOVE:")
	fmt.Println("  • All Hibana Stack packages")
	fmt.Println("  • PostgreSQL (INCLUDING ALL DATABASES)")
	fmt.Println("  • PowerDNS and all DNS records")
	fmt.Println("  • Mail server (Postfix/Dovecot) and all emails")
	fmt.Println("  • Docker containers and images")
	fmt.Println("  • Traefik reverse proxy")
	fmt.Println("  • All configuration files")
	fmt.Println("  • UFW firewall rules added by Hibana")
	fmt.Println("  • Modifications to /etc/hosts")
	fmt.Println()
	fmt.Println("⚠️  THIS ACTION CANNOT BE UNDONE! ⚠️")
	fmt.Println()

	// Ask for confirmation unless --force is used
	if !forceReset {
		fmt.Print("Type 'YES' in capital letters to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(response)
		if response != "YES" {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	fmt.Println()
	fmt.Println("🗑️  Starting cleanup process with Ansible...")
	fmt.Println()

	// Load configuration if it exists (to get primary_domain for cleanup)
	var cfg *config.Config
	if cfgFile == "" {
		cfgFile = "./hibana-config.yaml"
	}

	if _, err := os.Stat(cfgFile); err == nil {
		cfg, _ = config.LoadConfig(cfgFile)
	} else {
		// Create minimal config for reset
		cfg = &config.Config{
			PrimaryDomain: "localhost", // Default fallback
		}
	}

	// Check Ansible installation
	fmt.Println("📋 Checking Ansible installation...")
	if err := ansible.CheckAnsibleInstalled(); err != nil {
		fmt.Printf("⚠️  Ansible is not installed\n\n")

		// Install Ansible automatically
		if err := ansible.InstallAnsible(); err != nil {
			fmt.Println("\n" + ansible.InstallInstructions())
			return fmt.Errorf("failed to install ansible: %w", err)
		}

		// Verify installation
		if err := ansible.CheckAnsibleInstalled(); err != nil {
			return fmt.Errorf("ansible installation verification failed: %w", err)
		}
	}

	version, _ := ansible.GetAnsibleVersion()
	fmt.Printf("✓ Ansible %s detected\n", version)

	// Create Ansible workspace
	fmt.Println("\n📂 Creating Ansible workspace...")
	workspaceDir, err := ansible.CreateWorkspace()
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	defer os.RemoveAll(workspaceDir) // Cleanup on exit

	fmt.Printf("✓ Workspace created: %s\n", workspaceDir)

	// Generate inventory
	fmt.Println("\n📝 Generating Ansible inventory...")
	if err := ansible.GenerateInventory(cfg, workspaceDir); err != nil {
		return fmt.Errorf("failed to generate inventory: %w", err)
	}
	fmt.Println("✓ Inventory generated")

	// Generate group variables
	fmt.Println("\n📝 Generating Ansible variables...")
	if err := ansible.GenerateGroupVars(cfg, workspaceDir); err != nil {
		return fmt.Errorf("failed to generate group vars: %w", err)
	}
	fmt.Println("✓ Variables generated")

	// Copy reset role
	fmt.Println("\n📦 Copying reset role...")
	if err := ansible.CopyRoles(workspaceDir); err != nil {
		return fmt.Errorf("failed to copy roles: %w", err)
	}
	fmt.Println("✓ Reset role copied")

	// Copy reset playbook
	fmt.Println("\n📋 Copying reset playbook...")
	if err := ansible.CopyResetPlaybook(workspaceDir); err != nil {
		return fmt.Errorf("failed to copy reset playbook: %w", err)
	}
	fmt.Println("✓ Reset playbook copied")

	// Execute ansible-playbook for reset
	fmt.Println("\n🚀 Executing Ansible reset playbook...")
	fmt.Println(string(make([]byte, 80)))

	if err := ansible.ExecuteResetPlaybook(workspaceDir, verbose); err != nil {
		fmt.Println("\n" + string(make([]byte, 80)))
		fmt.Println("❌ Ansible reset playbook execution failed!")
		return fmt.Errorf("playbook execution failed: %w", err)
	}

	// Final message
	fmt.Println("\n" + string(make([]byte, 80)))
	fmt.Println("✓ Cleanup complete!")
	fmt.Println(string(make([]byte, 80)))
	fmt.Println()
	fmt.Println("The system has been reset. You can now run 'sudo hibana init' for a fresh installation.")

	return nil
}
