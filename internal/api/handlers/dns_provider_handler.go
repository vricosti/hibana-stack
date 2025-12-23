package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
)

// DNSProviderHandler handles DNS provider requests
type DNSProviderHandler struct {
	service *services.DNSProviderService
}

// NewDNSProviderHandler creates a new DNS provider handler
func NewDNSProviderHandler(service *services.DNSProviderService) *DNSProviderHandler {
	return &DNSProviderHandler{service: service}
}

// HandleProviders handles /api/v1/dns-providers
func (h *DNSProviderHandler) HandleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.List(w, r)
	case http.MethodPost:
		h.Create(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandleProvider handles /api/v1/dns-providers/:id
func (h *DNSProviderHandler) HandleProvider(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := r.URL.Path
	idStr := strings.TrimPrefix(path, "/api/v1/dns-providers/")
	idStr = strings.Split(idStr, "/")[0]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid provider ID")
		return
	}

	// Check for nested routes
	if strings.Contains(path, "/domains") {
		h.GetAvailableDomains(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.Get(w, r, id)
	case http.MethodPut:
		h.Update(w, r, id)
	case http.MethodDelete:
		h.Delete(w, r, id)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// List returns all DNS providers
func (h *DNSProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.GetAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Mask sensitive data
	for i := range providers {
		providers[i].APIToken = maskString(providers[i].APIToken)
		providers[i].ApplicationKey = maskString(providers[i].ApplicationKey)
		providers[i].ApplicationSecret = maskString(providers[i].ApplicationSecret)
		providers[i].ConsumerKey = maskString(providers[i].ConsumerKey)
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    providers,
	})
}

// Get returns a single DNS provider
func (h *DNSProviderHandler) Get(w http.ResponseWriter, r *http.Request, id int) {
	provider, err := h.service.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	// Mask sensitive data
	provider.APIToken = maskString(provider.APIToken)
	provider.ApplicationKey = maskString(provider.ApplicationKey)
	provider.ApplicationSecret = maskString(provider.ApplicationSecret)
	provider.ConsumerKey = maskString(provider.ConsumerKey)

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    provider,
	})
}

// Create creates a new DNS provider
func (h *DNSProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var create services.DNSProviderCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if create.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if create.Provider == "" {
		respondError(w, http.StatusBadRequest, "provider is required")
		return
	}

	// Test connection first
	if err := h.service.TestConnection(&create); err != nil {
		respondError(w, http.StatusBadRequest, "connection test failed: "+err.Error())
		return
	}

	provider, err := h.service.Create(&create)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Mask sensitive data
	provider.APIToken = maskString(provider.APIToken)
	provider.ApplicationKey = maskString(provider.ApplicationKey)
	provider.ApplicationSecret = maskString(provider.ApplicationSecret)
	provider.ConsumerKey = maskString(provider.ConsumerKey)

	respondJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "DNS provider created successfully",
		Data:    provider,
	})
}

// Update updates an existing DNS provider
func (h *DNSProviderHandler) Update(w http.ResponseWriter, r *http.Request, id int) {
	var update services.DNSProviderCreate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if update.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if update.Provider == "" {
		respondError(w, http.StatusBadRequest, "provider is required")
		return
	}

	// Test connection if credentials changed
	if update.APIToken != "" || update.ApplicationKey != "" {
		if err := h.service.TestConnection(&update); err != nil {
			respondError(w, http.StatusBadRequest, "connection test failed: "+err.Error())
			return
		}
	}

	provider, err := h.service.Update(id, &update)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Mask sensitive data
	provider.APIToken = maskString(provider.APIToken)
	provider.ApplicationKey = maskString(provider.ApplicationKey)
	provider.ApplicationSecret = maskString(provider.ApplicationSecret)
	provider.ConsumerKey = maskString(provider.ConsumerKey)

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "DNS provider updated successfully",
		Data:    provider,
	})
}

// Delete deletes a DNS provider
func (h *DNSProviderHandler) Delete(w http.ResponseWriter, r *http.Request, id int) {
	if err := h.service.Delete(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "DNS provider deleted successfully",
	})
}

// GetAvailableDomains returns domains available from the provider
func (h *DNSProviderHandler) GetAvailableDomains(w http.ResponseWriter, r *http.Request, providerID int) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	domains, err := h.service.GetAvailableDomains(providerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    domains,
	})
}

// TestConnection tests connection to a DNS provider
func (h *DNSProviderHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var create services.DNSProviderCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.TestConnection(&create); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Connection successful",
	})
}

func maskString(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
