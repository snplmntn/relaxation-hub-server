package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type ApplicationHandler struct {
	service *service.ApplicationService
}

func NewApplicationHandler(service *service.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{service: service}
}

func (h *ApplicationHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var req model.CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Provider == "" {
		req.Provider = "email"
	}

	app, err := h.service.Submit(r.Context(), &req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already in use") {
			respondServiceError(w, http.StatusConflict, err)
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"application_id": app.ApplicationID,
		"user_id":        app.UserID,
		"status":         app.Status,
		"message":        "Application submitted. Your account is pending admin approval.",
	})
}

func (h *ApplicationHandler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 20)
	if limit > MaxLimit {
		limit = MaxLimit
	}

	filters := model.ListApplicationsFilters{
		Role:   strings.TrimSpace(r.URL.Query().Get("role")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Search: strings.TrimSpace(r.URL.Query().Get("q")),
		Page:   page,
		Limit:  limit,
	}

	apps, total, err := h.service.List(r.Context(), filters)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	totalPages := (total + limit - 1) / limit
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"applications": apps,
		"pagination": map[string]interface{}{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

func (h *ApplicationHandler) GetAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	app, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "application not found")
			return
		}
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) UpdateStatusAdmin(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "invalid application id")
		return
	}
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "invalid actor")
		return
	}

	var req model.UpdateApplicationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	app, err := h.service.UpdateStatus(r.Context(), id, actorID, req)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "invalid status") {
			respondServiceError(w, http.StatusBadRequest, err)
			return
		}
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "application not found")
			return
		}
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, app)
}

func parsePositiveInt(input string, fallback int) int {
	v, err := strconv.Atoi(input)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
