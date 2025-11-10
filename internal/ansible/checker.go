package ansible

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CheckAnsibleInstalled vérifie qu'Ansible est installé
func CheckAnsibleInstalled() error {
	cmd := exec.Command("ansible-playbook", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ansible-playbook is not installed or not in PATH")
	}
	return nil
}

// GetAnsibleVersion récupère la version d'Ansible
func GetAnsibleVersion() (string, error) {
	cmd := exec.Command("ansible-playbook", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get ansible version: %w", err)
	}

	// Parse version from output (first line usually contains version)
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		// Output format: "ansible-playbook [core 2.XX.X]"
		parts := strings.Fields(lines[0])
		if len(parts) >= 2 {
			return parts[1], nil
		}
	}

	return "unknown", nil
}

// InstallInstructions retourne les instructions d'installation
func InstallInstructions() string {
	return `
Ansible is required to run Hibana Stack.

To install Ansible on Ubuntu:

  sudo apt update
  sudo apt install -y ansible

For other systems, visit: https://docs.ansible.com/ansible/latest/installation_guide/index.html
`
}

// InstallAnsible installe Ansible sur Ubuntu
func InstallAnsible() error {
	fmt.Println("📦 Installing Ansible...")

	// Update apt cache
	fmt.Println("  Updating package lists...")
	cmd := exec.Command("apt", "update")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update apt cache: %w\n%s", err, string(output))
	}

	// Install ansible
	fmt.Println("  Installing ansible package...")
	cmd = exec.Command("apt", "install", "-y", "ansible")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install ansible: %w", err)
	}

	fmt.Println("✓ Ansible installed successfully")
	return nil
}
