package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var requiredPackages = []string{
	"postgresql",
	"postgresql-contrib",
	"docker.io",
	"docker-compose",
	"postfix",
	"dovecot-core",
	"dovecot-imapd",
	"dovecot-pop3d",
	"dovecot-lmtpd",
	"dovecot-sieve",
	"dovecot-managesieved",
	"opendkim",
	"opendkim-tools",
	"openssl",
	"mailutils",
	"pdns-server",
	"pdns-backend-pgsql",
	"spamassassin",
	"spamc",
}

// CheckUbuntuVersion verifies the system is running Ubuntu
func CheckUbuntuVersion() error {
	// Check /etc/os-release
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return fmt.Errorf("failed to open /etc/os-release: %w", err)
	}
	defer file.Close()

	var isUbuntu bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			if strings.Contains(line, "ubuntu") {
				isUbuntu = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read /etc/os-release: %w", err)
	}

	if !isUbuntu {
		return fmt.Errorf("this system is not running Ubuntu")
	}

	return nil
}

// CheckRequiredPackages checks which required packages are installed
func CheckRequiredPackages() ([]string, error) {
	var missing []string

	for _, pkg := range requiredPackages {
		installed, err := isPackageInstalled(pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to check package %s: %w", pkg, err)
		}
		if !installed {
			missing = append(missing, pkg)
		}
	}

	return missing, nil
}

// isPackageInstalled checks if a package is installed using dpkg
func isPackageInstalled(pkg string) (bool, error) {
	cmd := exec.Command("dpkg", "-s", pkg)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Package not installed
		if strings.Contains(string(output), "is not installed") {
			return false, nil
		}
		return false, err
	}

	// Check if package is installed
	return strings.Contains(string(output), "Status: install ok installed"), nil
}

// InstallPackages installs missing packages using apt
func InstallPackages(packages []string, domain string) error {
	fmt.Printf("Installing packages: %s\n", strings.Join(packages, ", "))

	// Pre-configure Postfix if it's in the package list
	for _, pkg := range packages {
		if pkg == "postfix" {
			if err := preconfigurePostfix(domain); err != nil {
				return fmt.Errorf("failed to preconfigure Postfix: %w", err)
			}
			break
		}
	}

	// Update package list
	cmd := exec.Command("apt-get", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get update failed: %w", err)
	}

	// Install packages with non-interactive frontend
	args := append([]string{"install", "-y"}, packages...)
	cmd = exec.Command("apt-get", args...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get install failed: %w", err)
	}

	return nil
}

// preconfigurePostfix sets up debconf selections for Postfix
func preconfigurePostfix(domain string) error {
	fmt.Println("Pre-configuring Postfix for non-interactive installation...")

	// Prepare debconf selections
	selections := fmt.Sprintf(`postfix postfix/main_mailer_type select Internet Site
postfix postfix/mailname string %s
`, domain)

	// Use debconf-set-selections
	cmd := exec.Command("debconf-set-selections")
	cmd.Stdin = strings.NewReader(selections)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("debconf-set-selections failed: %w", err)
	}

	return nil
}

// GetServerIP attempts to get the server's public IP address
func GetServerIP() (string, error) {
	// Try to get IP from hostname -I
	cmd := exec.Command("hostname", "-I")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get server IP: %w", err)
	}

	ips := strings.Fields(string(output))
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found")
	}

	// Return first non-loopback IPv4
	for _, ip := range ips {
		if !strings.HasPrefix(ip, "127.") && !strings.Contains(ip, ":") {
			return strings.TrimSpace(ip), nil
		}
	}

	return strings.TrimSpace(ips[0]), nil
}

// GetServerIPv6 attempts to get the server's public IPv6 address
func GetServerIPv6() (string, error) {
	// Try to get IP from hostname -I
	cmd := exec.Command("hostname", "-I")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get server IP: %w", err)
	}

	ips := strings.Fields(string(output))
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found")
	}

	// Return first non-loopback public IPv6 (skip link-local fe80::)
	for _, ip := range ips {
		if strings.Contains(ip, ":") && !strings.HasPrefix(ip, "::1") && !strings.HasPrefix(ip, "fe80:") {
			return strings.TrimSpace(ip), nil
		}
	}

	return "", fmt.Errorf("no public IPv6 address found")
}

// GetServerIPs returns both IPv4 and IPv6 addresses
func GetServerIPs() (ipv4, ipv6 string) {
	ipv4, _ = GetServerIP()
	ipv6, _ = GetServerIPv6()
	return
}

// FixHostsFile ensures the hostname is resolvable in /etc/hosts to avoid sudo delays
func FixHostsFile() error {
	// Get current hostname
	hostnameBytes, err := exec.Command("hostname").Output()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	hostname := strings.TrimSpace(string(hostnameBytes))

	// Get FQDN hostname
	fqdnBytes, err := exec.Command("hostname", "-f").Output()
	fqdn := strings.TrimSpace(string(fqdnBytes))
	if err != nil {
		// If FQDN fails, just use hostname
		fqdn = hostname
	}

	// Read current /etc/hosts
	hostsData, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return fmt.Errorf("failed to read /etc/hosts: %w", err)
	}

	hostsContent := string(hostsData)

	// Check if hostname is already in /etc/hosts
	if strings.Contains(hostsContent, hostname) && strings.Contains(hostsContent, "127.0.1.1") {
		// Already configured
		return nil
	}

	// Check if there's already a 127.0.1.1 entry
	lines := strings.Split(hostsContent, "\n")
	var newLines []string
	found127011 := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "127.0.1.1") {
			// Replace existing 127.0.1.1 line
			newLines = append(newLines, fmt.Sprintf("127.0.1.1\t%s %s", fqdn, hostname))
			found127011 = true
		} else {
			newLines = append(newLines, line)
		}
	}

	if !found127011 {
		// Add new entry if 127.0.1.1 wasn't found
		newLines = append(newLines, "# Added by hibana for hostname resolution")
		newLines = append(newLines, fmt.Sprintf("127.0.1.1\t%s %s", fqdn, hostname))
	}

	// Write updated /etc/hosts
	newHostsContent := strings.Join(newLines, "\n")
	if err := os.WriteFile("/etc/hosts", []byte(newHostsContent), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/hosts: %w", err)
	}

	return nil
}

// DisableSystemdResolved disables systemd-resolved to free port 53
func DisableSystemdResolved() error {
	// Stop systemd-resolved
	cmd := exec.Command("systemctl", "stop", "systemd-resolved")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop systemd-resolved: %w", err)
	}

	// Disable systemd-resolved
	cmd = exec.Command("systemctl", "disable", "systemd-resolved")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable systemd-resolved: %w", err)
	}

	// Remove the symlink if it exists
	if _, err := os.Lstat("/etc/resolv.conf"); err == nil {
		if err := os.Remove("/etc/resolv.conf"); err != nil {
			return fmt.Errorf("failed to remove /etc/resolv.conf: %w", err)
		}
	}

	// Create temporary resolv.conf with public DNS
	resolvConf := `# Temporary DNS configuration - will be updated after PowerDNS setup
nameserver 8.8.8.8
nameserver 8.8.4.4
nameserver 1.1.1.1
`
	if err := os.WriteFile("/etc/resolv.conf", []byte(resolvConf), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/resolv.conf: %w", err)
	}

	return nil
}
