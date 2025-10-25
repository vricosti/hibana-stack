package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vricosti/hibana-stack/internal/config"
	"github.com/vricosti/hibana-stack/internal/installer"
	"github.com/vricosti/hibana-stack/internal/system"
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
	fmt.Println("🚀 Hibana Stack Installer")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Println()

	// Step 1: Load or create configuration
	var cfg *config.Config
	var err error

	if cfgFile == "" {
		cfgFile = "./hibana-config.json"
	}

	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		fmt.Printf("⚠️  Configuration file not found: %s\n", cfgFile)
		fmt.Println("Creating skeleton configuration file...")

		skeleton := config.GenerateSkeleton()
		data, _ := json.MarshalIndent(skeleton, "", "  ")

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

	// Check if running as root (required for actual installation)
	if os.Geteuid() != 0 {
		return fmt.Errorf("installation must be run as root (use sudo)")
	}

	// Step 2: Check Ubuntu version
	fmt.Println("\n📋 Checking system requirements...")
	if err := system.CheckUbuntuVersion(); err != nil {
		return fmt.Errorf("system check failed: %w", err)
	}
	fmt.Println("✓ Ubuntu 24.04 detected")

	// Step 3: Check required packages
	fmt.Println("\n📦 Checking required packages...")
	missing, err := system.CheckRequiredPackages()
	if err != nil {
		return fmt.Errorf("package check failed: %w", err)
	}

	if len(missing) > 0 {
		fmt.Println("⚠️  Missing packages detected. Installing...")
		if err := system.InstallPackages(missing, cfg.PrimaryDomain); err != nil {
			return fmt.Errorf("package installation failed: %w", err)
		}
		fmt.Println("✓ All packages installed")
	} else {
		fmt.Println("✓ All required packages are installed")
	}

	// Step 4: Initialize installer
	inst := installer.New(cfg)

	// Step 5: Setup PostgreSQL
	fmt.Println("\n🗄️  Setting up PostgreSQL...")
	if err := inst.SetupPostgreSQL(); err != nil {
		return fmt.Errorf("PostgreSQL setup failed: %w", err)
	}
	fmt.Println("✓ PostgreSQL configured")

	// Step 6: Setup PowerDNS
	fmt.Println("\n🌐 Setting up PowerDNS...")
	if err := inst.SetupPowerDNS(); err != nil {
		return fmt.Errorf("PowerDNS setup failed: %w", err)
	}
	fmt.Println("✓ PowerDNS configured")

	// Step 7: Setup Mail Server
	fmt.Println("\n📧 Setting up mail server (Postfix/Dovecot)...")
	if err := inst.SetupMailServer(); err != nil {
		return fmt.Errorf("mail server setup failed: %w", err)
	}
	fmt.Println("✓ Mail server configured")

	// Step 8: Setup DKIM and DMARC
	fmt.Println("\n🔐 Setting up DKIM and DMARC...")
	if err := inst.SetupDKIM(); err != nil {
		return fmt.Errorf("DKIM/DMARC setup failed: %w", err)
	}
	fmt.Println("✓ DKIM and DMARC configured")

	// Step 9: Setup SpamAssassin
	fmt.Println("\n🛡️  Setting up SpamAssassin anti-spam filter...")
	if err := inst.SetupSpamAssassin(); err != nil {
		return fmt.Errorf("SpamAssassin setup failed: %w", err)
	}
	fmt.Println("✓ SpamAssassin configured")

	// Step 10: Configure DNS records
	fmt.Println("\n📝 Adding DNS records to PowerDNS...")
	if err := inst.ConfigureDNSRecords(); err != nil {
		return fmt.Errorf("DNS configuration failed: %w", err)
	}
	fmt.Println("✓ DNS records configured")

	// Step 11: Setup Traefik
	fmt.Println("\n🔀 Setting up Traefik reverse proxy...")
	if err := inst.SetupTraefik(); err != nil {
		return fmt.Errorf("Traefik setup failed: %w", err)
	}
	fmt.Println("✓ Traefik configured")

	// Step 12: Deploy Docker containers
	fmt.Println("\n🐳 Deploying Docker containers...")
	if err := inst.DeployContainers(); err != nil {
		return fmt.Errorf("container deployment failed: %w", err)
	}
	fmt.Println("✓ Containers deployed")

	// Step 13: Test email
	fmt.Println("\n✉️  Testing email functionality...")
	if err := inst.TestEmail(); err != nil {
		fmt.Printf("⚠️  Email test failed: %v\n", err)
		fmt.Println("   You may need to check your configuration manually")
	} else {
		fmt.Println("✓ Email test successful")
	}

	// Step 14: Generate and display credentials summary
	fmt.Println("\n🔐 Generating credentials summary...")
	credentialsSummary, err := inst.GetCredentialsSummary()
	if err != nil {
		fmt.Printf("⚠️  Failed to generate credentials summary: %v\n", err)
	} else {
		if err := inst.SaveCredentialsSummary(credentialsSummary); err != nil {
			fmt.Printf("⚠️  Failed to save credentials summary: %v\n", err)
		}
		inst.DisplayCredentialsSummary(credentialsSummary)
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
