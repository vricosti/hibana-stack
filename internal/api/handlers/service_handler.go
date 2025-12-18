package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
	"github.com/vricosti/hibana-stack/internal/utils"
)

// ServiceHandler handles service-related HTTP requests
type ServiceHandler struct {
	service *services.ServiceService
}

// NewServiceHandler creates a new service handler
func NewServiceHandler(service *services.ServiceService) *ServiceHandler {
	return &ServiceHandler{service: service}
}

// HandleServices routes service requests
func (h *ServiceHandler) HandleServices(w http.ResponseWriter, r *http.Request) {
	// Extract domain from path: /api/v1/domains/{domain}/services
	parts := strings.Split(r.URL.Path, "/")

	// Find domain in path
	domainIdx := -1
	for i, p := range parts {
		if p == "domains" && i+1 < len(parts) {
			domainIdx = i + 1
			break
		}
	}

	if domainIdx == -1 || domainIdx >= len(parts) {
		respondServiceError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	domainName := parts[domainIdx]

	// Check if there's a service name in the path
	serviceIdx := -1
	for i, p := range parts {
		if p == "services" && i+1 < len(parts) {
			serviceIdx = i + 1
			break
		}
	}

	// /api/v1/domains/{domain}/services
	if serviceIdx == -1 || serviceIdx >= len(parts) {
		if r.Method == http.MethodGet {
			h.List(w, r, domainName)
			return
		}
		respondServiceError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	serviceName := parts[serviceIdx]

	// Check for action in path
	actionIdx := serviceIdx + 1
	if actionIdx < len(parts) {
		action := parts[actionIdx]
		switch action {
		case "start":
			h.Start(w, r, domainName, serviceName)
		case "stop":
			h.Stop(w, r, domainName, serviceName)
		case "restart":
			h.Restart(w, r, domainName, serviceName)
		case "logs":
			h.Logs(w, r, domainName, serviceName)
		case "deploy":
			h.Deploy(w, r, domainName, serviceName)
		default:
			respondServiceError(w, http.StatusNotFound, "Unknown action")
		}
		return
	}

	respondServiceError(w, http.StatusNotFound, "Not found")
}

// List returns all services for a domain
func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request, domainName string) {
	services, err := h.service.ListServices(domainName)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"data": services,
	})
}

// Start starts a service
func (h *ServiceHandler) Start(w http.ResponseWriter, r *http.Request, domainName, serviceName string) {
	if r.Method != http.MethodPost {
		respondServiceError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	err := h.service.StartService(domainName, serviceName)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Service %s started", serviceName),
	})
}

// Stop stops a service
func (h *ServiceHandler) Stop(w http.ResponseWriter, r *http.Request, domainName, serviceName string) {
	if r.Method != http.MethodPost {
		respondServiceError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	err := h.service.StopService(domainName, serviceName)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Service %s stopped", serviceName),
	})
}

// Restart restarts a service
func (h *ServiceHandler) Restart(w http.ResponseWriter, r *http.Request, domainName, serviceName string) {
	if r.Method != http.MethodPost {
		respondServiceError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	err := h.service.RestartService(domainName, serviceName)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Service %s restarted", serviceName),
	})
}

// Logs returns logs for a service
func (h *ServiceHandler) Logs(w http.ResponseWriter, r *http.Request, domainName, serviceName string) {
	if r.Method != http.MethodGet {
		respondServiceError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get lines parameter
	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		if l, err := strconv.Atoi(linesStr); err == nil && l > 0 {
			lines = l
		}
	}

	logs, err := h.service.GetLogs(domainName, serviceName, lines)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"data": logs,
	})
}

// Deploy deploys a new version of a service
func (h *ServiceHandler) Deploy(w http.ResponseWriter, r *http.Request, domainName, serviceName string) {
	if r.Method != http.MethodPost {
		respondServiceError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Only www is deployable
	if serviceName != "www" {
		respondServiceError(w, http.StatusBadRequest, "Only www service is deployable")
		return
	}

	// Check content type
	contentType := r.Header.Get("Content-Type")

	var req models.DeployRequest

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle file upload
		err := r.ParseMultipartForm(100 << 20) // 100MB max
		if err != nil {
			respondServiceError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
			return
		}

		req.Source = r.FormValue("source")
		req.GitURL = r.FormValue("git_url")
		req.GitBranch = r.FormValue("git_branch")

		if req.Source == "upload" {
			file, header, err := r.FormFile("file")
			if err != nil {
				respondServiceError(w, http.StatusBadRequest, "No file uploaded")
				return
			}
			defer file.Close()

			// Save to temp file
			tempFile, err := os.CreateTemp("", "hibana-upload-*"+filepath.Ext(header.Filename))
			if err != nil {
				respondServiceError(w, http.StatusInternalServerError, "Failed to create temp file")
				return
			}
			defer os.Remove(tempFile.Name())
			defer tempFile.Close()

			if _, err := io.Copy(tempFile, file); err != nil {
				respondServiceError(w, http.StatusInternalServerError, "Failed to save uploaded file")
				return
			}

			// Extract archive to service directory
			domainPath := utils.DomainToPath(domainName)
			servicePath := filepath.Join(domainPath, serviceName)

			if err := h.service.ExtractArchive(tempFile.Name(), servicePath); err != nil {
				respondServiceError(w, http.StatusInternalServerError, "Failed to extract archive: "+err.Error())
				return
			}

			req.FileName = header.Filename
		}
	} else {
		// JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondServiceError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
	}

	result, err := h.service.Deploy(domainName, serviceName, &req)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"data": result,
	})
}

func respondServiceJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Failed to marshal response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondServiceError(w http.ResponseWriter, code int, message string) {
	respondServiceJSON(w, code, map[string]string{"error": message})
}
