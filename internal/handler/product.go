package handler

import (
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// ProductHandler handles HTTP requests for product management.
type ProductHandler struct {
	catalog        *service.ProductCatalog
	storageService service.StorageService
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(catalog *service.ProductCatalog, storageService service.StorageService) *ProductHandler {
	return &ProductHandler{catalog: catalog, storageService: storageService}
}

// ListProducts handles GET /products — returns active products (public).
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.catalog.ListActive(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list products")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"products": products})
}

// ListAllProducts handles GET /admin/products — returns all products (admin).
func (h *ProductHandler) ListAllProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.catalog.ListAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list products")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"products": products})
}

// GetProduct handles GET /products/{id}.
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product ID")
		return
	}

	p, err := h.catalog.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "product not found")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"product": p})
}

// CreateProduct handles POST /products (admin only).
func (h *ProductHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req model.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, err := h.catalog.Create(r.Context(), &req)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{"product": p})
}

// UpdateProduct handles PUT /products/{id} (admin only).
func (h *ProductHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product ID")
		return
	}

	defer r.Body.Close()

	var req model.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, err := h.catalog.Update(r.Context(), id, &req)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"product": p})
}

// DeleteProduct handles DELETE /products/{id} (admin only).
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product ID")
		return
	}

	if err := h.catalog.Delete(r.Context(), id); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UploadProductImage handles POST /products/upload-image (admin only).
func (h *ProductHandler) UploadProductImage(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil || !h.storageService.IsConfigured() {
		respondError(w, http.StatusInternalServerError, "storage not configured")
		return
	}

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

	key := h.storageService.GenerateKey("products", header.Filename)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	imageURL, err := h.storageService.UploadFile(r.Context(), key, file, contentType)
	if err != nil {
		slog.Warn("product image upload error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"image_url": imageURL})
}
