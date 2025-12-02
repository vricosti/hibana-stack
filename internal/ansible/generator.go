package ansible

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/vricosti/hibana-stack/internal/config"
)

// CreateWorkspace créer un répertoire de travail temporaire
func CreateWorkspace() (string, error) {
	workspaceDir, err := os.MkdirTemp("", "hibana-ansible-*")
	if err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}
	return workspaceDir, nil
}

// GenerateInventory génère inventory.ini depuis le template
func GenerateInventory(cfg *config.Config, workspaceDir string) error {
	inventoryContent := `[hibana_server]
localhost ansible_connection=local

[hibana_server:vars]
ansible_python_interpreter=/usr/bin/python3
`

	inventoryPath := filepath.Join(workspaceDir, "inventory.ini")
	if err := os.WriteFile(inventoryPath, []byte(inventoryContent), 0644); err != nil {
		return fmt.Errorf("failed to write inventory: %w", err)
	}

	return nil
}

// GenerateGroupVars génère group_vars/all.yml depuis le template
func GenerateGroupVars(cfg *config.Config, workspaceDir string) error {
	// Create group_vars directory
	groupVarsDir := filepath.Join(workspaceDir, "group_vars")
	if err := os.MkdirAll(groupVarsDir, 0755); err != nil {
		return fmt.Errorf("failed to create group_vars directory: %w", err)
	}

	// Template for group_vars/all.yml
	tmpl := `---
# Generated from hibana-config.yaml
primary_domain: {{ .PrimaryDomain }}
server_ip: {{ .ServerIP }}

{{- if .SystemUsers }}
system_users:
{{- range .SystemUsers }}
  - username: {{ .Username }}
    password: "{{ .Password }}"
    name: "{{ .Name }}"
    sudoers: {{ .Sudoers }}
    {{- if .SSHPubKey }}
    ssh_pub_key: "{{ .SSHPubKey }}"
    {{- else }}
    ssh_pub_key: ""
    {{- end }}
{{- end }}
{{- end }}

{{- if .DomainUser }}
domain_user:
  password: "{{ .DomainUser.Password }}"
  ssh_key_mode: "{{ .DomainUser.SSHKeyMode }}"
  {{- if .DomainUser.SSHPublicKey }}
  ssh_public_key: "{{ .DomainUser.SSHPublicKey }}"
  {{- end }}
{{- end }}

subdomains:
{{- range .Subdomains }}
  - name: {{ .Name }}
    role: {{ .Role }}
{{- end }}

email_accounts:
{{- range .EmailAccounts }}
  - username: {{ .Username }}
    password: "{{ .Password }}"
    full_name: "{{ .FullName }}"
{{- end }}

{{- if .WebAdmin }}
webadmin:
  username: "{{ .WebAdmin.Username }}"
  password: "{{ .WebAdmin.Password }}"
{{- end }}

{{- if .DomainRedirects }}
domain_redirects:
{{- range .DomainRedirects }}
  - from: {{ .From }}
    {{- if .To }}
    to: "{{ .To }}"
    {{- end }}
    permanent: {{ .Permanent }}
{{- end }}
{{- end }}
`

	t, err := template.New("groupvars").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("failed to parse group_vars template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, cfg); err != nil {
		return fmt.Errorf("failed to execute group_vars template: %w", err)
	}

	groupVarsPath := filepath.Join(groupVarsDir, "all.yml")
	if err := os.WriteFile(groupVarsPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write group_vars/all.yml: %w", err)
	}

	return nil
}

// CopyRoles copie tous les roles dans le workspace
func CopyRoles(workspaceDir string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	sourcePath := filepath.Join(cwd, "ansible", "roles")
	destPath := filepath.Join(workspaceDir, "roles")

	// Copy roles directory
	if err := copyDir(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy roles: %w", err)
	}

	return nil
}

// CopyPlaybook copie le playbook principal
func CopyPlaybook(workspaceDir string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	sourcePath := filepath.Join(cwd, "ansible", "playbook.yml")
	destPath := filepath.Join(workspaceDir, "playbook.yml")

	// Copy playbook
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read playbook: %w", err)
	}

	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write playbook: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory tree
func copyDir(src string, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read directory contents
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectories
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy files
			content, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, content, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyResetPlaybook copie le playbook de reset
func CopyResetPlaybook(workspaceDir string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	sourcePath := filepath.Join(cwd, "ansible", "reset-playbook.yml")
	destPath := filepath.Join(workspaceDir, "reset-playbook.yml")

	// Copy reset playbook
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read reset playbook: %w", err)
	}

	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write reset playbook: %w", err)
	}

	return nil
}
