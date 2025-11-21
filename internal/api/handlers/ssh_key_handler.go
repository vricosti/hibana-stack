package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
)

// SSHKeyHandler handles SSH key-related HTTP requests
type SSHKeyHandler struct {
	service *services.SSHKeyService
}

// NewSSHKeyHandler creates a new SSH key handler
func NewSSHKeyHandler(service *services.SSHKeyService) *SSHKeyHandler {
	return &SSHKeyHandler{service: service}
}

// HandleDomainSSHKeys handles /api/v1/domains/:domain/ssh-keys
func (h *SSHKeyHandler) HandleDomainSSHKeys(w http.ResponseWriter, r *http.Request) {
	// Extract domain name from path: /api/v1/domains/:domain/ssh-keys or /api/v1/domains/:domain/ssh-keys/:fingerprint
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 5 {
		respondError(w, http.StatusBadRequest, "invalid path")
		return
	}

	domainName := pathParts[3] // domains/:domain/ssh-keys

	// Check if there's a fingerprint in the path
	if len(pathParts) > 5 {
		// /api/v1/domains/:domain/ssh-keys/:fingerprint
		fingerprint, err := url.PathUnescape(pathParts[5])
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid fingerprint")
			return
		}
		h.handleSSHKey(w, r, domainName, fingerprint)
		return
	}

	// /api/v1/domains/:domain/ssh-keys
	switch r.Method {
	case http.MethodGet:
		h.ListKeys(w, r, domainName)
	case http.MethodPost:
		h.AddKey(w, r, domainName)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSSHKey handles operations on a specific SSH key
func (h *SSHKeyHandler) handleSSHKey(w http.ResponseWriter, r *http.Request, domainName, fingerprint string) {
	switch r.Method {
	case http.MethodDelete:
		h.RemoveKey(w, r, domainName, fingerprint)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ListKeys lists all SSH keys for a domain
func (h *SSHKeyHandler) ListKeys(w http.ResponseWriter, r *http.Request, domainName string) {
	keys, err := h.service.ListKeys(domainName)
	if err != nil {
		log.Printf("Failed to list SSH keys for %s: %v", domainName, err)
		respondError(w, http.StatusInternalServerError, "Failed to list SSH keys: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    models.SSHKeyListResponse{Keys: keys},
	})
}

// AddKey adds a new SSH key for a domain
func (h *SSHKeyHandler) AddKey(w http.ResponseWriter, r *http.Request, domainName string) {
	var req models.SSHKeyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PublicKey == "" {
		respondError(w, http.StatusBadRequest, "public_key is required")
		return
	}

	if err := h.service.AddKey(domainName, req.PublicKey, req.Label); err != nil {
		log.Printf("Failed to add SSH key for %s: %v", domainName, err)
		respondError(w, http.StatusInternalServerError, "Failed to add SSH key: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "SSH key added successfully",
	})
}

// RemoveKey removes an SSH key from a domain
func (h *SSHKeyHandler) RemoveKey(w http.ResponseWriter, r *http.Request, domainName, fingerprint string) {
	if err := h.service.RemoveKey(domainName, fingerprint); err != nil {
		log.Printf("Failed to remove SSH key for %s: %v", domainName, err)
		respondError(w, http.StatusInternalServerError, "Failed to remove SSH key: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "SSH key removed successfully",
	})
}

// GenerateKeyPair generates a new SSH key pair
func (h *SSHKeyHandler) GenerateKeyPair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	publicKey, privateKey, err := h.service.GenerateKeyPair()
	if err != nil {
		log.Printf("Failed to generate SSH key pair: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to generate SSH key pair: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data: map[string]string{
			"public_key":  publicKey,
			"private_key": privateKey,
		},
	})
}
