package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type BlogPostHandler struct {
	blogService    *service.BlogPostService
	storageService service.StorageService
}

func NewBlogPostHandler(blogService *service.BlogPostService, storageService service.StorageService) *BlogPostHandler {
	return &BlogPostHandler{blogService: blogService, storageService: storageService}
}

func (h *BlogPostHandler) ListPublished(w http.ResponseWriter, r *http.Request) {
	posts, err := h.blogService.ListPublished(r.Context(), parseIntQuery(r, "limit", 20), parseIntQuery(r, "offset", 0))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list blog posts")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
	respondJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (h *BlogPostHandler) GetPublishedBySlug(w http.ResponseWriter, r *http.Request) {
	post, err := h.blogService.GetPublishedBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, service.ErrBlogPostNotFound) {
			respondError(w, http.StatusNotFound, "blog post not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load blog post")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
	respondJSON(w, http.StatusOK, map[string]any{"post": post})
}

func (h *BlogPostHandler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	posts, err := h.blogService.ListAdmin(
		r.Context(),
		r.URL.Query().Get("status"),
		r.URL.Query().Get("q"),
		parseIntQuery(r, "limit", 50),
		parseIntQuery(r, "offset", 0),
	)
	if err != nil {
		if errors.Is(err, service.ErrBlogPostInvalidStatus) {
			respondError(w, http.StatusBadRequest, "invalid status")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to list blog posts")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func (h *BlogPostHandler) Create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req model.CreateBlogPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	post, err := h.blogService.Create(r.Context(), &req)
	if err != nil {
		respondBlogError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{"post": post})
}

func (h *BlogPostHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid blog post ID")
		return
	}

	defer r.Body.Close()

	var req model.UpdateBlogPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	post, err := h.blogService.Update(r.Context(), id, &req)
	if err != nil {
		respondBlogError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"post": post})
}

func (h *BlogPostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid blog post ID")
		return
	}

	if err := h.blogService.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete blog post")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *BlogPostHandler) UploadCover(w http.ResponseWriter, r *http.Request) {
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

	key := h.storageService.GenerateKey("blog-posts", header.Filename)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}

	url, err := h.storageService.UploadFile(r.Context(), key, file, contentType)
	if err != nil {
		slog.Warn("blog cover upload error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"url": url})
}

func respondBlogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBlogPostNotFound):
		respondError(w, http.StatusNotFound, "blog post not found")
	case errors.Is(err, service.ErrBlogPostTitleRequired),
		errors.Is(err, service.ErrBlogPostContentRequired),
		errors.Is(err, service.ErrBlogPostInvalidStatus),
		errors.Is(err, service.ErrBlogPostDuplicateSlug):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "failed to save blog post")
	}
}

func parseIntQuery(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
