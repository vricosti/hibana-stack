package ansible

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// ExecutePlaybook exécute ansible-playbook
func ExecutePlaybook(workspaceDir string, verbose bool) error {
	playbookPath := filepath.Join(workspaceDir, "playbook.yml")
	inventoryPath := filepath.Join(workspaceDir, "inventory.ini")

	args := []string{
		"-i", inventoryPath,
		playbookPath,
	}

	if verbose {
		args = append(args, "-vvv")
	}

	cmd := exec.Command("ansible-playbook", args...)
	cmd.Dir = workspaceDir

	// Stream output in real-time
	if err := StreamOutput(cmd); err != nil {
		return fmt.Errorf("ansible-playbook execution failed: %w", err)
	}

	return nil
}

// StreamOutput affiche la sortie Ansible en temps réel
func StreamOutput(cmd *exec.Cmd) error {
	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Copy stdout and stderr to os.Stdout and os.Stderr
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// ExecuteResetPlaybook exécute le playbook de reset
func ExecuteResetPlaybook(workspaceDir string, verbose bool) error {
	playbookPath := filepath.Join(workspaceDir, "reset-playbook.yml")
	inventoryPath := filepath.Join(workspaceDir, "inventory.ini")

	args := []string{
		"-i", inventoryPath,
		playbookPath,
	}

	if verbose {
		args = append(args, "-vvv")
	}

	cmd := exec.Command("ansible-playbook", args...)
	cmd.Dir = workspaceDir

	// Stream output in real-time
	if err := StreamOutput(cmd); err != nil {
		return fmt.Errorf("ansible-playbook execution failed: %w", err)
	}

	return nil
}
