package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vricosti/hibana-stack/internal/ansible"
	"github.com/vricosti/hibana-stack/internal/config"
	"github.com/vricosti/hibana-stack/internal/dnsprovider"
	"github.com/vricosti/hibana-stack/internal/system"
	"golang.org/x/net/idna"
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
	fmt.Println("Hibana Stack Installer (Ansible)")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Println()

	// Step 1: Load or create configuration
	var cfg *config.Config
	var err error

	if cfgFile == "" {
		cfgFile = "./hibana-config.yaml"
	}

	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		fmt.Printf("Configuration file not found: %s\n", cfgFile)

		// Interactive mode: prompt for configuration
		cfg, err = InteractiveConfig()
		if err != nil {
			return fmt.Errorf("interactive configuration failed: %w", err)
		}

		// Save the configuration
		if err := SaveConfig(cfg, cfgFile); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Println("\nConfiguration created. Continuing with installation...")
	} else {
		// Load existing configuration file
		cfg, err = config.LoadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	primaryDomain := cfg.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	fmt.Printf("Configuration loaded for primary domain: %s\n", primaryDomain.Name)
	if len(cfg.Domains) > 1 {
		fmt.Printf("Additional domains: %d\n", len(cfg.Domains)-1)
		for _, d := range cfg.GetSecondaryDomains() {
			fmt.Printf("  - %s\n", d.Name)
		}
	}

	// Step 1.5: Verify DNS provider if specified
	if cfg.DNSProvider != nil && cfg.DNSProvider.Type != "manual" {
		// Verify ownership of all domains
		for _, domain := range cfg.Domains {
			if err := dnsprovider.VerifyDomainOwnership(cfg.DNSProvider.Name, cfg.DNSProvider.APIToken, domain.Name); err != nil {
				return fmt.Errorf("DNS provider verification failed for %s: %w\n\nPlease verify:\n  - Your API token is valid\n  - Domain %s is managed by your %s account\n  - The API has proper permissions", domain.Name, err, domain.Name, cfg.DNSProvider.Name)
			}
		}
	}

	// Step 2: Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("installation must be run as root (use sudo)")
	}

	// Step 3: Check Ansible installation
	fmt.Println("\nChecking Ansible installation...")
	if err := ansible.CheckAnsibleInstalled(); err != nil {
		fmt.Printf("Ansible is not installed\n\n")

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
	fmt.Printf("Ansible %s detected\n", version)

	// Step 4: Create Ansible workspace
	fmt.Println("\nCreating Ansible workspace...")
	workspaceDir, err := ansible.CreateWorkspace()
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	defer os.RemoveAll(workspaceDir) // Cleanup on exit

	fmt.Printf("Workspace created: %s\n", workspaceDir)

	// Step 5: Generate inventory
	fmt.Println("\nGenerating Ansible inventory...")
	if err := ansible.GenerateInventory(cfg, workspaceDir); err != nil {
		return fmt.Errorf("failed to generate inventory: %w", err)
	}
	fmt.Println("Inventory generated")

	// Step 6: Generate group variables
	fmt.Println("\nGenerating Ansible variables from configuration...")
	if err := ansible.GenerateGroupVars(cfg, workspaceDir); err != nil {
		return fmt.Errorf("failed to generate group vars: %w", err)
	}
	fmt.Println("Variables generated")

	// Step 7: Copy Ansible roles
	fmt.Println("\nCopying Ansible roles...")
	if err := ansible.CopyRoles(workspaceDir); err != nil {
		return fmt.Errorf("failed to copy roles: %w", err)
	}
	fmt.Println("Roles copied")

	// Step 7.5: Build API and frontend
	fmt.Println("\nBuilding API and admin interface...")
	if err := ansible.BuildAPIAndFrontend(workspaceDir); err != nil {
		fmt.Printf("Warning: Failed to build API/frontend: %v\n", err)
		fmt.Println("   API will use placeholder. You can build manually later with:")
		fmt.Println("   ./build-all.sh && docker-compose -f /srv/<domain>/api/docker-compose.yml up -d --build")
	} else {
		fmt.Println("API and frontend built successfully")
	}

	// Step 8: Copy playbook
	fmt.Println("\nCopying Ansible playbook...")
	if err := ansible.CopyPlaybook(workspaceDir); err != nil {
		return fmt.Errorf("failed to copy playbook: %w", err)
	}
	fmt.Println("Playbook copied")

	// Step 8.5: Configure DNS records BEFORE starting containers (PRE-INSTALL)
	if cfg.DNSProvider != nil && cfg.DNSProvider.Type != "manual" {
		fmt.Println("\n" + string(make([]byte, 80)))
		fmt.Println("STEP: DNS PRE-CONFIGURATION")
		fmt.Println(string(make([]byte, 80)))

		// Configure DNS for all domains (A and MX records only)
		for _, domain := range cfg.Domains {
			domainCfg := dnsprovider.DomainConfig{
				Name:       domain.Name,
				Subdomains: make([]dnsprovider.SubdomainConfig, len(domain.Subdomains)),
			}
			for i, sub := range domain.Subdomains {
				domainCfg.Subdomains[i] = dnsprovider.SubdomainConfig{
					Name: sub.Name,
					Role: sub.Role,
				}
			}

			// Use OVH-specific function if provider is OVH
			if cfg.DNSProvider.Name == "ovh" || cfg.DNSProvider.Name == "ovhcloud" {
				ovhCreds := dnsprovider.OVHCloudCredentials{
					Endpoint:          cfg.DNSProvider.Endpoint,
					ApplicationKey:    cfg.DNSProvider.ApplicationKey,
					ApplicationSecret: cfg.DNSProvider.ApplicationSecret,
					ConsumerKey:       cfg.DNSProvider.ConsumerKey,
				}
				if err := dnsprovider.UpdateDNSRecordsPreInstallOVH(ovhCreds, domainCfg, cfg.ServerIP); err != nil {
					return fmt.Errorf("DNS pre-install configuration failed for %s: %w\n\nPlease check your DNS provider settings and try again.", domain.Name, err)
				}
			} else {
				if err := dnsprovider.UpdateDNSRecordsPreInstall(cfg.DNSProvider.Name, cfg.DNSProvider.Type, cfg.DNSProvider.APIToken, domainCfg, cfg.ServerIP); err != nil {
					return fmt.Errorf("DNS pre-install configuration failed for %s: %w\n\nPlease check your DNS provider settings and try again.", domain.Name, err)
				}
			}
		}

		// Step 8.6: Verify DNS propagation for all domains
		fmt.Println("\n" + string(make([]byte, 80)))
		fmt.Println("STEP: VERIFYING DNS PROPAGATION")
		fmt.Println(string(make([]byte, 80)))
		fmt.Println("\nWaiting for DNS records to propagate before starting containers...")
		fmt.Println("This ensures SSL certificates can be generated successfully.\n")

		for _, domain := range cfg.Domains {
			// Convert domain to Punycode for DNS verification
			domainASCII, err := idna.ToASCII(domain.Name)
			if err != nil {
				return fmt.Errorf("invalid domain name %s: %w", domain.Name, err)
			}

			domainCfg := dnsprovider.DomainConfig{
				Name:       domainASCII, // Use Punycode version for DNS queries
				Subdomains: make([]dnsprovider.SubdomainConfig, len(domain.Subdomains)),
			}
			for i, sub := range domain.Subdomains {
				domainCfg.Subdomains[i] = dnsprovider.SubdomainConfig{
					Name: sub.Name,
					Role: sub.Role,
				}
			}

			// Wait up to 10 minutes (60 attempts × 10 seconds)
			if err := dnsprovider.VerifyDNSPropagation(domainCfg, cfg.ServerIP, 60, 10); err != nil {
				return fmt.Errorf("DNS propagation verification failed: %w", err)
			}
		}

		fmt.Println("\n" + string(make([]byte, 80)))
	}

	// Step 9: Execute ansible-playbook (NOW that DNS is propagated)
	fmt.Println("\nExecuting Ansible playbook...")
	fmt.Println(string(make([]byte, 80)))

	if err := ansible.ExecutePlaybook(workspaceDir, verbose); err != nil {
		fmt.Println("\n" + string(make([]byte, 80)))
		fmt.Println("Ansible playbook execution failed!")
		return fmt.Errorf("playbook execution failed: %w", err)
	}

	// Step 10: Configure remaining DNS records (POST-INSTALL)
	if cfg.DNSProvider != nil && cfg.DNSProvider.Type != "manual" {
		fmt.Println("\n" + string(make([]byte, 80)))
		fmt.Println("STEP: DNS POST-CONFIGURATION")
		fmt.Println(string(make([]byte, 80)))

		// Configure SPF and DMARC records for all domains
		for _, domain := range cfg.Domains {
			domainCfg := dnsprovider.DomainConfig{
				Name:       domain.Name,
				Subdomains: make([]dnsprovider.SubdomainConfig, len(domain.Subdomains)),
			}
			for i, sub := range domain.Subdomains {
				domainCfg.Subdomains[i] = dnsprovider.SubdomainConfig{
					Name: sub.Name,
					Role: sub.Role,
				}
			}

			// Use OVH-specific function if provider is OVH
			if cfg.DNSProvider.Name == "ovh" || cfg.DNSProvider.Name == "ovhcloud" {
				ovhCreds := dnsprovider.OVHCloudCredentials{
					Endpoint:          cfg.DNSProvider.Endpoint,
					ApplicationKey:    cfg.DNSProvider.ApplicationKey,
					ApplicationSecret: cfg.DNSProvider.ApplicationSecret,
					ConsumerKey:       cfg.DNSProvider.ConsumerKey,
				}
				if err := dnsprovider.UpdateDNSRecordsPostInstallOVH(ovhCreds, domainCfg, cfg.ServerIP); err != nil {
					fmt.Printf("\n  Warning: DNS post-install configuration failed for %s: %v\n", domain.Name, err)
				}
			} else {
				if err := dnsprovider.UpdateDNSRecordsPostInstall(cfg.DNSProvider.Name, cfg.DNSProvider.Type, cfg.DNSProvider.APIToken, domainCfg, cfg.ServerIP); err != nil {
					fmt.Printf("\n  Warning: DNS post-install configuration failed for %s: %v\n", domain.Name, err)
				}
			}
		}

		// Configure PTR records if mailserver role is enabled on primary domain
		if primaryDomain.HasSubdomainRole("mailserver") {
			if err := dnsprovider.ConfigurePTR(cfg.DNSProvider.Name, cfg.DNSProvider.APIToken, cfg.ServerIP, primaryDomain.Name); err != nil {
				fmt.Printf("\n  Warning: PTR configuration failed: %v\n", err)
			}
		}

		// Add DKIM records for all domains with mailserver role
		for _, domain := range cfg.Domains {
			if domain.HasSubdomainRole("mailserver") || domain.IsPrimary {
				// Read the DKIM public key generated by Ansible
				dkimKey, err := dnsprovider.ReadDKIMPublicKey(domain.Name)
				if err != nil {
					fmt.Printf("\n  Warning: Could not read DKIM key for %s: %v\n", domain.Name, err)
					continue
				}

				// Add DKIM record to DNS
				if cfg.DNSProvider.Name == "ovh" || cfg.DNSProvider.Name == "ovhcloud" {
					ovhCreds := dnsprovider.OVHCloudCredentials{
						Endpoint:          cfg.DNSProvider.Endpoint,
						ApplicationKey:    cfg.DNSProvider.ApplicationKey,
						ApplicationSecret: cfg.DNSProvider.ApplicationSecret,
						ConsumerKey:       cfg.DNSProvider.ConsumerKey,
					}
					if err := dnsprovider.AddDKIMRecordOVH(ovhCreds, domain.Name, dkimKey); err != nil {
						fmt.Printf("\n  Warning: Could not add DKIM record for %s: %v\n", domain.Name, err)
					}
				} else {
					if err := dnsprovider.AddDKIMRecord(cfg.DNSProvider.Name, cfg.DNSProvider.APIToken, domain.Name, dkimKey); err != nil {
						fmt.Printf("\n  Warning: Could not add DKIM record for %s: %v\n", domain.Name, err)
					}
				}
			}
		}
	} else if primaryDomain.HasSubdomainRole("mailserver") {
		// No DNS provider configured but mailserver is enabled - show manual PTR instructions
		fmt.Println("\nPTR records must be configured manually for email deliverability:")
		fmt.Printf("    Set PTR for IPv4 to: mx.%s\n", primaryDomain.Name)
		fmt.Printf("    Set PTR for IPv6 to: mx.%s\n", primaryDomain.Name)
	}

	// Build list of enabled roles for primary domain
	enabledRoles := make(map[string]bool)
	for _, sub := range primaryDomain.Subdomains {
		enabledRoles[sub.Role] = true
	}

	// Final summary
	fmt.Println("\n" + string(make([]byte, 80)))
	fmt.Println("Hibana Stack installation complete!")
	fmt.Println(string(make([]byte, 80)))
	fmt.Println()
	fmt.Printf("Your services are now available at:\n")
	if enabledRoles["webadmin"] {
		fmt.Printf("  - Web admin:  https://adm.%s\n", primaryDomain.Name)
	}
	if enabledRoles["webmail"] {
		fmt.Printf("  - Webmail:    https://webmail.%s\n", primaryDomain.Name)
	}
	if enabledRoles["website"] {
		fmt.Printf("  - Website:    https://www.%s\n", primaryDomain.Name)
	}
	if enabledRoles["mailserver"] {
		fmt.Printf("  - Mail:       mx.%s\n", primaryDomain.Name)
	}

	// Show secondary domains
	if len(cfg.Domains) > 1 {
		fmt.Println("\nSecondary domains configured:")
		for _, d := range cfg.GetSecondaryDomains() {
			fmt.Printf("  - %s\n", d.Name)
			for _, sub := range d.Subdomains {
				if sub.Role == "webmail" {
					fmt.Printf("      Webmail: https://webmail.%s\n", d.Name)
				}
				if sub.Role == "website" {
					fmt.Printf("      Website: https://www.%s\n", d.Name)
				}
			}
		}
	}

	fmt.Println()

	if cfg.DNSProvider == nil || cfg.DNSProvider.Type == "manual" {
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

	// Generate random passwords and replace all __RANDOM__ placeholders
	content := string(templateData)
	re := regexp.MustCompile(`__RANDOM__`)
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		return generateRandomPassword(16)
	})

	// Try to detect and fill in the server IP address
	if strings.Contains(content, "YOUR_SERVER_IP") {
		serverIP, err := system.GetServerIP()
		if err != nil {
			fmt.Printf("⚠ Warning: Could not detect server IP address: %v\n", err)
			fmt.Println("  Please manually update server_ip in the configuration file.")
		} else {
			content = strings.ReplaceAll(content, "YOUR_SERVER_IP", serverIP)
			fmt.Printf("✓ Server IP address detected: %s\n", serverIP)
		}
	}

	// Write to output file
	if err := os.WriteFile(outputFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Configuration file created: %s\n\n", outputFile)
	fmt.Println("Random passwords have been generated for all accounts.")
	fmt.Println("Please review and edit the configuration file before running 'sudo hibana init' again.")

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
