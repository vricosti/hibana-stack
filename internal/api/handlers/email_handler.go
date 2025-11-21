package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
)

// EmailHandler handles email account requests
type EmailHandler struct {
	service *services.EmailService
}

// NewEmailHandler creates a new email handler
func NewEmailHandler(service *services.EmailService) *EmailHandler {
	return &EmailHandler{service: service}
}

// HandleDomainEmails handles /api/v1/domains/:id/emails
func (h *EmailHandler) HandleDomainEmails(w http.ResponseWriter, r *http.Request) {
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

// HandleEmail handles /api/v1/emails/:id
func (h *EmailHandler) HandleEmail(w http.ResponseWriter, r *http.Request) {
	id, err := extractID(r.URL.Path, "/api/v1/emails/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid email ID")
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

// ListByDomain returns all email accounts for a domain
func (h *EmailHandler) ListByDomain(w http.ResponseWriter, r *http.Request, domainID int) {
	accounts, err := h.service.GetByDomain(domainID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    accounts,
	})
}

// Get returns a single email account
func (h *EmailHandler) Get(w http.ResponseWriter, r *http.Request, id int) {
	account, err := h.service.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    account,
	})
}

// Create creates a new email account
func (h *EmailHandler) Create(w http.ResponseWriter, r *http.Request, domainID int) {
	var create models.EmailAccountCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	account, err := h.service.Create(domainID, &create)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Email account created successfully",
		Data:    account,
	})
}

// Update updates an email account
func (h *EmailHandler) Update(w http.ResponseWriter, r *http.Request, id int) {
	var update models.EmailAccountUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	account, err := h.service.Update(id, &update)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Email account updated successfully",
		Data:    account,
	})
}

// Delete deletes an email account
func (h *EmailHandler) Delete(w http.ResponseWriter, r *http.Request, id int) {
	if err := h.service.Delete(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Email account deleted successfully",
	})
}

// ListAll returns all email accounts across all domains
func (h *EmailHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.service.GetAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    accounts,
	})
}
