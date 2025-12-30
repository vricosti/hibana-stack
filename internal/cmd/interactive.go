package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vricosti/hibana-stack/internal/config"
	"github.com/vricosti/hibana-stack/internal/dnsprovider"
	"github.com/vricosti/hibana-stack/internal/system"
)

// InteractiveConfig prompts the user for configuration when config file doesn't exist
func InteractiveConfig() (*config.Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nNo configuration file found. Let's create one interactively.\n")

	// Step 1: Domain name
	fmt.Print("Domain name (ex: mycompany.com): ")
	domainName, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read domain name: %w", err)
	}
	domainName = strings.TrimSpace(domainName)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	// Step 2: DNS Provider - use registry to list available providers
	supportedProviders := dnsprovider.SupportedProviderNames()
	fmt.Printf("\nDNS Provider (%s): ", supportedProviders)
	providerInput, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read DNS provider: %w", err)
	}
	providerName := strings.ToLower(strings.TrimSpace(providerInput))

	// Handle alias
	if providerName == "ovh" {
		providerName = "ovhcloud"
	}

	// Use the registry to prompt for credentials
	dnsProvider, err := promptProviderCredentials(reader, providerName, domainName)
	if err != nil {
		return nil, err
	}

	// Detect server IP
	serverIP, err := system.GetServerIP()
	if err != nil {
		fmt.Print("\nCould not detect server IP. Please enter it manually: ")
		ipInput, _ := reader.ReadString('\n')
		serverIP = strings.TrimSpace(ipInput)
	} else {
		fmt.Printf("\nServer IP detected: %s\n", serverIP)
	}

	// Build configuration
	cfg := &config.Config{
		ServerIP:    serverIP,
		DNSProvider: dnsProvider,
		Domains: []config.Domain{
			{
				Name:      domainName,
				IsPrimary: true,
				Subdomains: []config.Subdomain{
					{Name: "adm", Role: "webadmin"},
					{Name: "mx", Role: "mailserver"},
					{Name: "webmail", Role: "webmail"},
					{Name: "www", Role: "website"},
				},
				WebAdmin: &config.WebAdminConfig{
					Username: "admin",
					Password: generateRandomPassword(16),
				},
				EmailAccounts: []config.EmailAccount{
					{
						Username: "contact",
						Password: generateRandomPassword(16),
						FullName: "Contact",
					},
				},
				DomainUser: &config.DomainUserConfig{
					Password:   generateRandomPassword(16),
					SSHKeyMode: "auto",
				},
			},
		},
	}

	return cfg, nil
}

// promptProviderCredentials prompts for provider credentials using the registry
func promptProviderCredentials(reader *bufio.Reader, providerName, domain string) (*config.DNSProviderConfig, error) {
	// Get the prompter from registry
	prompter, err := dnsprovider.GetPrompter(providerName)
	if err != nil {
		return nil, fmt.Errorf("unsupported DNS provider: %s (supported: %s)", providerName, dnsprovider.SupportedProviderNames())
	}

	// Display setup instructions
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("%s Configuration\n", strings.ToUpper(providerName))
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println(prompter.SetupInstructions())
	fmt.Println(strings.Repeat("=", 70))

	fmt.Print("\nPress Enter when you have your credentials ready...")
	reader.ReadString('\n')

	// Collect credential values
	values := make(map[string]string)
	for _, field := range prompter.PromptFields() {
		fmt.Printf("\n%s: ", field.Label)
		value, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", field.Label, err)
		}
		value = strings.TrimSpace(value)
		if field.Required && value == "" {
			return nil, fmt.Errorf("%s is required", field.Label)
		}
		values[field.Key] = value
	}

	// Create credentials and validate
	creds, err := prompter.CreateCredentials(values)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// Create provider and verify domain
	fmt.Println("\nVerifying credentials and domain ownership...")
	provider, err := dnsprovider.CreateProvider(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", providerName, err)
	}

	if err := provider.VerifyDomainManaged(domain); err != nil {
		return nil, fmt.Errorf("domain verification failed: %w", err)
	}

	fmt.Printf("\n  %s credentials verified successfully\n", providerName)

	// Return config using new Credentials format
	return &config.DNSProviderConfig{
		Type:        "external",
		Name:        providerName,
		Credentials: creds.ToYAMLFields(),
	}, nil
}

// promptHostingerCredentials prompts for Hostinger API token and verifies it
// Deprecated: Use promptProviderCredentials instead
func promptHostingerCredentials(reader *bufio.Reader, domain string) (*config.DNSProviderConfig, error) {
	fmt.Print("\nEnter your Hostinger API Token: ")
	apiToken, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read API token: %w", err)
	}
	apiToken = strings.TrimSpace(apiToken)
	if apiToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	// Verify domain ownership
	fmt.Println("\nVerifying domain ownership...")
	provider := dnsprovider.NewHostingerProvider(apiToken)
	if err := provider.VerifyDomainManaged(domain); err != nil {
		return nil, fmt.Errorf("domain verification failed: %w", err)
	}

	return &config.DNSProviderConfig{
		Type:     "external",
		Name:     "hostinger",
		APIToken: apiToken,
	}, nil
}

// promptOVHCloudCredentials prompts for OVHcloud credentials and verifies them
func promptOVHCloudCredentials(reader *bufio.Reader, domain string) (*config.DNSProviderConfig, error) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("OVHcloud API Configuration")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\nTo use OVHcloud DNS, you need to create API credentials.")
	fmt.Println("\n1. Go to: https://manager.eu.ovhcloud.com/#/iam/api-keys/onboarding")
	fmt.Println("\n2. Fill in the following permissions (add each line separately):")
	fmt.Println("   GET    /domain/zone")
	fmt.Println("   GET    /domain/zone/*")
	fmt.Println("   POST   /domain/zone/*")
	fmt.Println("   PUT    /domain/zone/*")
	fmt.Println("   DELETE /domain/zone/*")
	fmt.Println("\n3. Set validity to 'Unlimited' for permanent access")
	fmt.Println("\n4. Copy the three keys generated:")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Print("\nPress Enter when you have your credentials ready...")
	reader.ReadString('\n')

	// Application Key
	fmt.Print("\nApplication Key: ")
	appKey, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read Application Key: %w", err)
	}
	appKey = strings.TrimSpace(appKey)
	if appKey == "" {
		return nil, fmt.Errorf("Application Key is required")
	}

	// Application Secret
	fmt.Print("Application Secret: ")
	appSecret, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read Application Secret: %w", err)
	}
	appSecret = strings.TrimSpace(appSecret)
	if appSecret == "" {
		return nil, fmt.Errorf("Application Secret is required")
	}

	// Consumer Key
	fmt.Print("Consumer Key: ")
	consumerKey, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read Consumer Key: %w", err)
	}
	consumerKey = strings.TrimSpace(consumerKey)
	if consumerKey == "" {
		return nil, fmt.Errorf("Consumer Key is required")
	}

	// Create OVH provider and verify
	fmt.Println("\nVerifying OVHcloud credentials and domain ownership...")
	creds := dnsprovider.OVHCloudCredentials{
		Endpoint:          "ovh-eu",
		ApplicationKey:    appKey,
		ApplicationSecret: appSecret,
		ConsumerKey:       consumerKey,
	}

	provider, err := dnsprovider.NewOVHCloudProvider(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OVHcloud: %w", err)
	}

	if err := provider.VerifyDomainManaged(domain); err != nil {
		return nil, fmt.Errorf("domain verification failed: %w", err)
	}

	// Check existing mail records
	fmt.Println("\nChecking existing DNS records...")
	mailRecords, err := provider.GetMailRecords(domain)
	if err != nil {
		fmt.Printf("  Warning: Could not fetch existing records: %v\n", err)
	} else if len(mailRecords) > 0 {
		fmt.Println("\n" + strings.Repeat("!", 70))
		fmt.Println("WARNING: Existing mail-related DNS records found!")
		fmt.Println(strings.Repeat("!", 70))
		fmt.Println("\nThe following records will be REPLACED during installation:")
		for _, r := range mailRecords {
			name := r.SubDomain
			if name == "" {
				name = "@"
			}
			fmt.Printf("  - %s %s %s\n", name, r.FieldType, r.Target)
		}
		fmt.Println("\nThese records will be backed up in the database before modification.")
		fmt.Print("\nDo you want to continue? (yes/no): ")
		confirm, _ := reader.ReadString('\n')
		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "yes" && confirm != "y" {
			return nil, fmt.Errorf("installation cancelled by user")
		}
	}

	fmt.Println("\n  OVHcloud credentials verified successfully")

	return &config.DNSProviderConfig{
		Type:              "external",
		Name:              "ovhcloud",
		Endpoint:          "ovh-eu",
		ApplicationKey:    appKey,
		ApplicationSecret: appSecret,
		ConsumerKey:       consumerKey,
	}, nil
}

// SaveConfig saves the configuration to a YAML file
func SaveConfig(cfg *config.Config, path string) error {
	// Use yaml.Marshal to create the config file
	data := fmt.Sprintf(`# Hibana Stack Configuration
# Generated interactively

server_ip: %s

dns_provider:
  type: %s
  name: %s
`, cfg.ServerIP, cfg.DNSProvider.Type, cfg.DNSProvider.Name)

	// Add provider credentials using new unified format
	if cfg.DNSProvider.Credentials != nil && len(cfg.DNSProvider.Credentials) > 0 {
		data += "  credentials:\n"
		for key, value := range cfg.DNSProvider.Credentials {
			data += fmt.Sprintf("    %s: %s\n", key, value)
		}
	} else {
		// Fallback to legacy format for backward compatibility
		if cfg.DNSProvider.Name == "hostinger" && cfg.DNSProvider.APIToken != "" {
			data += fmt.Sprintf("  api_token: %s\n", cfg.DNSProvider.APIToken)
		} else if cfg.DNSProvider.Name == "ovhcloud" && cfg.DNSProvider.ApplicationKey != "" {
			data += fmt.Sprintf(`  endpoint: %s
  application_key: %s
  application_secret: %s
  consumer_key: %s
`, cfg.DNSProvider.Endpoint, cfg.DNSProvider.ApplicationKey,
				cfg.DNSProvider.ApplicationSecret, cfg.DNSProvider.ConsumerKey)
		}
	}

	// Add domains
	data += "\ndomains:\n"
	for _, d := range cfg.Domains {
		data += fmt.Sprintf(`  - name: %s
    is_primary: %t
    subdomains:
`, d.Name, d.IsPrimary)
		for _, sub := range d.Subdomains {
			data += fmt.Sprintf("      - name: %s\n        role: %s\n", sub.Name, sub.Role)
		}

		if d.WebAdmin != nil {
			data += fmt.Sprintf(`    webadmin:
      username: %s
      password: %s
`, d.WebAdmin.Username, d.WebAdmin.Password)
		}

		data += "    email_accounts:\n"
		for _, email := range d.EmailAccounts {
			data += fmt.Sprintf(`      - username: %s
        password: %s
        full_name: %s
`, email.Username, email.Password, email.FullName)
		}

		if d.DomainUser != nil {
			data += fmt.Sprintf(`    domain_user:
      password: %s
      ssh_key_mode: %s
`, d.DomainUser.Password, d.DomainUser.SSHKeyMode)
		}
	}

	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("\nConfiguration saved to: %s\n", path)
	return nil
}
