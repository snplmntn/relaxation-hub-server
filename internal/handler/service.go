package handler

import (
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type ServiceHandler struct {
	catalog        *service.ServiceCatalog
	storageService service.StorageService
}

func NewServiceHandler(catalog *service.ServiceCatalog, storageService service.StorageService) *ServiceHandler {
	return &ServiceHandler{catalog: catalog, storageService: storageService}
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

// UploadServiceImage handles file upload for service preview image.
func (h *ServiceHandler) UploadServiceImage(w http.ResponseWriter, r *http.Request) {
	// Verify storage is configured
	if h.storageService == nil || !h.storageService.IsConfigured() {
		respondError(w, http.StatusInternalServerError, "storage not configured")
		return
	}

	// Parse multipart form (max 5MB)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid form data")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing image file")
		return
	}
	defer file.Close()

	// Generate storage key - using "services" prefix
	key := h.storageService.GenerateKey("services", header.Filename)

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Upload to storage
	imageURL, err := h.storageService.UploadFile(r.Context(), key, file, contentType)
	if err != nil {
		slog.Warn("storage upload error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"image_url": imageURL,
	})
}

// UpdateService handles PATCH /services/{id} for updating an existing service.
func (h *ServiceHandler) UpdateService(w http.ResponseWriter, r *http.Request) {
	serviceIDStr := chi.URLParam(r, "id")
	serviceID, err := strconv.ParseInt(serviceIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid service ID")
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	svc, err := h.catalog.Update(r.Context(), serviceID, req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "service not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"service": svc})
}

// DeleteService handles DELETE /services/{id} for soft deleting a service.
func (h *ServiceHandler) DeleteService(w http.ResponseWriter, r *http.Request) {
	serviceIDStr := chi.URLParam(r, "id")
	serviceID, err := strconv.ParseInt(serviceIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid service ID")
		return
	}

	if err := h.catalog.Delete(r.Context(), serviceID); err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "service not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
