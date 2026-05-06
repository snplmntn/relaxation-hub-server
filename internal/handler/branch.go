package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type BranchHandler struct {
	branchService *service.BranchService
}

func NewBranchHandler(branchService *service.BranchService) *BranchHandler {
	return &BranchHandler{branchService: branchService}
}

func (h *BranchHandler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	var req model.CreateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	branch, err := h.branchService.Create(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toBranchResponse(branch))
}

func (h *BranchHandler) GetBranch(w http.ResponseWriter, r *http.Request) {
	branchIDStr := chi.URLParam(r, "id")
	branchID, err := strconv.ParseInt(branchIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid branch id")
		return
	}

	branch, err := h.branchService.GetByID(r.Context(), branchID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "branch not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBranchResponse(branch))
}

func (h *BranchHandler) ListBranches(w http.ResponseWriter, r *http.Request) {
	activeOnlyStr := r.URL.Query().Get("active")
	activeOnly := activeOnlyStr == "true"

	branches, err := h.branchService.List(r.Context(), activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var resp []model.BranchResponse
	for _, b := range branches {
		resp = append(resp, toBranchResponse(&b))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ListPublicBranches returns branch options for unauthenticated application flows.
func (h *BranchHandler) ListPublicBranches(w http.ResponseWriter, r *http.Request) {
	activeOnly := true
	if activeOnlyStr := r.URL.Query().Get("active"); activeOnlyStr != "" {
		activeOnly = activeOnlyStr == "true"
	}

	branches, err := h.branchService.List(r.Context(), activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type branchOption struct {
		BranchID   int64  `json:"branch_id"`
		BranchName string `json:"branch_name"`
		City       string `json:"city"`
		Province   string `json:"province"`
		IsActive   bool   `json:"is_active"`
	}

	resp := make([]branchOption, 0, len(branches))
	for _, b := range branches {
		city := ""
		if b.City != nil {
			city = *b.City
		}
		province := ""
		if b.Province != nil {
			province = *b.Province
		}
		isActive := false
		if b.IsActive != nil {
			isActive = *b.IsActive
		}
		resp = append(resp, branchOption{
			BranchID:   b.BranchID,
			BranchName: b.BranchName,
			City:       city,
			Province:   province,
			IsActive:   isActive,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *BranchHandler) UpdateBranch(w http.ResponseWriter, r *http.Request) {
	branchIDStr := chi.URLParam(r, "id")
	branchID, err := strconv.ParseInt(branchIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid branch id")
		return
	}

	var req model.UpdateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	branch, err := h.branchService.Update(r.Context(), branchID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "branch not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBranchResponse(branch))
}

func toBranchResponse(b *model.Branch) model.BranchResponse {
	addressLine := ""
	if b.AddressLine != nil {
		addressLine = *b.AddressLine
	}
	city := ""
	if b.City != nil {
		city = *b.City
	}
	province := ""
	if b.Province != nil {
		province = *b.Province
	}
	isActive := false
	if b.IsActive != nil {
		isActive = *b.IsActive
	}

	return model.BranchResponse{
		BranchID:    b.BranchID,
		BranchName:  b.BranchName,
		AddressLine: addressLine,
		Barangay:    b.Barangay,
		City:        city,
		Province:    province,
		PostalCode:  b.PostalCode,
		Latitude:    b.Latitude,
		Longitude:   b.Longitude,
		ContactNo:   b.ContactNo,
		Email:       b.Email,
		IsActive:    isActive,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}
