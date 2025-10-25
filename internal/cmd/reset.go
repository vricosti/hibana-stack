package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
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
	fmt.Println("🗑️  Starting cleanup process...")
	fmt.Println()

	// Stop services first
	fmt.Println("⏹️  Stopping services...")
	services := []string{
		"postfix",
		"dovecot",
		"opendkim",
		"pdns",
		"postgresql",
		"spamassassin",
		"docker",
	}

	for _, service := range services {
		cmd := exec.Command("systemctl", "stop", service)
		_ = cmd.Run() // Ignore errors if service doesn't exist
	}

	// Re-enable systemd-resolved
	fmt.Println("\n🔄 Re-enabling systemd-resolved...")
	exec.Command("chattr", "-i", "/etc/resolv.conf").Run() // Remove immutable flag
	exec.Command("systemctl", "enable", "systemd-resolved").Run()
	exec.Command("systemctl", "start", "systemd-resolved").Run()

	// Remove Docker containers and images
	fmt.Println("\n🐳 Removing Docker containers and images...")
	exec.Command("docker", "stop", "$(docker ps -aq)").Run()
	exec.Command("docker", "rm", "$(docker ps -aq)").Run()
	exec.Command("docker", "rmi", "$(docker images -q)").Run()

	// Purge packages
	fmt.Println("\n📦 Removing packages...")
	packages := []string{
		"postgresql*",
		"pdns-server",
		"pdns-backend-pgsql",
		"postfix*",
		"dovecot-core",
		"dovecot-imapd",
		"dovecot-pop3d",
		"dovecot-sieve",
		"dovecot-managesieved",
		"opendkim",
		"opendkim-tools",
		"mailutils",
		"spamassassin",
		"spamc",
		"docker.io",
		"docker-compose",
	}

	purgeArgs := append([]string{"purge", "-y"}, packages...)
	purgeCmd := exec.Command("apt-get", purgeArgs...)
	purgeCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	purgeCmd.Stdout = os.Stdout
	purgeCmd.Stderr = os.Stderr

	if err := purgeCmd.Run(); err != nil {
		fmt.Printf("⚠️  Warning: some packages may not have been removed: %v\n", err)
	}

	// Autoremove
	fmt.Println("\n🧹 Removing unused dependencies...")
	autoremoveCmd := exec.Command("apt-get", "autoremove", "-y")
	autoremoveCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	autoremoveCmd.Stdout = os.Stdout
	autoremoveCmd.Stderr = os.Stderr
	autoremoveCmd.Run()

	// Clean apt cache
	fmt.Println("\n🗑️  Cleaning apt cache...")
	exec.Command("apt-get", "clean").Run()

	// Remove configuration directories
	fmt.Println("\n📁 Removing configuration directories...")
	dirsToRemove := []string{
		"/etc/postfix",
		"/etc/dovecot",
		"/etc/opendkim",
		"/etc/powerdns",
		"/var/lib/postgresql",
		"/var/spool/mail",
		"/var/mail",
		"/etc/traefik",
		"/opt/traefik",
		"/var/lib/docker",
		"/etc/spamassassin",
	}

	for _, dir := range dirsToRemove {
		if _, err := os.Stat(dir); err == nil {
			fmt.Printf("  Removing %s...\n", dir)
			if err := os.RemoveAll(dir); err != nil {
				fmt.Printf("  ⚠️  Failed to remove %s: %v\n", dir, err)
			}
		}
	}

	// Remove mail users
	fmt.Println("\n👤 Removing mail system users...")
	usersToRemove := []string{"vmail", "opendkim"}
	for _, user := range usersToRemove {
		exec.Command("userdel", "-r", user).Run() // Ignore errors
	}

	fmt.Println()
	fmt.Println("✓ Cleanup complete!")
	fmt.Println()
	fmt.Println("The system has been reset. You can now run 'sudo hibana init' for a fresh installation.")

	return nil
}
