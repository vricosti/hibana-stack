package models

// Service represents a subdomain service with its Docker container
type Service struct {
	Name          string `json:"name"`           // Subdomain name (www, adm, webmail, mail)
	Role          string `json:"role"`           // Role (website, webadmin, webmail, mailserver)
	ContainerName string `json:"container_name"` // Docker container name
	Status        string `json:"status"`         // running, stopped, not_deployed
	Deployable    bool   `json:"deployable"`     // Can be deployed (only www)
}

// ServiceAction represents a request to perform an action on a service
type ServiceAction struct {
	Action string `json:"action"` // start, stop, restart
}

// ServiceLogs represents logs from a service
type ServiceLogs struct {
	ServiceName string `json:"service_name"`
	Logs        string `json:"logs"`
	Lines       int    `json:"lines"`
}

// DeployRequest represents a deployment request
type DeployRequest struct {
	Source    string `json:"source"`     // "git" or "upload"
	GitURL    string `json:"git_url"`    // Git repository URL (SSH format for private repos)
	GitBranch string `json:"git_branch"` // Branch name (default: main)
	FileName  string `json:"file_name"`  // Original filename for upload
}

// DeployResponse represents the result of a deployment
type DeployResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"` // Build/deploy output
}
