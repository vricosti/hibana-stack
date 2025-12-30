package ansible

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/vricosti/hibana-stack/internal/config"
	"golang.org/x/net/idna"
)

// CreateWorkspace creates a temporary working directory
func CreateWorkspace() (string, error) {
	workspaceDir, err := os.MkdirTemp("", "hibana-ansible-*")
	if err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}
	return workspaceDir, nil
}

// GenerateInventory generates inventory.ini from template
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

// GenerateGroupVars generates group_vars/all.yml from template
func GenerateGroupVars(cfg *config.Config, workspaceDir string) error {
	// Create group_vars directory
	groupVarsDir := filepath.Join(workspaceDir, "group_vars")
	if err := os.MkdirAll(groupVarsDir, 0755); err != nil {
		return fmt.Errorf("failed to create group_vars directory: %w", err)
	}

	// Get primary domain for backwards compatibility in templates
	primaryDomain := cfg.GetPrimaryDomain()
	if primaryDomain == nil {
		return fmt.Errorf("no primary domain configured")
	}

	// Convert primary domain to Punycode for system compatibility (hostnames, etc.)
	primaryDomainASCII, err := idna.ToASCII(primaryDomain.Name)
	if err != nil {
		return fmt.Errorf("invalid primary domain name: %w", err)
	}

	// Get Unicode version for display (with accents)
	primaryDomainDisplay, err := idna.ToUnicode(primaryDomain.Name)
	if err != nil {
		// Fallback to original name if conversion fails
		primaryDomainDisplay = primaryDomain.Name
	}

	// Convert all domain names to Punycode for system compatibility
	domainsASCII := make([]config.Domain, len(cfg.Domains))
	for i, domain := range cfg.Domains {
		domainASCII, err := idna.ToASCII(domain.Name)
		if err != nil {
			return fmt.Errorf("invalid domain name %s: %w", domain.Name, err)
		}
		// Copy domain and replace name with ASCII version
		domainsASCII[i] = domain
		domainsASCII[i].Name = domainASCII
	}

	// Convert domain redirects to Punycode
	redirectsASCII := make([]config.DomainRedirect, len(cfg.DomainRedirects))
	for i, redirect := range cfg.DomainRedirects {
		fromASCII, err := idna.ToASCII(redirect.From)
		if err != nil {
			return fmt.Errorf("invalid redirect source domain %s: %w", redirect.From, err)
		}
		redirectsASCII[i] = redirect
		redirectsASCII[i].From = fromASCII
		// Convert 'To' if it contains a domain (may be a URL)
		if redirect.To != "" {
			// Try to convert, but don't fail if it's a URL
			if toASCII, err := idna.ToASCII(redirect.To); err == nil {
				redirectsASCII[i].To = toASCII
			}
		}
	}

	// Template data structure for backwards compatibility
	type TemplateData struct {
		ServerIP             string
		DNSProvider          *config.DNSProviderConfig
		Domains              []config.Domain
		PrimaryDomain        string
		PrimaryDomainDisplay string                   // Unicode version for display (with accents)
		Subdomains           []config.Subdomain       // For backwards compatibility with Ansible roles
		WebAdmin             *config.WebAdminConfig   // Extracted from primary domain for backwards compatibility
		DomainUser           *config.DomainUserConfig // Extracted from primary domain for backwards compatibility
		SystemUsers          []config.SystemUser
		DomainRedirects      []config.DomainRedirect
		MailserverSubdomain  string // Dynamic mailserver subdomain name (e.g., "mx", "mail", etc.)
	}

	// Extract mailserver subdomain name from primary domain
	mailserverSubdomain := "mx" // default fallback
	if sub := primaryDomain.GetSubdomainByRole("mailserver"); sub != nil {
		mailserverSubdomain = sub.Name
	}

	data := TemplateData{
		ServerIP:             cfg.ServerIP,
		DNSProvider:          cfg.DNSProvider,
		Domains:              domainsASCII,             // Use Punycode versions
		PrimaryDomain:        primaryDomainASCII,       // Use Punycode version
		PrimaryDomainDisplay: primaryDomainDisplay,     // Unicode version for display
		Subdomains:           primaryDomain.Subdomains, // Extract subdomains from primary domain
		WebAdmin:             primaryDomain.WebAdmin,   // Extract webadmin from primary domain
		DomainUser:           primaryDomain.DomainUser, // Extract domain_user from primary domain
		SystemUsers:          cfg.SystemUsers,
		DomainRedirects:      redirectsASCII,           // Use Punycode versions
		MailserverSubdomain:  mailserverSubdomain,
	}

	// Template for group_vars/all.yml
	tmpl := `---
# Generated from hibana-config.yaml
server_ip: {{ .ServerIP }}

{{- if .DNSProvider }}
dns_provider:
  type: {{ .DNSProvider.Type }}
  {{- if .DNSProvider.Name }}
  name: {{ .DNSProvider.Name }}
  {{- end }}
  {{- if .DNSProvider.APIToken }}
  api_token: "{{ .DNSProvider.APIToken }}"
  {{- end }}
  {{- if .DNSProvider.Endpoint }}
  endpoint: "{{ .DNSProvider.Endpoint }}"
  {{- end }}
  {{- if .DNSProvider.ApplicationKey }}
  application_key: "{{ .DNSProvider.ApplicationKey }}"
  {{- end }}
  {{- if .DNSProvider.ApplicationSecret }}
  application_secret: "{{ .DNSProvider.ApplicationSecret }}"
  {{- end }}
  {{- if .DNSProvider.ConsumerKey }}
  consumer_key: "{{ .DNSProvider.ConsumerKey }}"
  {{- end }}
{{- end }}

# Primary domain (for backwards compatibility)
primary_domain: {{ .PrimaryDomain }}

# Primary domain display name (Unicode version with accents for display)
primary_domain_display: "{{ .PrimaryDomainDisplay }}"

# Mailserver subdomain name (dynamic - e.g., "mx", "mail", etc.)
mailserver_subdomain: {{ .MailserverSubdomain }}

# Subdomains for primary domain (for backwards compatibility with Ansible roles)
subdomains:
{{- range .Subdomains }}
  - name: {{ .Name }}
    role: {{ .Role }}
{{- end }}

{{- if .WebAdmin }}
# Web admin credentials for primary domain (for backwards compatibility with Ansible roles)
webadmin:
  username: "{{ .WebAdmin.Username }}"
  password: "{{ .WebAdmin.Password }}"
{{- end }}

{{- if .DomainUser }}
# Domain user for primary domain (for backwards compatibility with Ansible roles)
domain_user:
  password: "{{ .DomainUser.Password }}"
  ssh_key_mode: "{{ .DomainUser.SSHKeyMode }}"
  {{- if .DomainUser.SSHPublicKey }}
  ssh_public_key: "{{ .DomainUser.SSHPublicKey }}"
  {{- end }}
{{- end }}

# All domains
domains:
{{- range .Domains }}
  - name: {{ .Name }}
    is_primary: {{ .IsPrimary }}
    subdomains:
    {{- range .Subdomains }}
      - name: {{ .Name }}
        role: {{ .Role }}
    {{- end }}
    {{- if .WebAdmin }}
    webadmin:
      username: "{{ .WebAdmin.Username }}"
      password: "{{ .WebAdmin.Password }}"
    {{- end }}
    email_accounts:
    {{- range .EmailAccounts }}
      - username: {{ .Username }}
        password: "{{ .Password }}"
        full_name: "{{ .FullName }}"
    {{- end }}
    {{- if .DomainUser }}
    domain_user:
      password: "{{ .DomainUser.Password }}"
      ssh_key_mode: "{{ .DomainUser.SSHKeyMode }}"
      {{- if .DomainUser.SSHPublicKey }}
      ssh_public_key: "{{ .DomainUser.SSHPublicKey }}"
      {{- end }}
    {{- end }}
{{- end }}

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
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute group_vars template: %w", err)
	}

	groupVarsPath := filepath.Join(groupVarsDir, "all.yml")
	if err := os.WriteFile(groupVarsPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write group_vars/all.yml: %w", err)
	}

	return nil
}

// CopyRoles copies all roles to the workspace
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

// CopyPlaybook copies the main playbook
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

// CopyResetPlaybook copies the reset playbook
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
