package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type StaffAttendanceHandler struct {
	staffAttendanceService service.StaffAttendanceService
}

func NewStaffAttendanceHandler(staffAttendanceService service.StaffAttendanceService) *StaffAttendanceHandler {
	return &StaffAttendanceHandler{staffAttendanceService: staffAttendanceService}
}

func (h *StaffAttendanceHandler) ListStaffAttendance(w http.ResponseWriter, r *http.Request) {
	workDate, ok := parseRequiredReportDate(w, r, "work_date")
	if !ok {
		return
	}
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	if role != "" && !validStaffAttendanceRole(role) {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	items, err := h.staffAttendanceService.ListStaffAttendance(r.Context(), model.StaffAttendanceFilter{
		WorkDate: workDate,
		Role:     role,
		Search:   strings.TrimSpace(r.URL.Query().Get("search")),
	})
	if err != nil {
		http.Error(w, "Failed to list attendance", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Data []model.StaffAttendance `json:"data"`
	}{Data: items})
}

func (h *StaffAttendanceHandler) ListStaffAttendanceAdminTargets(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 500 {
			http.Error(w, "Invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	items, err := h.staffAttendanceService.ListStaffAttendanceAdminTargets(
		r.Context(),
		strings.TrimSpace(r.URL.Query().Get("search")),
		limit,
	)
	if err != nil {
		http.Error(w, "Failed to list attendance staff", http.StatusInternalServerError)
		return
	}
	writeReportJSON(w, http.StatusOK, struct {
		Data []model.StaffAttendanceUser `json:"data"`
	}{Data: items})
}

func (h *StaffAttendanceHandler) CreateStaffAttendance(w http.ResponseWriter, r *http.Request) {
	attendance, ok := decodeStaffAttendanceRequest(w, r, 0)
	if !ok {
		return
	}
	actorID, _ := middleware.GetUserID(r)
	actorRole, _ := middleware.GetUserRole(r)
	created, err := h.staffAttendanceService.CreateStaffAttendance(r.Context(), attendance, actorID, actorRole)
	if err != nil {
		writeStaffAttendanceError(w, err, "Failed to create attendance")
		return
	}
	writeReportJSON(w, http.StatusCreated, created)
}

func (h *StaffAttendanceHandler) UpdateStaffAttendance(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	attendance, ok := decodeStaffAttendanceRequest(w, r, id)
	if !ok {
		return
	}
	actorID, _ := middleware.GetUserID(r)
	actorRole, _ := middleware.GetUserRole(r)
	updated, err := h.staffAttendanceService.UpdateStaffAttendance(r.Context(), attendance, actorID, actorRole)
	if err != nil {
		writeStaffAttendanceError(w, err, "Failed to update attendance")
		return
	}
	writeReportJSON(w, http.StatusOK, updated)
}

func (h *StaffAttendanceHandler) DeleteStaffAttendance(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r, "id")
	if !ok {
		return
	}
	actorID, _ := middleware.GetUserID(r)
	actorRole, _ := middleware.GetUserRole(r)
	if err := h.staffAttendanceService.VoidStaffAttendance(r.Context(), id, actorID, actorRole); err != nil {
		writeStaffAttendanceError(w, err, "Failed to void attendance")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeStaffAttendanceRequest(w http.ResponseWriter, r *http.Request, attendanceID int64) (model.StaffAttendance, bool) {
	var req model.StaffAttendanceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return model.StaffAttendance{}, false
	}
	workDate, err := time.Parse("2006-01-02", req.WorkDate)
	if err != nil || req.UserID <= 0 {
		http.Error(w, "Invalid attendance payload", http.StatusBadRequest)
		return model.StaffAttendance{}, false
	}
	timeInAt, ok := parseOptionalAttendanceTime(w, req.TimeInAt, "time_in_at")
	if !ok {
		return model.StaffAttendance{}, false
	}
	timeOutAt, ok := parseOptionalAttendanceTime(w, req.TimeOutAt, "time_out_at")
	if !ok {
		return model.StaffAttendance{}, false
	}
	actorID, _ := middleware.GetUserID(r)
	return model.StaffAttendance{
		AttendanceID: attendanceID,
		UserID:       req.UserID,
		WorkDate:     workDate,
		Date:         req.WorkDate,
		TimeInAt:     timeInAt,
		TimeOutAt:    timeOutAt,
		Notes:        strings.TrimSpace(req.Notes),
		CreatedBy:    &actorID,
		UpdatedBy:    &actorID,
	}, true
}

func parseOptionalAttendanceTime(w http.ResponseWriter, value string, field string) (*time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		http.Error(w, "Invalid "+field+" format (RFC3339)", http.StatusBadRequest)
		return nil, false
	}
	return &parsed, true
}

func writeStaffAttendanceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		http.Error(w, "Attendance not found", http.StatusNotFound)
	case errors.Is(err, model.ErrStaffAttendanceSelfEditForbidden):
		respondServiceError(w, http.StatusForbidden, err)
	case errors.Is(err, model.ErrStaffAttendanceLocked), errors.Is(err, model.ErrStaffAttendanceDuplicate):
		respondServiceError(w, http.StatusConflict, err)
	case errors.Is(err, model.ErrInvalidStaffAttendanceTargetRole),
		errors.Is(err, model.ErrStaffAttendanceOutsideWorkDateWindow),
		errors.Is(err, model.ErrStaffAttendanceTimeOutBeforeTimeIn),
		errors.Is(err, model.ErrStaffAttendanceShiftTooLong):
		respondClientError(w, http.StatusBadRequest, err)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func validStaffAttendanceRole(role string) bool {
	switch role {
	case model.RoleTherapist, model.RoleRider, model.RoleAdmin:
		return true
	default:
		return false
	}
}
