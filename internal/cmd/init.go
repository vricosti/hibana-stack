package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vricosti/hibana-stack/internal/ansible"
	"github.com/vricosti/hibana-stack/internal/config"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Hibana Stack on this server",
	Long:  `Performs initial setup of PowerDNS, mail server, Traefik, and web interfaces`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Hibana Stack Installer (Ansible)")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Println()

	// Step 1: Load or create configuration
	var cfg *config.Config
	var err error

	if cfgFile == "" {
		cfgFile = "./hibana-config.yaml"
	}

	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		fmt.Printf("⚠️  Configuration file not found: %s\n", cfgFile)
		fmt.Println("Creating skeleton configuration file...")

		skeleton := config.GenerateSkeleton()
		data, _ := yaml.Marshal(skeleton)

		if err := os.WriteFile(cfgFile, data, 0600); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}

		fmt.Printf("✓ Configuration skeleton created: %s\n", cfgFile)
		fmt.Println("\nPlease edit this file with your domain and email settings, then run 'sudo hibana init' again.")
		return nil
	}

	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("✓ Configuration loaded for domain: %s\n", cfg.PrimaryDomain)

	// Step 2: Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("installation must be run as root (use sudo)")
	}

	// Step 3: Check Ansible installation
	fmt.Println("\n📋 Checking Ansible installation...")
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

	// Step 4: Create Ansible workspace
	fmt.Println("\n📂 Creating Ansible workspace...")
	workspaceDir, err := ansible.CreateWorkspace()
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	defer os.RemoveAll(workspaceDir) // Cleanup on exit

	fmt.Printf("✓ Workspace created: %s\n", workspaceDir)

	// Step 5: Generate inventory
	fmt.Println("\n📝 Generating Ansible inventory...")
	if err := ansible.GenerateInventory(cfg, workspaceDir); err != nil {
		return fmt.Errorf("failed to generate inventory: %w", err)
	}
	fmt.Println("✓ Inventory generated")

	// Step 6: Generate group variables
	fmt.Println("\n📝 Generating Ansible variables from configuration...")
	if err := ansible.GenerateGroupVars(cfg, workspaceDir); err != nil {
		return fmt.Errorf("failed to generate group vars: %w", err)
	}
	fmt.Println("✓ Variables generated")

	// Step 7: Copy Ansible roles
	fmt.Println("\n📦 Copying Ansible roles...")
	if err := ansible.CopyRoles(workspaceDir); err != nil {
		return fmt.Errorf("failed to copy roles: %w", err)
	}
	fmt.Println("✓ Roles copied")

	// Step 8: Copy playbook
	fmt.Println("\n📋 Copying Ansible playbook...")
	if err := ansible.CopyPlaybook(workspaceDir); err != nil {
		return fmt.Errorf("failed to copy playbook: %w", err)
	}
	fmt.Println("✓ Playbook copied")

	// Step 9: Execute ansible-playbook
	fmt.Println("\n🚀 Executing Ansible playbook...")
	fmt.Println(string(make([]byte, 80)))

	if err := ansible.ExecutePlaybook(workspaceDir, verbose); err != nil {
		fmt.Println("\n" + string(make([]byte, 80)))
		fmt.Println("❌ Ansible playbook execution failed!")
		return fmt.Errorf("playbook execution failed: %w", err)
	}

	// Final summary
	fmt.Println("\n" + string(make([]byte, 80)))
	fmt.Println("🎉 Hibana Stack installation complete!")
	fmt.Println(string(make([]byte, 80)))
	fmt.Println()
	fmt.Printf("Your services are now available at:\n")
	fmt.Printf("  • Web admin:  https://adm.%s\n", cfg.PrimaryDomain)
	fmt.Printf("  • Webmail:    https://webmail.%s\n", cfg.PrimaryDomain)
	fmt.Printf("  • Website:    https://www.%s\n", cfg.PrimaryDomain)
	fmt.Printf("  • Mail:       mail.%s\n", cfg.PrimaryDomain)
	fmt.Println()
	fmt.Println("Don't forget to update your domain's nameservers to point to this server!")

	return nil
}
