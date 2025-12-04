package installer

import (
	"fmt"
	"os"
	"os/exec"
)

// SetupSpamAssassin configures SpamAssassin for spam filtering
func (i *Installer) SetupSpamAssassin() error {
	// Create SpamAssassin user if doesn't exist
	if err := i.createSpamAssassinUser(); err != nil {
		return fmt.Errorf("failed to create SpamAssassin user: %w", err)
	}

	// Configure SpamAssassin
	if err := i.configureSpamAssassin(); err != nil {
		return fmt.Errorf("failed to configure SpamAssassin: %w", err)
	}

	// Update SpamAssassin rules
	if err := i.updateSpamAssassinRules(); err != nil {
		return fmt.Errorf("failed to update SpamAssassin rules: %w", err)
	}

	// Configure Postfix to use SpamAssassin
	if err := i.configurePostfixSpamAssassin(); err != nil {
		return fmt.Errorf("failed to configure Postfix with SpamAssassin: %w", err)
	}

	// Configure Dovecot Sieve for spam filtering
	if err := i.configureDovecotSieve(); err != nil {
		return fmt.Errorf("failed to configure Dovecot Sieve: %w", err)
	}

	// Start SpamAssassin service
	if err := i.startSpamAssassin(); err != nil {
		return fmt.Errorf("failed to start SpamAssassin: %w", err)
	}

	return nil
}

// createSpamAssassinUser creates the spamd user
func (i *Installer) createSpamAssassinUser() error {
	// Check if user exists
	cmd := exec.Command("id", "spamd")
	if err := cmd.Run(); err == nil {
		// User already exists
		return nil
	}

	// Create system user for SpamAssassin
	cmd = exec.Command("useradd", "-r", "-d", "/var/lib/spamassassin", "-s", "/bin/false", "spamd")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Ignore error if user already exists
		if !containsString(string(output), "already exists") {
			return fmt.Errorf("failed to create spamd user: %w\n%s", err, output)
		}
	}

	return nil
}

// configureSpamAssassin creates SpamAssassin configuration
func (i *Installer) configureSpamAssassin() error {
	config := `# SpamAssassin Configuration

# Rewrite message Subject
rewrite_header Subject [***SPAM***]

# Required score to mark as spam
required_score 5.0

# Use Bayes filter
use_bayes 1
bayes_auto_learn 1

# Store bayes data in SQL (future enhancement)
# bayes_store_module Mail::SpamAssassin::BayesStore::PgSQL

# Network tests
skip_rbl_checks 0
use_razor2 0
use_dcc 0
use_pyzor 0

# Report settings
report_safe 0

# Trusted networks (local network)
trusted_networks 127.0.0.0/8

# Custom scores
score BAYES_00 -1.9
score BAYES_05 -1.9
score BAYES_20 -0.9
score BAYES_40 -0.5
score BAYES_50 0.5
score BAYES_60 1.0
score BAYES_80 2.5
score BAYES_95 3.5
score BAYES_99 4.5
`

	if err := os.WriteFile("/etc/spamassassin/local.cf", []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write SpamAssassin config: %w", err)
	}

	// Enable SpamAssassin daemon
	spamassassinDefault := `# SpamAssassin daemon configuration
ENABLED=1
SPAMD_HOME="/var/lib/spamassassin/"
OPTIONS="--create-prefs --max-children 5 --helper-home-dir --username spamd -s /var/log/spamd.log"
PIDFILE="/var/run/spamd.pid"
CRON=1
`

	if err := os.WriteFile("/etc/default/spamassassin", []byte(spamassassinDefault), 0644); err != nil {
		return fmt.Errorf("failed to write SpamAssassin default config: %w", err)
	}

	return nil
}

// updateSpamAssassinRules updates SpamAssassin rules
func (i *Installer) updateSpamAssassinRules() error {
	cmd := exec.Command("sa-update")
	output, err := cmd.CombinedOutput()

	// sa-update returns error if rules are already up to date
	if err != nil && !containsString(string(output), "already up to date") {
		// Log but don't fail - rules can be updated later
		fmt.Printf("Warning: sa-update returned: %v\n%s\n", err, output)
	}

	return nil
}

// configurePostfixSpamAssassin configures Postfix to filter through SpamAssassin
func (i *Installer) configurePostfixSpamAssassin() error {
	// Read current main.cf
	mainCf, err := os.ReadFile("/etc/postfix/main.cf")
	if err != nil {
		return fmt.Errorf("failed to read main.cf: %w", err)
	}

	mainCfStr := string(mainCf)

	// Check if SpamAssassin content filter is already configured
	if containsString(mainCfStr, "content_filter") && containsString(mainCfStr, "spamassassin") {
		// Already configured
		return nil
	}

	// Append SpamAssassin content filter configuration
	spamassassinConfig := `
# SpamAssassin content filter
content_filter = spamassassin
`

	f, err := os.OpenFile("/etc/postfix/main.cf", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open main.cf: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(spamassassinConfig); err != nil {
		return fmt.Errorf("failed to append to main.cf: %w", err)
	}

	// Update master.cf to add SpamAssassin filter
	masterCf, err := os.ReadFile("/etc/postfix/master.cf")
	if err != nil {
		return fmt.Errorf("failed to read master.cf: %w", err)
	}

	masterCfStr := string(masterCf)

	// Check if already configured
	if containsString(masterCfStr, "spamassassin unix") {
		return nil
	}

	// Append SpamAssassin service definition
	spamassassinService := `
# SpamAssassin filter
spamassassin unix -     n       n       -       -       pipe
  user=spamd argv=/usr/bin/spamc -f -e /usr/sbin/sendmail -oi -f ${sender} ${recipient}
`

	f, err = os.OpenFile("/etc/postfix/master.cf", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open master.cf: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(spamassassinService); err != nil {
		return fmt.Errorf("failed to append to master.cf: %w", err)
	}

	return nil
}

// configureDovecotSieve configures Dovecot Sieve for spam filtering
func (i *Installer) configureDovecotSieve() error {
	primaryDomain := i.config.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	// Update Dovecot configuration to enable Sieve
	sieveConfig := `
# Sieve filtering
protocol lmtp {
  mail_plugins = $mail_plugins sieve
}

plugin {
  sieve = file:~/sieve;active=~/.dovecot.sieve
  sieve_default = /var/lib/dovecot/sieve/default.sieve
  sieve_default_name = main
}
`

	// Check if Sieve is already configured
	dovecotConf, err := os.ReadFile("/etc/dovecot/dovecot.conf")
	if err != nil {
		return fmt.Errorf("failed to read dovecot.conf: %w", err)
	}

	if containsString(string(dovecotConf), "sieve") {
		// Already configured
		return nil
	}

	// Append Sieve configuration
	f, err := os.OpenFile("/etc/dovecot/dovecot.conf", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open dovecot.conf: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(sieveConfig); err != nil {
		return fmt.Errorf("failed to append to dovecot.conf: %w", err)
	}

	// Create default Sieve script directory
	sieveDir := "/var/lib/dovecot/sieve"
	if err := os.MkdirAll(sieveDir, 0755); err != nil {
		return fmt.Errorf("failed to create sieve directory: %w", err)
	}

	// Create default Sieve script to move spam to Junk folder
	defaultSieve := `require ["fileinto", "mailbox"];

# Move spam to Junk folder
if header :contains "X-Spam-Flag" "YES" {
  fileinto :create "Junk";
  stop;
}

# Move high-score spam directly
if header :contains "X-Spam-Level" "**********" {
  fileinto :create "Junk";
  stop;
}
`

	sievePath := sieveDir + "/default.sieve"
	if err := os.WriteFile(sievePath, []byte(defaultSieve), 0644); err != nil {
		return fmt.Errorf("failed to write default sieve script: %w", err)
	}

	// Compile the Sieve script
	cmd := exec.Command("sievec", sievePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to compile sieve script: %w", err)
	}

	// Create per-user Sieve scripts for each email account in all domains
	for _, domain := range i.config.Domains {
		for _, account := range domain.EmailAccounts {
			userSieveDir := fmt.Sprintf("/var/mail/vhosts/%s/%s/sieve", domain.Name, account.Username)
			if err := os.MkdirAll(userSieveDir, 0770); err != nil {
				continue // Skip if fails
			}

			userSievePath := userSieveDir + "/spam-filter.sieve"
			if err := os.WriteFile(userSievePath, []byte(defaultSieve), 0644); err != nil {
				continue
			}

			// Compile user script
			exec.Command("sievec", userSievePath).Run()

			// Set proper ownership
			exec.Command("chown", "-R", "vmail:vmail", userSieveDir).Run()
		}
	}

	return nil
}

// startSpamAssassin starts and enables SpamAssassin service
func (i *Installer) startSpamAssassin() error {
	// Start SpamAssassin daemon (service name is spamd, not spamassassin)
	cmd := exec.Command("systemctl", "restart", "spamd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart spamd: %w", err)
	}

	// Enable SpamAssassin
	cmd = exec.Command("systemctl", "enable", "spamd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable spamd: %w", err)
	}

	// Verify service is running
	cmd = exec.Command("systemctl", "is-active", "spamd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("spamd service is not active after restart")
	}

	return nil
}
