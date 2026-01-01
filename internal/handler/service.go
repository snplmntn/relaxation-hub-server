package handler

import (
	"encoding/json"
	"net/http"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type ServiceHandler struct {
	catalog *service.ServiceCatalog
}

func NewServiceHandler(catalog *service.ServiceCatalog) *ServiceHandler {
	return &ServiceHandler{catalog: catalog}
}

// CreateService handles POST /services for adding a catalog entry.
func (h *ServiceHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req model.CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	svc, err := h.catalog.Create(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"service": svc})
}

// ListServices handles GET /services to fetch active services.
func (h *ServiceHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.catalog.ListActive(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list services")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"services": services})
}

// ListRecentServices handles GET /services/recent to fetch user's recently booked services.
// Requires authentication.
func (h *ServiceHandler) ListRecentServices(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	services, err := h.catalog.ListRecentByUser(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list recent services")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"services": services})
}

// ListPopularServices handles GET /services/popular to fetch most-booked services.
func (h *ServiceHandler) ListPopularServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.catalog.ListPopular(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list popular services")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"services": services})
}

// ListUnavailableServices handles GET /services/unavailable to fetch inactive services.
func (h *ServiceHandler) ListUnavailableServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.catalog.ListUnavailable(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list unavailable services")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"services": services})
}
