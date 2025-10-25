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

// CheckUbuntuVersion verifies the system is running Ubuntu 24.04
func CheckUbuntuVersion() error {
	// Check /etc/os-release
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return fmt.Errorf("failed to open /etc/os-release: %w", err)
	}
	defer file.Close()

	var isUbuntu bool
	var version string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			if strings.Contains(line, "ubuntu") {
				isUbuntu = true
			}
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read /etc/os-release: %w", err)
	}

	if !isUbuntu {
		return fmt.Errorf("this system is not running Ubuntu")
	}

	if version != "24.04" {
		return fmt.Errorf("Ubuntu version %s detected, but 24.04 is required", version)
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

	// Return first non-loopback IP
	for _, ip := range ips {
		if !strings.HasPrefix(ip, "127.") && !strings.HasPrefix(ip, "::1") {
			return strings.TrimSpace(ip), nil
		}
	}

	return strings.TrimSpace(ips[0]), nil
}
