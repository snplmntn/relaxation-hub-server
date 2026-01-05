package handler

import (
	"encoding/json"
	"log"
	"mime"
	"net/http"
	"path/filepath"

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
		log.Printf("Storage upload error: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"image_url": imageURL,
	})
}
