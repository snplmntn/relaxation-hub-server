package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type TherapistHandler struct {
	therapistService *service.TherapistService
	storageService   service.StorageService
}

func NewTherapistHandler(therapistService *service.TherapistService, storageService service.StorageService) *TherapistHandler {
	return &TherapistHandler{therapistService: therapistService, storageService: storageService}
}

func (h *TherapistHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid therapist id")
		return
	}

	profile, err := h.therapistService.GetProfile(r.Context(), therapistID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "therapist not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTherapistProfileResponse(profile))
}

func (h *TherapistHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.UpdateTherapistProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := h.therapistService.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "therapist profile not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTherapistProfileResponse(profile))
}

func (h *TherapistHandler) AdminUpdateProfile(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid therapist id")
		return
	}

	var req model.UpdateTherapistProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := h.therapistService.UpdateProfile(r.Context(), therapistID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "therapist profile not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toTherapistProfileResponse(profile))
}

func (h *TherapistHandler) AdminUpdateServices(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid therapist id")
		return
	}

	var req []model.AddServiceWithPressuresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.therapistService.BatchUpdateServices(r.Context(), therapistID, req); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TherapistHandler) ListTherapists(w http.ResponseWriter, r *http.Request) {
	availableOnlyStr := r.URL.Query().Get("available")
	availableOnly := availableOnlyStr == "true"

	profiles, err := h.therapistService.List(r.Context(), availableOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
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
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	// Check if this is a multipart form (file upload) or JSON (URL only)
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && len(contentType) >= 19 && contentType[:19] == "multipart/form-data" {
		// Handle file upload
		if h.storageService == nil || !h.storageService.IsConfigured() {
			respondError(w, http.StatusInternalServerError, "storage not configured")
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			respondError(w, http.StatusBadRequest, "invalid form data")
			return
		}

		file, header, err := r.FormFile("document")
		if err != nil {
			respondError(w, http.StatusBadRequest, "missing document file")
			return
		}
		defer file.Close()

		docType := r.FormValue("document_type")
		if docType == "" {
			respondError(w, http.StatusBadRequest, "document_type is required")
			return
		}

		// Generate storage key
		key := h.storageService.GenerateKey(fmt.Sprintf("documents/therapist_%d", userID), header.Filename)

		// Determine content type
		fileContentType := header.Header.Get("Content-Type")
		if fileContentType == "" {
			fileContentType = mime.TypeByExtension(filepath.Ext(header.Filename))
		}
		if fileContentType == "" {
			fileContentType = "application/octet-stream"
		}

		// Upload to storage
		docURL, err := h.storageService.UploadFile(r.Context(), key, file, fileContentType)
		if err != nil {
			slog.Warn("storage upload error", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to upload document")
			return
		}

		req := &model.UploadDocumentRequest{
			DocumentType: docType,
			DocumentURL:  docURL,
		}

		doc, err := h.therapistService.UploadDocument(r.Context(), userID, req)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toTherapistDocumentResponse(doc))
		return
	}

	// Handle JSON request (URL provided directly)
	var req model.UploadDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	doc, err := h.therapistService.UploadDocument(r.Context(), userID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
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
		respondError(w, http.StatusBadRequest, "invalid therapist id")
		return
	}

	// Security: IDOR Check
	requestingUserID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	requestingUserRole, _ := middleware.GetUserRole(r)

	// Allow if admin OR if requesting their own documents
	if requestingUserRole != "admin" && requestingUserID != therapistID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	docs, err := h.therapistService.GetDocuments(r.Context(), therapistID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
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
		respondError(w, http.StatusBadRequest, "invalid document id")
		return
	}

	verifierID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.VerifyDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.therapistService.VerifyDocument(r.Context(), documentID, verifierID, &req); err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "document not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TherapistHandler) AddService(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var req model.AddServiceWithPressuresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.therapistService.AddService(r.Context(), userID, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TherapistHandler) RemoveService(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	serviceIDStr := chi.URLParam(r, "service_id")
	serviceID, err := strconv.ParseInt(serviceIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	if err := h.therapistService.RemoveService(r.Context(), userID, serviceID); err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "service not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TherapistHandler) GetServices(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "id")
	therapistID, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid therapist id")
		return
	}

	svcMap, err := h.therapistService.GetServicesWithPressures(r.Context(), therapistID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// respond with array of { service_id, pressures }
	var resp []map[string]interface{}
	for sid, pressures := range svcMap {
		resp = append(resp, map[string]interface{}{"service_id": sid, "pressures": pressures})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]map[string]interface{}{"services": resp})
}

// CheckInAtBranch allows therapist to mark themselves as returned to branch.
// POST /api/v1/therapist/check-in/branch
func (h *TherapistHandler) CheckInAtBranch(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	if err := h.therapistService.SetAtBranch(r.Context(), userID, true); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Checked in at branch",
		"at_branch": true,
	})
}

func toTherapistProfileResponse(tp *model.TherapistProfile) model.TherapistProfileResponse {
	return model.TherapistProfileResponse{
		TherapistID:       tp.TherapistID,
		FullName:          tp.FullName,
		BranchID:          tp.BranchID,
		Bio:               tp.Bio,
		Specialization:    tp.Specialization,
		YearsExperience:   tp.YearsExperience,
		AvgRating:         tp.AvgRating,
		TotalReviews:      tp.TotalReviews,
		TotalBookings:     tp.TotalBookings,
		IsVerified:        tp.IsVerified,
		AcceptAssignments: tp.AcceptAssignments,
		CreatedAt:         tp.CreatedAt,
		UpdatedAt:         tp.UpdatedAt,
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
