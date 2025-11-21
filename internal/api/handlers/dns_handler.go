package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
)

// DNSHandler handles DNS record requests
type DNSHandler struct {
	service *services.DNSService
}

// NewDNSHandler creates a new DNS handler
func NewDNSHandler(service *services.DNSService) *DNSHandler {
	return &DNSHandler{service: service}
}

// HandleDomainDNS handles /api/v1/domains/:id/dns
func (h *DNSHandler) HandleDomainDNS(w http.ResponseWriter, r *http.Request) {
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

// HandleDNSRecord handles /api/v1/dns/:id
func (h *DNSHandler) HandleDNSRecord(w http.ResponseWriter, r *http.Request) {
	id, err := extractID(r.URL.Path, "/api/v1/dns/")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid DNS record ID")
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

// ListByDomain returns all DNS records for a domain
func (h *DNSHandler) ListByDomain(w http.ResponseWriter, r *http.Request, domainID int) {
	records, err := h.service.GetByDomain(domainID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    records,
	})
}

// Get returns a single DNS record
func (h *DNSHandler) Get(w http.ResponseWriter, r *http.Request, id int) {
	record, err := h.service.GetByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    record,
	})
}

// Create creates a new DNS record
func (h *DNSHandler) Create(w http.ResponseWriter, r *http.Request, domainID int) {
	var create models.DNSRecordCreate
	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	record, err := h.service.Create(domainID, &create)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "DNS record created successfully",
		Data:    record,
	})
}

// Update updates a DNS record
func (h *DNSHandler) Update(w http.ResponseWriter, r *http.Request, id int) {
	var update models.DNSRecordUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	record, err := h.service.Update(id, &update)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "DNS record updated successfully",
		Data:    record,
	})
}

// Delete deletes a DNS record
func (h *DNSHandler) Delete(w http.ResponseWriter, r *http.Request, id int) {
	if err := h.service.Delete(id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "DNS record deleted successfully",
	})
}
