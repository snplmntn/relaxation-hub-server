package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type SupportTicketHandler struct {
	ticketService *service.SupportTicketService
}

func NewSupportTicketHandler(ticketService *service.SupportTicketService) *SupportTicketHandler {
	return &SupportTicketHandler{ticketService: ticketService}
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
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// CreateTicket handles the submission of a support ticket with optional attachments.
// It expects a multipart/form-data request.
func (h *SupportTicketHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	// 1. Get User ID from middleware
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	// 2. Parse Multipart Form (32 MB max memory)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
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
	// For MVP, we save to local disk "uploads/tickets/{timestamp}_{filename}"
	// In prod, this should go to S3/GCS.
	files := r.MultipartForm.File["attachments"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue // skip unreadable files
		}
		defer file.Close()

		// Ensure uploads dir exists
		uploadDir := "uploads/tickets"
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to create upload dir")
			return
		}

		// Sanitize filename
		timestamp := time.Now().UnixNano()
		cleanName := filepath.Base(fileHeader.Filename)
		cleanName = strings.ReplaceAll(cleanName, " ", "_")
		destPath := fmt.Sprintf("%s/%d_%s", uploadDir, timestamp, cleanName)

		dst, err := os.Create(destPath)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save file")
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			continue
		}

		// URL accessible by client (needs static file serving setup in main.go)
		// For now, assuming server serves /uploads
		publicURL := "/" + destPath // e.g., /uploads/tickets/123_img.jpg
		fileURLs = append(fileURLs, publicURL)
	}

	// 5. Call Service
	ticket, err := h.ticketService.Create(r.Context(), userID, req, fileURLs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ticket)
}
