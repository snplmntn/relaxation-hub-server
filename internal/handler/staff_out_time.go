package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type StaffOutTimeHandler struct {
	staffOutTimeService service.StaffOutTimeService
}

func NewStaffOutTimeHandler(staffOutTimeService service.StaffOutTimeService) *StaffOutTimeHandler {
	return &StaffOutTimeHandler{staffOutTimeService: staffOutTimeService}
}

func (h *StaffOutTimeHandler) ListStaffOutTimes(w http.ResponseWriter, r *http.Request) {
	workDate, ok := parseRequiredReportDate(w, r, "work_date")
	if !ok {
		return
	}
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	if role != "" && !validStaffOutTimeRole(role) {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	items, err := h.staffOutTimeService.ListStaffOutTimes(r.Context(), model.StaffOutTimeFilter{
		WorkDate: workDate,
		Role:     role,
		Search:   strings.TrimSpace(r.URL.Query().Get("search")),
	})
	if err != nil {
		http.Error(w, "Failed to list out times", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Data []model.StaffOutTime `json:"data"`
	}{Data: items})
}

func (h *StaffOutTimeHandler) CreateStaffOutTime(w http.ResponseWriter, r *http.Request) {
	outTime, ok := decodeStaffOutTimeRequest(w, r, 0)
	if !ok {
		return
	}
	created, err := h.staffOutTimeService.CreateStaffOutTime(r.Context(), outTime)
	if err != nil {
		writeStaffOutTimeError(w, err, "Failed to create out time")
		return
	}
	writeReportJSON(w, http.StatusCreated, created)
}

func (h *StaffOutTimeHandler) UpdateStaffOutTime(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	outTime, ok := decodeStaffOutTimeRequest(w, r, id)
	if !ok {
		return
	}
	updated, err := h.staffOutTimeService.UpdateStaffOutTime(r.Context(), outTime)
	if err != nil {
		writeStaffOutTimeError(w, err, "Failed to update out time")
		return
	}
	writeReportJSON(w, http.StatusOK, updated)
}

func (h *StaffOutTimeHandler) DeleteStaffOutTime(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	actorID, _ := middleware.GetUserID(r)
	if err := h.staffOutTimeService.VoidStaffOutTime(r.Context(), id, actorID); err != nil {
		writeStaffOutTimeError(w, err, "Failed to void out time")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeStaffOutTimeRequest(w http.ResponseWriter, r *http.Request, outTimeID int64) (model.StaffOutTime, bool) {
	var req model.StaffOutTimeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return model.StaffOutTime{}, false
	}
	workDate, err := time.Parse("2006-01-02", req.WorkDate)
	if err != nil || req.UserID <= 0 {
		http.Error(w, "Invalid out time payload", http.StatusBadRequest)
		return model.StaffOutTime{}, false
	}
	outAt, err := time.Parse(time.RFC3339, req.OutAt)
	if err != nil {
		http.Error(w, "Invalid out_at format (RFC3339)", http.StatusBadRequest)
		return model.StaffOutTime{}, false
	}
	actorID, _ := middleware.GetUserID(r)
	return model.StaffOutTime{
		OutTimeID: outTimeID,
		UserID:    req.UserID,
		WorkDate:  workDate,
		Date:      req.WorkDate,
		OutAt:     outAt,
		Notes:     strings.TrimSpace(req.Notes),
		CreatedBy: &actorID,
		UpdatedBy: &actorID,
	}, true
}

func writeStaffOutTimeError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		http.Error(w, "Out time not found", http.StatusNotFound)
	case errors.Is(err, model.ErrInvalidStaffOutTimeTargetRole), errors.Is(err, model.ErrStaffOutTimeOutsideWorkDateWindow):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func validStaffOutTimeRole(role string) bool {
	switch role {
	case model.RoleTherapist, model.RoleRider, model.RoleAdmin, model.RoleSuperAdmin:
		return true
	default:
		return false
	}
}
