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
	return model.BranchResponse{
		BranchID:    b.BranchID,
		BranchName:  b.BranchName,
		AddressLine: b.AddressLine,
		Barangay:    b.Barangay,
		City:        b.City,
		Province:    b.Province,
		PostalCode:  b.PostalCode,
		Latitude:    b.Latitude,
		Longitude:   b.Longitude,
		ContactNo:   b.ContactNo,
		Email:       b.Email,
		IsActive:    b.IsActive,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}
