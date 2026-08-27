package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type LegalDocumentHandler struct {
	legalService *service.LegalDocumentService
}

func NewLegalDocumentHandler(legalService *service.LegalDocumentService) *LegalDocumentHandler {
	return &LegalDocumentHandler{legalService: legalService}
}

func (h *LegalDocumentHandler) GetLegalDocument(w http.ResponseWriter, r *http.Request) {
	docKey := chi.URLParam(r, "docKey")
	doc, err := h.legalService.GetByLegacyKey(r.Context(), docKey)
	if err != nil {
		if err == service.ErrInvalidLegalDocumentKey || err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "legal document not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load legal document")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"document": doc,
	})
}

func (h *LegalDocumentHandler) GetContentPage(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	doc, err := h.legalService.GetByContentKey(r.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrInvalidLegalDocumentKey) {
			respondError(w, http.StatusBadRequest, "invalid content key")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to load content")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"key":         doc.DocKey,
		"title":       doc.Title,
		"content":     doc.ContentMarkdown,
		"lastUpdated": doc.UpdatedAt,
	})
}

func (h *LegalDocumentHandler) UpdateContentPage(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req model.UpdateLegalDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	doc, err := h.legalService.UpdateContentByKey(r.Context(), key, req.Title, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidLegalDocumentKey):
			respondError(w, http.StatusBadRequest, "invalid content key")
		case errors.Is(err, service.ErrLegalDocumentTitleRequired), errors.Is(err, service.ErrLegalDocumentContentMissing):
			respondServiceError(w, http.StatusBadRequest, err)
		default:
			respondError(w, http.StatusInternalServerError, "failed to update content")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"key":         doc.DocKey,
		"title":       doc.Title,
		"content":     doc.ContentMarkdown,
		"lastUpdated": doc.UpdatedAt,
	})
}
