package handler

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type SupportTicketHandler struct {
	ticketService  *service.SupportTicketService
	storageService service.StorageService
}

func NewSupportTicketHandler(ticketService *service.SupportTicketService, storageService service.StorageService) *SupportTicketHandler {
	return &SupportTicketHandler{ticketService: ticketService, storageService: storageService}
}

// ListTickets exposes admin listing with optional status filtering.
func (h *SupportTicketHandler) ListTickets(w http.ResponseWriter, r *http.Request) {
	page := 1
	limit := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	statusParam := r.URL.Query().Get("status")
	var status *string
	if statusParam != "" {
		status = &statusParam
	}

	result, err := h.ticketService.ListForAdmin(r.Context(), status, page, limit)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// ListMyTickets returns the authenticated user's own support tickets, or the
// full ticket list for operational admins using the same endpoint.
func (h *SupportTicketHandler) ListMyTickets(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	if role, _ := middleware.GetUserRole(r); model.IsAdminRole(role) {
		h.ListTickets(w, r)
		return
	}

	page := 1
	limit := 20
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	result, err := h.ticketService.ListForUser(r.Context(), userID, page, limit)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// CreateTicket handles the submission of a support ticket with optional attachments.
// It expects a multipart/form-data request.
func (h *SupportTicketHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	// 1. Get User ID from middleware
	// 1. Get User ID from middleware (optional)
	userID, _ := middleware.GetUserID(r)
	// If not authenticated, userID will be 0, which service handles as anonymous.

	// 2. Parse Multipart Form (32 MB max memory)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	// 3. Extract Fields
	category := r.FormValue("category")
	description := r.FormValue("description")
	contactEmailPhone := r.FormValue("contact_email_phone")
	bookingReferenceCode := r.FormValue("booking_reference_code")

	if category == "" || description == "" {
		respondError(w, http.StatusBadRequest, "category and description are required")
		return
	}

	var bookingID *int64
	if bookingReferenceCode != "" {
		id, err := h.ticketService.GetBookingIDByReferenceCode(r.Context(), bookingReferenceCode)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid booking reference code")
			return
		}
		bookingID = id
	}

	req := &model.CreateSupportTicketRequest{
		Category:          category,
		Description:       description,
		ContactEmailPhone: contactEmailPhone,
		BookingID:         bookingID,
	}

	// 4. Handle File Uploads
	var fileURLs []string
	if h.storageService != nil && h.storageService.IsConfigured() {
		files := r.MultipartForm.File["attachments"]
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue // skip unreadable files
			}
			defer file.Close()

			// Generate storage key
			key := h.storageService.GenerateKey("tickets", fileHeader.Filename)

			// Determine content type
			contentType := fileHeader.Header.Get("Content-Type")
			if contentType == "" {
				contentType = mime.TypeByExtension(filepath.Ext(fileHeader.Filename))
			}
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			// Upload to storage
			publicURL, err := h.storageService.UploadFile(r.Context(), key, file, contentType)
			if err != nil {
				fmt.Printf("Warning: failed to upload ticket attachment: %v\n", err)
				continue
			}
			fileURLs = append(fileURLs, publicURL)
		}
	}

	// 5. Call Service
	ticket, err := h.ticketService.Create(r.Context(), userID, req, fileURLs)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ticket)
}

func (h *SupportTicketHandler) UpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.ticketService.UpdateStatus(r.Context(), id, req.Status); err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		// If error contains "not found", return 404? Service currently wraps generic error.
		// Repo returns "ticket not found".
		if err.Error() == "failed to update ticket status: ticket not found" { // String matching is brittle but workable for now
			respondError(w, http.StatusNotFound, "ticket not found")
			return
		}
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"updated"}`))
}
