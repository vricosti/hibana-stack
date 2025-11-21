package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
)

// DomainHandler handles domain-related requests
type DomainHandler struct {
	service *services.DomainService
}

// NewDomainHandler creates a new domain handler
func NewDomainHandler(service *services.DomainService) *DomainHandler {
	return &DomainHandler{service: service}
}

// HandleDomains handles /api/v1/domains
func (h *DomainHandler) HandleDomains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.List(w, r)
	case http.MethodPost:
		h.Create(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandleDomain handles /api/v1/domains/:id
func (h *DomainHandler) HandleDomain(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	id, err := extractID(r.URL.Path, "/api/v1/domains/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid domain ID")
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

// List returns all domains
func (h *DomainHandler) List(w http.ResponseWriter, r *http.Request) {
	domains, err := h.service.GetAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    domains,
	})
}

// Get returns a single domain
func (h *DomainHandler) Get(w http.ResponseWriter, r *http.Request, id int) {
	domain, err := h.service.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    domain,
	})
}

// Create creates a new domain
func (h *DomainHandler) Create(w http.ResponseWriter, r *http.Request) {
	var create models.DomainCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	domain, err := h.service.Create(&create)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Domain created successfully",
		Data:    domain,
	})
}

// Update updates an existing domain
func (h *DomainHandler) Update(w http.ResponseWriter, r *http.Request, id int) {
	var update models.DomainUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	domain, err := h.service.Update(id, &update)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Domain updated successfully",
		Data:    domain,
	})
}

// Delete deletes a domain
func (h *DomainHandler) Delete(w http.ResponseWriter, r *http.Request, id int) {
	if err := h.service.Delete(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Domain deleted successfully",
	})
}

// GetStats returns domain statistics
func (h *DomainHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStats()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    stats,
	})
}

// extractID extracts the ID from a URL path
func extractID(path, prefix string) (int, error) {
	idStr := strings.TrimPrefix(path, prefix)
	idStr = strings.Split(idStr, "/")[0]
	return strconv.Atoi(idStr)
}
