package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type AdminActionHandler struct {
	adminActionService *service.AdminActionService
}

func NewAdminActionHandler(adminActionService *service.AdminActionService) *AdminActionHandler {
	return &AdminActionHandler{adminActionService: adminActionService}
}

func (h *AdminActionHandler) LogAction(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	var req model.CreateAdminActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	action, err := h.adminActionService.Log(r.Context(), adminID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toAdminActionResponse(action))
}

func (h *AdminActionHandler) GetMyActions(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := DefaultAdminActionsLimit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > MaxLimit {
				limit = MaxLimit
			} else {
				limit = l
			}
		}
	}

	actions, err := h.adminActionService.GetByAdmin(r.Context(), adminID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var resp []model.AdminActionResponse
	for _, a := range actions {
		resp = append(resp, toAdminActionResponse(&a))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminActionHandler) GetAllActions(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := DefaultAdminActionsLimit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > MaxLimit {
				limit = MaxLimit
			} else {
				limit = l
			}
		}
	}

	actions, err := h.adminActionService.GetAll(r.Context(), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var resp []model.AdminActionResponse
	for _, a := range actions {
		resp = append(resp, toAdminActionResponse(&a))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func toAdminActionResponse(a *model.AdminAction) model.AdminActionResponse {
	return model.AdminActionResponse{
		ActionID:    a.ActionID,
		AdminID:     a.AdminID,
		ActionType:  a.ActionType,
		TargetType:  a.TargetType,
		TargetID:    a.TargetID,
		Description: a.Description,
		OldValue:    a.OldValue,
		NewValue:    a.NewValue,
		IPAddress:   a.IPAddress,
		PerformedAt: a.PerformedAt,
	}
}
