package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
)

// AliasHandler handles email alias requests
type AliasHandler struct {
	service *services.AliasService
}

// NewAliasHandler creates a new alias handler
func NewAliasHandler(service *services.AliasService) *AliasHandler {
	return &AliasHandler{service: service}
}

// HandleDomainAliases handles /api/v1/domains/:id/aliases
func (h *AliasHandler) HandleDomainAliases(w http.ResponseWriter, r *http.Request) {
	// Extract domain ID from path
	domainID, err := extractID(r.URL.Path, "/api/v1/domains/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.ListByDomain(w, r, domainID)
	case http.MethodPost:
		h.Create(w, r, domainID)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandleAlias handles /api/v1/aliases/:id
func (h *AliasHandler) HandleAlias(w http.ResponseWriter, r *http.Request) {
	id, err := extractID(r.URL.Path, "/api/v1/aliases/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid alias ID")
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

// ListByDomain returns all email aliases for a domain
func (h *AliasHandler) ListByDomain(w http.ResponseWriter, r *http.Request, domainID int) {
	aliases, err := h.service.GetByDomain(domainID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    aliases,
	})
}

// Get returns a single email alias
func (h *AliasHandler) Get(w http.ResponseWriter, r *http.Request, id int) {
	alias, err := h.service.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    alias,
	})
}

// Create creates a new email alias
func (h *AliasHandler) Create(w http.ResponseWriter, r *http.Request, domainID int) {
	var create models.EmailAliasCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	alias, err := h.service.Create(domainID, &create)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Email alias created successfully",
		Data:    alias,
	})
}

// Update updates an email alias
func (h *AliasHandler) Update(w http.ResponseWriter, r *http.Request, id int) {
	var update models.EmailAliasUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	alias, err := h.service.Update(id, &update)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Email alias updated successfully",
		Data:    alias,
	})
}

// Delete deletes an email alias
func (h *AliasHandler) Delete(w http.ResponseWriter, r *http.Request, id int) {
	if err := h.service.Delete(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Email alias deleted successfully",
	})
}

// ListAll returns all email aliases across all domains
func (h *AliasHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	aliases, err := h.service.GetAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    aliases,
	})
}
