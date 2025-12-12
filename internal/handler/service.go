package handler

import (
	"encoding/json"
	"net/http"

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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc, err := h.catalog.Create(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, "failed to list services", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"services": services})
}
