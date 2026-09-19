package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"golang.org/x/net/html"
)

const maxBlogImageBytes int64 = 5 << 20

var supportedBlogImageTypes = map[string]string{
	"image/gif":  ".gif",
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

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

	w.Header().Set("Cache-Control", "public, no-cache, must-revalidate")
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

	w.Header().Set("Cache-Control", "public, no-cache, must-revalidate")
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
		h.deleteBlogAssetKeys(r.Context(), blogAssetKeysFromURLs(req.UploadedAssets))
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

	previous, err := h.blogService.GetAdminByID(r.Context(), id)
	if err != nil {
		respondBlogError(w, err)
		return
	}

	post, err := h.blogService.Update(r.Context(), id, &req)
	if err != nil {
		h.deleteBlogAssetKeys(r.Context(), blogAssetKeysFromURLs(req.UploadedAssets))
		respondBlogError(w, err)
		return
	}

	h.deleteRemovedBlogAssets(r.Context(), previous, post)
	respondJSON(w, http.StatusOK, map[string]any{"post": post})
}

func (h *BlogPostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid blog post ID")
		return
	}

	post, err := h.blogService.GetAdminByID(r.Context(), id)
	if err != nil {
		respondBlogError(w, err)
		return
	}

	if err := h.blogService.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete blog post")
		return
	}

	h.deleteBlogAssetKeys(r.Context(), blogAssetKeys(post))
	w.WriteHeader(http.StatusNoContent)
}

func (h *BlogPostHandler) UploadCover(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil || !h.storageService.IsConfigured() {
		respondError(w, http.StatusInternalServerError, "storage not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBlogImageBytes+(1<<20))
	if err := r.ParseMultipartForm(maxBlogImageBytes); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			respondError(w, http.StatusRequestEntityTooLarge, "image must be 5 MB or smaller")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid form data")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing image file")
		return
	}
	defer file.Close()

	if header.Size <= 0 || header.Size > maxBlogImageBytes {
		respondError(w, http.StatusRequestEntityTooLarge, "image must be 5 MB or smaller")
		return
	}

	signature := make([]byte, 512)
	read, readErr := file.Read(signature)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		respondError(w, http.StatusBadRequest, "could not read image")
		return
	}
	contentType := http.DetectContentType(signature[:read])
	extension, supported := supportedBlogImageTypes[contentType]
	if !supported {
		respondError(w, http.StatusUnsupportedMediaType, "supported images are JPEG, PNG, GIF, and WebP")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		respondError(w, http.StatusBadRequest, "could not read image")
		return
	}

	baseName := strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	if strings.TrimSpace(baseName) == "" {
		baseName = "image"
	}
	key := h.storageService.GenerateKey("blog-posts", baseName+extension)

	_, err = h.storageService.UploadFile(r.Context(), key, file, contentType)
	if err != nil {
		slog.Warn("blog cover upload error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}

	filename := path.Base(filepath.ToSlash(key))
	assetPath := "/api/v1/blog-assets/" + url.PathEscape(filename)
	respondJSON(w, http.StatusCreated, map[string]string{
		"url": absoluteRequestURL(r, assetPath),
	})
}

func (h *BlogPostHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil || !h.storageService.IsConfigured() {
		respondError(w, http.StatusServiceUnavailable, "storage not configured")
		return
	}

	filename := chi.URLParam(r, "filename")
	if filename == "" || filename != path.Base(filename) {
		respondError(w, http.StatusBadRequest, "invalid image path")
		return
	}

	if err := h.storageService.DeleteFile(r.Context(), "blog-posts/"+filename); err != nil {
		slog.Warn("blog image cleanup error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to delete image")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BlogPostHandler) GetAsset(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil || !h.storageService.IsConfigured() {
		respondError(w, http.StatusServiceUnavailable, "storage not configured")
		return
	}

	filename := chi.URLParam(r, "filename")
	if filename == "" || filename != path.Base(filename) {
		respondError(w, http.StatusBadRequest, "invalid image path")
		return
	}

	signedURL, err := h.storageService.GetPresignedURL(
		r.Context(),
		"blog-posts/"+filename,
		time.Hour,
	)
	if err != nil {
		slog.Warn("blog image URL error", "error", err)
		respondError(w, http.StatusNotFound, "image not found")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=300")
	http.Redirect(w, r, signedURL, http.StatusTemporaryRedirect)
}

func absoluteRequestURL(r *http.Request, requestPath string) string {
	scheme := firstForwardedValue(r.Header.Get("X-Forwarded-Proto"))
	if scheme != "http" && scheme != "https" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := firstForwardedValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}

	return scheme + "://" + host + requestPath
}

func firstForwardedValue(value string) string {
	if index := strings.IndexByte(value, ','); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func (h *BlogPostHandler) deleteRemovedBlogAssets(ctx context.Context, previous, current *model.BlogPost) {
	previousKeys := blogAssetKeys(previous)
	for key := range blogAssetKeys(current) {
		delete(previousKeys, key)
	}
	h.deleteBlogAssetKeys(ctx, previousKeys)
}

func (h *BlogPostHandler) deleteBlogAssetKeys(ctx context.Context, keys map[string]struct{}) {
	if h.storageService == nil || !h.storageService.IsConfigured() {
		return
	}
	for key := range keys {
		if err := h.storageService.DeleteFile(ctx, key); err != nil {
			slog.Warn("blog image cleanup error", "key", key, "error", err)
		}
	}
}

func blogAssetKeys(post *model.BlogPost) map[string]struct{} {
	keys := make(map[string]struct{})
	if post == nil {
		return keys
	}
	if post.CoverImageURL != nil {
		if key := blogAssetKeyFromURL(*post.CoverImageURL); key != "" {
			keys[key] = struct{}{}
		}
	}

	tokenizer := html.NewTokenizer(strings.NewReader(post.ContentHTML))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return keys
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != "img" {
				continue
			}
			for _, attribute := range token.Attr {
				if attribute.Key != "src" {
					continue
				}
				if key := blogAssetKeyFromURL(attribute.Val); key != "" {
					keys[key] = struct{}{}
				}
			}
		}
	}
}

func blogAssetKeysFromURLs(urls []string) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, assetURL := range urls {
		if key := blogAssetKeyFromURL(assetURL); key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func blogAssetKeyFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}

	const stableMarker = "/api/v1/blog-assets/"
	if index := strings.Index(parsed.Path, stableMarker); index >= 0 {
		filename := strings.TrimPrefix(parsed.Path[index:], stableMarker)
		if filename != "" && filename == path.Base(filename) {
			return "blog-posts/" + filename
		}
	}

	const legacyMarker = "/blog-posts/"
	if index := strings.Index(parsed.Path, legacyMarker); index >= 0 {
		filename := strings.TrimPrefix(parsed.Path[index:], legacyMarker)
		if filename != "" && filename == path.Base(filename) {
			return "blog-posts/" + filename
		}
	}
	return ""
}

func respondBlogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBlogPostNotFound):
		respondError(w, http.StatusNotFound, "blog post not found")
	case errors.Is(err, service.ErrBlogPostTitleRequired),
		errors.Is(err, service.ErrBlogPostContentRequired),
		errors.Is(err, service.ErrBlogPostInvalidContent),
		errors.Is(err, service.ErrBlogPostInvalidStatus),
		errors.Is(err, service.ErrBlogPostDuplicateSlug):
		respondServiceError(w, http.StatusBadRequest, err)
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
