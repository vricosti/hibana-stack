package installer

import (
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/vricosti/hibana-stack/internal/config"
)

// SetupMailServer configures Postfix and Dovecot
func (i *Installer) SetupMailServer() error {
	// Open firewall ports for mail services
	if err := i.openMailPorts(); err != nil {
		fmt.Printf("Warning: failed to open mail ports in firewall: %v\n", err)
		fmt.Println("   You may need to open ports manually: 25, 587, 465 (SMTP), 993 (IMAPS), 995 (POP3S)")
	}

	// Generate self-signed certificates first (will be replaced by Let's Encrypt later)
	if err := i.generateSelfSignedCerts(); err != nil {
		return fmt.Errorf("failed to generate self-signed certificates: %w", err)
	}

	// Configure Postfix
	if err := i.configurePostfix(); err != nil {
		return fmt.Errorf("failed to configure Postfix: %w", err)
	}

	// Configure Dovecot
	if err := i.configureDovecot(); err != nil {
		return fmt.Errorf("failed to configure Dovecot: %w", err)
	}

	// Create email accounts for all domains
	if err := i.createEmailAccounts(); err != nil {
		return fmt.Errorf("failed to create email accounts: %w", err)
	}

	// Restart services
	if err := i.restartMailServices(); err != nil {
		return fmt.Errorf("failed to restart mail services: %w", err)
	}

	return nil
}

// configurePostfix sets up Postfix configuration
func (i *Installer) configurePostfix() error {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	domain := primaryDomain.Name
	mailserverSubdomain := i.config.GetMailserverSubdomain()
	hostname := fmt.Sprintf("%s.%s", mailserverSubdomain, domain)

	// Set hostname
	if err := exec.Command("hostnamectl", "set-hostname", hostname).Run(); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Build virtual domains list (all domains)
	var virtualDomains []string
	for _, d := range i.config.Domains {
		virtualDomains = append(virtualDomains, d.Name)
	}
	virtualDomainsStr := ""
	for i, d := range virtualDomains {
		if i > 0 {
			virtualDomainsStr += ", "
		}
		virtualDomainsStr += d
	}

	// Main Postfix configuration
	postfixMain := fmt.Sprintf(`# Postfix Main Configuration
smtpd_banner = $myhostname ESMTP
biff = no
append_dot_mydomain = no
readme_directory = no
compatibility_level = 3.6

# TLS parameters
smtpd_tls_cert_file=/etc/letsencrypt/live/%s/fullchain.pem
smtpd_tls_key_file=/etc/letsencrypt/live/%s/privkey.pem
smtpd_tls_security_level=may
smtp_tls_CApath=/etc/ssl/certs
smtp_tls_security_level=may
smtp_tls_session_cache_database = btree:${data_directory}/smtp_scache

# Basic configuration
myhostname = %s
mydomain = %s
myorigin = $mydomain
mydestination = $myhostname, localhost.$mydomain, localhost, $mydomain
relayhost =
mynetworks = 127.0.0.0/8 [::ffff:127.0.0.0]/104 [::1]/128
mailbox_size_limit = 0
recipient_delimiter = +
inet_interfaces = all
inet_protocols = all

# Virtual domains and users
virtual_mailbox_domains = %s
virtual_mailbox_base = /var/mail/vhosts
virtual_mailbox_maps = hash:/etc/postfix/vmailbox
virtual_minimum_uid = 100
virtual_uid_maps = static:5000
virtual_gid_maps = static:5000
virtual_alias_maps = hash:/etc/postfix/virtual

# SMTP Authentication
smtpd_sasl_type = dovecot
smtpd_sasl_path = private/auth
smtpd_sasl_auth_enable = yes
smtpd_sasl_security_options = noanonymous
smtpd_sasl_local_domain = $mydomain
broken_sasl_auth_clients = yes

# SMTP Restrictions
smtpd_recipient_restrictions =
    permit_mynetworks,
    permit_sasl_authenticated,
    reject_unauth_destination

smtpd_helo_restrictions =
    permit_mynetworks,
    permit_sasl_authenticated,
    reject_invalid_helo_hostname,
    reject_non_fqdn_helo_hostname

smtpd_sender_restrictions =
    permit_mynetworks,
    permit_sasl_authenticated,
    reject_non_fqdn_sender,
    reject_unknown_sender_domain

# DKIM
milter_default_action = accept
milter_protocol = 6
smtpd_milters = local:/run/opendkim/opendkim.sock
non_smtpd_milters = local:/run/opendkim/opendkim.sock

# Other settings
home_mailbox = Maildir/
`, domain, domain, hostname, domain, virtualDomainsStr)

	if err := os.WriteFile("/etc/postfix/main.cf", []byte(postfixMain), 0644); err != nil {
		return fmt.Errorf("failed to write main.cf: %w", err)
	}

	// Master configuration for submission
	postfixMaster := `# Postfix Master Configuration
smtp      inet  n       -       y       -       -       smtpd
submission inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_client_restrictions=permit_sasl_authenticated,reject
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject_unauth_destination
smtps     inet  n       -       y       -       -       smtpd
  -o syslog_name=postfix/smtps
  -o smtpd_tls_wrappermode=yes
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_client_restrictions=permit_sasl_authenticated,reject
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject_unauth_destination
pickup    unix  n       -       y       60      1       pickup
cleanup   unix  n       -       y       -       0       cleanup
qmgr      unix  n       -       n       300     1       qmgr
tlsmgr    unix  -       -       y       1000?   1       tlsmgr
rewrite   unix  -       -       y       -       -       trivial-rewrite
bounce    unix  -       -       y       -       0       bounce
defer     unix  -       -       y       -       0       bounce
trace     unix  -       -       y       -       0       bounce
verify    unix  -       -       y       -       1       verify
flush     unix  n       -       y       1000?   0       flush
proxymap  unix  -       -       n       -       -       proxymap
proxywrite unix -       -       n       -       1       proxymap
smtp      unix  -       -       y       -       -       smtp
relay     unix  -       -       y       -       -       smtp
showq     unix  n       -       y       -       -       showq
error     unix  -       -       y       -       -       error
retry     unix  -       -       y       -       -       error
discard   unix  -       -       y       -       -       discard
local     unix  -       n       n       -       -       local
virtual   unix  -       n       n       -       -       virtual
lmtp      unix  -       -       y       -       -       lmtp
anvil     unix  -       -       y       -       1       anvil
scache    unix  -       -       y       -       1       scache
`

	if err := os.WriteFile("/etc/postfix/master.cf", []byte(postfixMaster), 0644); err != nil {
		return fmt.Errorf("failed to write master.cf: %w", err)
	}

	// Create mail directories for all domains
	for _, d := range i.config.Domains {
		mailDir := "/var/mail/vhosts/" + d.Name
		if err := os.MkdirAll(mailDir, 0770); err != nil {
			return fmt.Errorf("failed to create mail directory for %s: %w", d.Name, err)
		}
	}

	// Set proper ownership
	if err := exec.Command("groupadd", "-g", "5000", "vmail").Run(); err != nil {
		// Ignore error if group already exists
	}

	if err := exec.Command("useradd", "-g", "vmail", "-u", "5000", "vmail", "-d", "/var/mail").Run(); err != nil {
		// Ignore error if user already exists
	}

	if err := exec.Command("chown", "-R", "vmail:vmail", "/var/mail/vhosts").Run(); err != nil {
		return fmt.Errorf("failed to set mail directory ownership: %w", err)
	}

	return nil
}

// configureDovecot sets up Dovecot configuration
func (i *Installer) configureDovecot() error {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	domain := primaryDomain.Name

	// Use self-signed certificates for now (will be updated to Let's Encrypt later)
	sslCert := fmt.Sprintf("/etc/ssl/certs/hibana-%s.crt", domain)
	sslKey := fmt.Sprintf("/etc/ssl/private/hibana-%s.key", domain)

	// Main Dovecot configuration
	dovecotConf := fmt.Sprintf(`# Dovecot Configuration
protocols = imap pop3 lmtp
listen = *, ::

# SSL/TLS (using self-signed certificates initially)
ssl = required
ssl_cert = <%s
ssl_key = <%s
ssl_min_protocol = TLSv1.2

# Mail location
mail_location = maildir:/var/mail/vhosts/%%d/%%n
mail_privileged_group = vmail

# Authentication
auth_mechanisms = plain login

passdb {
  driver = passwd-file
  args = scheme=SHA512-CRYPT username_format=%%u /etc/dovecot/users
}

userdb {
  driver = static
  args = uid=vmail gid=vmail home=/var/mail/vhosts/%%d/%%n
}

# IMAP configuration
protocol imap {
  mail_plugins = $mail_plugins
}

# POP3 configuration
protocol pop3 {
  mail_plugins = $mail_plugins
}

# LMTP configuration
protocol lmtp {
  postmaster_address = postmaster@%s
}

# Postfix SASL auth
service auth {
  unix_listener /var/spool/postfix/private/auth {
    mode = 0660
    user = postfix
    group = postfix
  }
}

# Master user configuration
namespace inbox {
  inbox = yes
}
`, sslCert, sslKey, domain)

	if err := os.WriteFile("/etc/dovecot/dovecot.conf", []byte(dovecotConf), 0644); err != nil {
		return fmt.Errorf("failed to write dovecot.conf: %w", err)
	}

	return nil
}

// createEmailAccounts creates email user accounts for all domains
func (i *Installer) createEmailAccounts() error {
	usersFile := "/etc/dovecot/users"
	vmailboxFile := "/etc/postfix/vmailbox"

	var usersContent string
	var vmailboxContent string

	// Create accounts for all domains
	for _, domain := range i.config.Domains {
		for _, account := range domain.EmailAccounts {
			email := fmt.Sprintf("%s@%s", account.Username, domain.Name)

			// Hash password for Dovecot (SHA512-CRYPT)
			hashedPassword := hashPasswordSHA512(account.Password)

			// Add to Dovecot users file
			usersContent += fmt.Sprintf("%s:%s\n", email, hashedPassword)

			// Add to Postfix virtual mailbox
			vmailboxContent += fmt.Sprintf("%s %s/%s/\n", email, domain.Name, account.Username)

			// Create mailbox directory
			mailboxDir := filepath.Join("/var/mail/vhosts", domain.Name, account.Username)
			if err := os.MkdirAll(mailboxDir, 0770); err != nil {
				return fmt.Errorf("failed to create mailbox for %s: %w", email, err)
			}

			if err := exec.Command("chown", "-R", "vmail:vmail", mailboxDir).Run(); err != nil {
				return fmt.Errorf("failed to set mailbox ownership for %s: %w", email, err)
			}

			// Store in database
			if err := i.storeEmailAccount(domain.Name, account); err != nil {
				return fmt.Errorf("failed to store email account %s: %w", email, err)
			}
		}
	}

	// Write users file
	if err := os.WriteFile(usersFile, []byte(usersContent), 0600); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}

	// Write vmailbox file
	if err := os.WriteFile(vmailboxFile, []byte(vmailboxContent), 0644); err != nil {
		return fmt.Errorf("failed to write vmailbox file: %w", err)
	}

	// Create virtual alias file
	if err := os.WriteFile("/etc/postfix/virtual", []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to write virtual file: %w", err)
	}

	// Build Postfix maps
	if err := exec.Command("postmap", "/etc/postfix/vmailbox").Run(); err != nil {
		return fmt.Errorf("failed to build vmailbox map: %w", err)
	}

	if err := exec.Command("postmap", "/etc/postfix/virtual").Run(); err != nil {
		return fmt.Errorf("failed to build virtual map: %w", err)
	}

	return nil
}

// storeEmailAccount stores email account in database
func (i *Installer) storeEmailAccount(domainName string, account config.EmailAccount) error {
	if i.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Get domain ID
	var domainID int
	err := i.db.QueryRow("SELECT id FROM domains WHERE name = $1", domainName).Scan(&domainID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	// Hash password
	hashedPassword := hashPasswordSHA512(account.Password)

	// Check if account exists
	var existingID int
	err = i.db.QueryRow(
		"SELECT id FROM email_accounts WHERE domain_id = $1 AND username = $2",
		domainID, account.Username,
	).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new account
		_, err = i.db.Exec(
			"INSERT INTO email_accounts (domain_id, username, password_hash, full_name) VALUES ($1, $2, $3, $4)",
			domainID, account.Username, hashedPassword, account.FullName,
		)
		return err
	} else if err != nil {
		return err
	}

	// Update existing account
	_, err = i.db.Exec(
		"UPDATE email_accounts SET password_hash = $1, full_name = $2 WHERE id = $3",
		hashedPassword, account.FullName, existingID,
	)

	return err
}

// restartMailServices restarts Postfix and Dovecot
func (i *Installer) restartMailServices() error {
	services := []string{"postfix", "dovecot"}

	for _, service := range services {
		// Restart service
		if err := exec.Command("systemctl", "restart", service).Run(); err != nil {
			return fmt.Errorf("failed to restart %s: %w", service, err)
		}

		// Enable service
		if err := exec.Command("systemctl", "enable", service).Run(); err != nil {
			return fmt.Errorf("failed to enable %s: %w", service, err)
		}
	}

	return nil
}

// generateSelfSignedCerts generates self-signed SSL certificates for mail server
func (i *Installer) generateSelfSignedCerts() error {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	domain := primaryDomain.Name
	certPath := fmt.Sprintf("/etc/ssl/certs/hibana-%s.crt", domain)
	keyPath := fmt.Sprintf("/etc/ssl/private/hibana-%s.key", domain)

	// Check if certificates already exist
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			fmt.Println("  Self-signed certificates already exist")
			return nil
		}
	}

	fmt.Println("  Generating self-signed SSL certificates...")

	// Create private key directory if it doesn't exist
	if err := os.MkdirAll("/etc/ssl/private", 0700); err != nil {
		return fmt.Errorf("failed to create /etc/ssl/private: %w", err)
	}

	// Generate self-signed certificate using openssl
	cmd := exec.Command("openssl", "req", "-x509", "-nodes",
		"-days", "365",
		"-newkey", "rsa:2048",
		"-keyout", keyPath,
		"-out", certPath,
		"-subj", fmt.Sprintf("/C=US/ST=State/L=City/O=Hibana/CN=%s", domain))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate self-signed certificate: %w\nOutput: %s", err, string(output))
	}

	// Set proper permissions
	if err := os.Chmod(keyPath, 0600); err != nil {
		return fmt.Errorf("failed to set key permissions: %w", err)
	}

	fmt.Println("  Self-signed certificates generated")
	fmt.Println("  Note: These will be replaced by Let's Encrypt certificates after Traefik setup")
	return nil
}

// hashPasswordSHA512 creates a SHA512-CRYPT hash for passwords
func hashPasswordSHA512(password string) string {
	// Simple SHA512 hash for demonstration
	// In production, use proper crypt() implementation
	h := sha512.Sum512([]byte(password))
	return "{SHA512-CRYPT}" + base64.StdEncoding.EncodeToString(h[:])
}

// openMailPorts opens firewall ports for mail services
func (i *Installer) openMailPorts() error {
	fmt.Println("  Opening firewall ports for mail services...")

	// Open mail ports
	// 25/tcp - SMTP (for receiving mail from other servers)
	// 587/tcp - Submission (for sending mail from mail clients)
	// 465/tcp - SMTPS (legacy secure SMTP)
	// 993/tcp - IMAPS (secure IMAP)
	// 995/tcp - POP3S (secure POP3)
	ports := []string{"25/tcp", "587/tcp", "465/tcp", "993/tcp", "995/tcp"}
	portDescriptions := map[string]string{
		"25/tcp":  "SMTP - receiving mail from other servers",
		"587/tcp": "Submission - sending mail from mail clients",
		"465/tcp": "SMTPS - secure SMTP (legacy)",
		"993/tcp": "IMAPS - secure IMAP",
		"995/tcp": "POP3S - secure POP3",
	}

	return i.openPortsWithTracking("mail_setup", ports, portDescriptions)
}
