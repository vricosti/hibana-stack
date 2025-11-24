package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vricosti/hibana-stack/internal/ansible"
	"github.com/vricosti/hibana-stack/internal/config"
	"github.com/vricosti/hibana-stack/internal/dnsprovider"
)

const templateFileName = "hibana-config.skel.yaml"

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
		fmt.Println("Creating configuration file from template...")

		if err := generateConfigFromTemplate(cfgFile); err != nil {
			return err
		}

		fmt.Println("\nPlease edit this file with your domain and email settings, then run 'sudo hibana init' again.")
		return nil
	}

	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("✓ Configuration loaded for domain: %s\n", cfg.PrimaryDomain)

	// Step 1.5: Verify DNS provider if specified
	if cfg.DNSProvider != nil && cfg.DNSProvider.Name != "" && cfg.DNSProvider.APIToken != "" {
		if err := dnsprovider.VerifyDomainOwnership(cfg.DNSProvider.Name, cfg.DNSProvider.APIToken, cfg.PrimaryDomain); err != nil {
			return fmt.Errorf("DNS provider verification failed: %w\n\nPlease verify:\n  • Your API token is valid\n  • Domain %s is managed by your %s account\n  • The API has proper permissions", err, cfg.PrimaryDomain, cfg.DNSProvider.Name)
		}
	}

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

	// Step 7.5: Build API and frontend
	fmt.Println("\n🔨 Building API and admin interface...")
	if err := ansible.BuildAPIAndFrontend(workspaceDir); err != nil {
		fmt.Printf("⚠️  Warning: Failed to build API/frontend: %v\n", err)
		fmt.Println("   API will use placeholder. You can build manually later with:")
		fmt.Println("   ./build-all.sh && docker-compose -f /srv/<domain>/api/docker-compose.yml up -d --build")
	} else {
		fmt.Println("✓ API and frontend built successfully")
	}

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

	// Step 10: Configure DNS provider if specified (after successful installation)
	if cfg.DNSProvider != nil && cfg.DNSProvider.Name != "" && cfg.DNSProvider.APIToken != "" {
		// Automatically configure DNS records
		simulate := false
		if err := dnsprovider.UpdateDNSRecords(cfg.DNSProvider.Name, cfg.DNSProvider.APIToken, cfg.PrimaryDomain, cfg.ServerIP, simulate); err != nil {
			fmt.Printf("\n⚠️  Warning: DNS provider configuration failed: %v\n", err)
			fmt.Printf("Please configure DNS records manually:\n")
			fmt.Printf("  • ns1.%s  A  %s  (TTL: 14400)\n", cfg.PrimaryDomain, cfg.ServerIP)
			fmt.Printf("  • ns2.%s  A  %s  (TTL: 14400)\n", cfg.PrimaryDomain, cfg.ServerIP)
			fmt.Printf("  • Update nameservers to: ns1.%s, ns2.%s\n\n", cfg.PrimaryDomain, cfg.PrimaryDomain)
		}
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

	if cfg.DNSProvider == nil || cfg.DNSProvider.Name == "" {
		fmt.Println("Don't forget to update your domain's nameservers to point to this server!")
	} else {
		fmt.Println("DNS records have been automatically configured via your DNS provider.")
	}

	return nil
}

// generateConfigFromTemplate creates a config file from the template with random passwords
func generateConfigFromTemplate(outputFile string) error {
	// Check if template file exists in current directory
	if _, err := os.Stat(templateFileName); os.IsNotExist(err) {
		return fmt.Errorf("template file not found: %s (run this command from the hibana-stack directory)", templateFileName)
	}

	// Read template file
	templateData, err := os.ReadFile(templateFileName)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Generate random passwords
	webadminPassword := generateRandomPassword(16)
	emailPassword := generateRandomPassword(16)
	domainUserPassword := generateRandomPassword(16)

	// Replace placeholders with random passwords
	content := string(templateData)
	content = strings.Replace(content, "__RANDOM_WEBADMIN_PASSWORD__", webadminPassword, 1)
	content = strings.Replace(content, "__RANDOM_EMAIL_PASSWORD__", emailPassword, 1)
	content = strings.Replace(content, "__RANDOM_DOMAIN_USER_PASSWORD__", domainUserPassword, 1)

	// Write to output file
	if err := os.WriteFile(outputFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Configuration file created: %s\n\n", outputFile)
	fmt.Println("Generated credentials:")
	fmt.Printf("  • Web admin password: %s\n", webadminPassword)
	fmt.Printf("  • Email account password: %s\n", emailPassword)
	fmt.Printf("  • Domain user password: %s\n", domainUserPassword)

	return nil
}

// generateRandomPassword generates a random password of the specified length
func generateRandomPassword(length int) string {
	// Generate random bytes
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "HibanaDefaultPwd123!"
	}

	// Encode to base64 and clean up
	password := base64.URLEncoding.EncodeToString(bytes)
	password = strings.ReplaceAll(password, "-", "")
	password = strings.ReplaceAll(password, "_", "")
	password = strings.ReplaceAll(password, "=", "")

	if len(password) > length {
		password = password[:length]
	}

	return password
}
