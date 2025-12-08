package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type TherapistHandler struct {
	therapistService *service.TherapistService
}

func NewTherapistHandler(therapistService *service.TherapistService) *TherapistHandler {
	return &TherapistHandler{therapistService: therapistService}
}

func (h *TherapistHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid therapist id", http.StatusBadRequest)
		return
	}

	profile, err := h.therapistService.GetProfile(r.Context(), therapistID)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "therapist not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTherapistProfileResponse(profile))
}

func (h *TherapistHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	var req model.UpdateTherapistProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	profile, err := h.therapistService.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "therapist profile not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTherapistProfileResponse(profile))
}

func (h *TherapistHandler) ListTherapists(w http.ResponseWriter, r *http.Request) {
	availableOnlyStr := r.URL.Query().Get("available")
	availableOnly := availableOnlyStr == "true"

	profiles, err := h.therapistService.List(r.Context(), availableOnly)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var resp []model.TherapistProfileResponse
	for _, p := range profiles {
		resp = append(resp, toTherapistProfileResponse(&p))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *TherapistHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	var req model.UploadDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	doc, err := h.therapistService.UploadDocument(r.Context(), userID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toTherapistDocumentResponse(doc))
}

func (h *TherapistHandler) GetDocuments(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid therapist id", http.StatusBadRequest)
		return
	}

	docs, err := h.therapistService.GetDocuments(r.Context(), therapistID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var resp []model.TherapistDocumentResponse
	for _, d := range docs {
		resp = append(resp, toTherapistDocumentResponse(&d))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *TherapistHandler) VerifyDocument(w http.ResponseWriter, r *http.Request) {
	documentIDStr := chi.URLParam(r, "document_id")
	documentID, err := strconv.ParseInt(documentIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid document id", http.StatusBadRequest)
		return
	}

	verifierID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	var req model.VerifyDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.therapistService.VerifyDocument(r.Context(), documentID, verifierID, &req); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TherapistHandler) AddService(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	var req model.AddServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.therapistService.AddService(r.Context(), userID, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TherapistHandler) RemoveService(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	serviceIDStr := chi.URLParam(r, "service_id")
	serviceID, err := strconv.ParseInt(serviceIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return
	}

	if err := h.therapistService.RemoveService(r.Context(), userID, serviceID); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TherapistHandler) GetServices(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid therapist id", http.StatusBadRequest)
		return
	}

	serviceIDs, err := h.therapistService.GetServices(r.Context(), therapistID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]int64{"service_ids": serviceIDs})
}

func toTherapistProfileResponse(tp *model.TherapistProfile) model.TherapistProfileResponse {
	return model.TherapistProfileResponse{
		TherapistID:     tp.TherapistID,
		Bio:             tp.Bio,
		Specialization:  tp.Specialization,
		YearsExperience: tp.YearsExperience,
		AvgRating:       tp.AvgRating,
		TotalReviews:    tp.TotalReviews,
		TotalBookings:   tp.TotalBookings,
		IsVerified:      tp.IsVerified,
		IsAvailable:     tp.IsAvailable,
		CreatedAt:       tp.CreatedAt,
		UpdatedAt:       tp.UpdatedAt,
	}
}

func toTherapistDocumentResponse(doc *model.TherapistDocument) model.TherapistDocumentResponse {
	return model.TherapistDocumentResponse{
		DocumentID:   doc.DocumentID,
		TherapistID:  doc.TherapistID,
		DocumentType: doc.DocumentType,
		DocumentURL:  doc.DocumentURL,
		Status:       doc.Status,
		UploadedAt:   doc.UploadedAt,
		VerifiedAt:   doc.VerifiedAt,
		VerifiedBy:   doc.VerifiedBy,
	}
}
