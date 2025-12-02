package installer

import (
	"fmt"
	"net/smtp"
	"os/exec"
	"strings"
	"time"
)

// TestEmailTo tests email sending to a specified address
func (i *Installer) TestEmailTo(testEmail string) error {
	if testEmail == "" {
		fmt.Println("⚠️  No test email address provided, skipping email test")
		return nil
	}

	domain := i.config.PrimaryDomain
	if len(i.config.EmailAccounts) == 0 {
		return fmt.Errorf("no email accounts configured")
	}

	fromAccount := i.config.EmailAccounts[0]
	fromEmail := fmt.Sprintf("%s@%s", fromAccount.Username, domain)

	fmt.Printf("Testing email from %s to %s...\n", fromEmail, testEmail)

	// Test 1: Send email via SMTP
	if err := i.sendTestEmailTo(fromEmail, fromAccount.Password, testEmail); err != nil {
		return fmt.Errorf("failed to send test email: %w", err)
	}

	fmt.Println("✓ Test email sent successfully")

	// Test 2: Check mail queue
	if err := i.checkMailQueue(); err != nil {
		fmt.Printf("⚠️  Warning: mail queue check failed: %v\n", err)
	}

	return nil
}

// sendTestEmailTo sends a test email via SMTP to a specified address
func (i *Installer) sendTestEmailTo(from, password, to string) error {
	domain := i.config.PrimaryDomain
	smtpPort := "587"
	subject := "Hibana Stack - Test Email"
	body := fmt.Sprintf(`This is a test email from your Hibana Stack installation.

Domain: %s
From: %s
Time: %s

If you received this email, your mail server is configured correctly!

--
Hibana Stack
https://github.com/vricosti/hibana-stack
`, domain, from, time.Now().Format(time.RFC1123))

	message := []byte(fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", from, to, subject, body))

	// Try to send via localhost SMTP
	auth := smtp.PlainAuth("", from, password, "localhost")

	// Connect to local SMTP server
	err := smtp.SendMail(
		"localhost:"+smtpPort,
		auth,
		from,
		[]string{to},
		message,
	)

	if err != nil {
		// Try without authentication for local delivery
		err = smtp.SendMail(
			"localhost:25",
			nil,
			from,
			[]string{to},
			message,
		)
	}

	return err
}

// checkMailQueue checks the Postfix mail queue
func (i *Installer) checkMailQueue() error {
	cmd := exec.Command("mailq")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	queueOutput := string(output)

	// Check if queue is empty
	if strings.Contains(queueOutput, "Mail queue is empty") {
		fmt.Println("✓ Mail queue is empty (all messages delivered)")
		return nil
	}

	// Show queue status
	fmt.Printf("Mail queue status:\n%s\n", queueOutput)

	return nil
}
