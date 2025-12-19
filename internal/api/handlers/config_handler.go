package handlers

import (
	"net/http"

	"github.com/vricosti/hibana-stack/internal/api/models"
	"github.com/vricosti/hibana-stack/internal/api/services"
)

// ConfigHandler handles configuration-related requests
type ConfigHandler struct {
	service *services.ConfigService
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(service *services.ConfigService) *ConfigHandler {
	return &ConfigHandler{service: service}
}

// GetDNSProvider returns the DNS provider configuration
func (h *ConfigHandler) GetDNSProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config, err := h.service.GetDNSProvider()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    config,
	})
}
