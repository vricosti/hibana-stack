package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CredentialsSummary stores all credentials generated during installation
type CredentialsSummary struct {
	Domain            string
	ServerIP          string
	DatabasePasswords map[string]string
	EmailAccounts     []EmailAccountCreds
	GeneratedAt       time.Time
}

// EmailAccountCreds stores email account credentials
type EmailAccountCreds struct {
	Email    string
	Password string
	FullName string
}

// GetCredentialsSummary collects all credentials from the installation
func (i *Installer) GetCredentialsSummary() (*CredentialsSummary, error) {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return nil, fmt.Errorf("no primary domain configured")
	}

	summary := &CredentialsSummary{
		Domain:            primaryDomain.Name,
		ServerIP:          i.config.ServerIP,
		DatabasePasswords: make(map[string]string),
		GeneratedAt:       time.Now(),
	}

	// Collect database passwords
	dbUsers := []string{"hibana", "pdns", "pdns-api"}
	for _, user := range dbUsers {
		password := i.getOrGeneratePassword(user)
		summary.DatabasePasswords[user] = password
	}

	// Collect email account credentials from all domains
	for _, domain := range i.config.Domains {
		for _, account := range domain.EmailAccounts {
			summary.EmailAccounts = append(summary.EmailAccounts, EmailAccountCreds{
				Email:    fmt.Sprintf("%s@%s", account.Username, domain.Name),
				Password: account.Password,
				FullName: account.FullName,
			})
		}
	}

	return summary, nil
}

// SaveCredentialsSummary saves credentials to a file
func (i *Installer) SaveCredentialsSummary(summary *CredentialsSummary) error {
	credentialsDir := "/etc/hibana"
	credentialsFile := filepath.Join(credentialsDir, "credentials-summary.txt")

	content := fmt.Sprintf(`# Hibana Stack - Credentials Summary
# Generated: %s
# IMPORTANT: Keep this file secure and delete after noting the credentials

================================================================================
                           HIBANA STACK CREDENTIALS
================================================================================

Domain: %s
Server IP: %s

--------------------------------------------------------------------------------
DATABASE CREDENTIALS
--------------------------------------------------------------------------------

PostgreSQL databases have been created with the following credentials:

1. Hibana Database
   Database: hibana
   Username: hibana
   Password: %s

   Connection: psql -U hibana -d hibana -h localhost

2. PowerDNS Database
   Database: powerdns
   Username: pdns
   Password: %s

   Connection: psql -U pdns -d powerdns -h localhost

3. PowerDNS API
   API Key: %s

   Access: http://localhost:8081 (local only)

--------------------------------------------------------------------------------
EMAIL ACCOUNTS
--------------------------------------------------------------------------------

The following email accounts have been created:

`,
		summary.GeneratedAt.Format(time.RFC3339),
		summary.Domain,
		summary.ServerIP,
		summary.DatabasePasswords["hibana"],
		summary.DatabasePasswords["pdns"],
		summary.DatabasePasswords["pdns-api"],
	)

	// Add email accounts
	for i, account := range summary.EmailAccounts {
		content += fmt.Sprintf(`%d. %s
   Email: %s
   Password: %s
   Full Name: %s

   IMAP Server: mx.%s:993 (SSL/TLS)
   SMTP Server: mx.%s:587 (STARTTLS)
   Webmail: https://webmail.%s

`,
			i+1,
			account.Email,
			account.Email,
			account.Password,
			account.FullName,
			summary.Domain,
			summary.Domain,
			summary.Domain,
		)
	}

	content += `
--------------------------------------------------------------------------------
ACCESS INFORMATION
--------------------------------------------------------------------------------

Web Interfaces:
  - Main Website:  https://www.` + summary.Domain + `
  - Admin Panel:   https://adm.` + summary.Domain + ` (Phase 2)
  - Webmail:       https://webmail.` + summary.Domain + `

Mail Server:
  - IMAP:          mx.` + summary.Domain + `:993 (SSL/TLS)
  - SMTP:          mx.` + summary.Domain + `:587 (STARTTLS)

DNS Server:
  - Nameserver:    ns1.` + summary.Domain + ` (` + summary.ServerIP + `)

--------------------------------------------------------------------------------
PASSWORD FILES LOCATION
--------------------------------------------------------------------------------

All passwords are also stored individually in:
  /etc/hibana/passwords/

These files are only readable by root (permissions: 0600)

To retrieve a password manually:
  sudo cat /etc/hibana/passwords/<username>

--------------------------------------------------------------------------------
SECURITY RECOMMENDATIONS
--------------------------------------------------------------------------------

1. Store these credentials in a secure password manager
2. Delete this file after noting the credentials:
   sudo rm /etc/hibana/credentials-summary.txt

3. Restrict access to /etc/hibana/passwords/:
   sudo chmod 700 /etc/hibana/passwords/

4. Enable firewall:
   sudo ufw enable
   sudo ufw allow 22,25,80,443,587,993,53/tcp
   sudo ufw allow 53/udp

5. Install fail2ban for brute force protection:
   sudo apt install fail2ban

6. Regularly update the system:
   sudo apt update && sudo apt upgrade -y

================================================================================
`

	// Write to file with restricted permissions
	if err := os.WriteFile(credentialsFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write credentials summary: %w", err)
	}

	return nil
}

// DisplayCredentialsSummary prints credentials to console
func (i *Installer) DisplayCredentialsSummary(summary *CredentialsSummary) {
	fmt.Println("\n" + string(make([]byte, 80)))
	fmt.Println("CREDENTIALS SUMMARY")
	fmt.Println(string(make([]byte, 80)))

	fmt.Println("\nDatabase Credentials:")
	fmt.Printf("   Hibana DB:    User: hibana    Password: %s\n", maskPassword(summary.DatabasePasswords["hibana"]))
	fmt.Printf("   PowerDNS DB:  User: pdns      Password: %s\n", maskPassword(summary.DatabasePasswords["pdns"]))
	fmt.Printf("   PowerDNS API: API Key: %s\n", maskPassword(summary.DatabasePasswords["pdns-api"]))

	fmt.Println("\nEmail Accounts:")
	for i, account := range summary.EmailAccounts {
		fmt.Printf("   %d. %s\n", i+1, account.Email)
		fmt.Printf("      Password: %s\n", maskPassword(account.Password))
	}

	fmt.Println("\nCredentials saved to:")
	fmt.Println("   /etc/hibana/credentials-summary.txt")
	fmt.Println("   /etc/hibana/passwords/ (individual files)")

	fmt.Println("\nIMPORTANT:")
	fmt.Println("   - Keep these credentials secure")
	fmt.Println("   - Store in a password manager")
	fmt.Println("   - Delete credentials-summary.txt after noting them")

	fmt.Println("\nTo view credentials later:")
	fmt.Println("   sudo cat /etc/hibana/credentials-summary.txt")
	fmt.Println("   sudo cat /etc/hibana/passwords/<username>")

	fmt.Println()
}

// maskPassword masks a password for display (shows first 4 and last 2 chars)
func maskPassword(password string) string {
	if len(password) <= 6 {
		return "******"
	}

	visible := 4
	if len(password) < 10 {
		visible = 2
	}

	return password[:visible] + "********" + password[len(password)-2:]
}
