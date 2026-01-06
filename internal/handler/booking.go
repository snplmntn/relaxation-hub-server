package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type BookingHandler struct {
	bookingService *service.BookingService
	paymentService *service.PaymentService
	serviceRepo    repository.ServiceRepository
	addressRepo    repository.AddressRepository
	therapistRepo  repository.TherapistRepository
	storageService service.StorageService
}

func NewBookingHandler(bookingService *service.BookingService, paymentService *service.PaymentService, serviceRepo repository.ServiceRepository, addressRepo repository.AddressRepository, therapistRepo repository.TherapistRepository, storageService service.StorageService) *BookingHandler {
	return &BookingHandler{bookingService: bookingService, paymentService: paymentService, serviceRepo: serviceRepo, addressRepo: addressRepo, therapistRepo: therapistRepo, storageService: storageService}
}

func (h *BookingHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	req, err := parseCreateBookingRequest(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, err := h.bookingService.Create(r.Context(), clientID, &req, &clientID)
	if err != nil {
		// Handle structured validation errors from service
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

func (h *BookingHandler) ListBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	role, _ := middleware.GetUserRole(r)

	// Parse pagination parameters (default: page=1, limit=50)
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
	offset := (page - 1) * limit

	// Use paginated queries that return all data with count
	var results []repository.BookingDetailsResult
	var total int
	var err error

	if role == "therapist" {
		results, total, err = h.bookingService.ListByTherapistWithDetailsPaginated(r.Context(), userID, limit, offset)
	} else if role == "admin" {
		results, total, err = h.bookingService.ListAllWithDetailsPaginated(r.Context(), limit, offset)
	} else {
		results, total, err = h.bookingService.ListByClientWithDetailsPaginated(r.Context(), userID, limit, offset)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format - all data already available from JOINs
	bookings := make([]model.BookingResponse, 0, len(results))
	for _, r := range results {
		bookings = append(bookings, toBookingResponse(
			r.Booking,
			r.Service,
			r.Address,
			nil, // Payment
			r.TherapistName,
			r.TherapistPhone,
			r.TherapistPhoto,
			r.TherapistGender,
			r.TherapistRating,
			r.ClientName,
			r.ClientPhone,
			r.ClientPhoto,
			r.ClientGender,
			r.PromoCode,
		))
	}

	// Calculate pagination metadata
	totalPages := (total + limit - 1) / limit
	hasMore := page < totalPages

	response := model.PaginatedBookingsResponse{
		Bookings:   bookings,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *BookingHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	
	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	var booking *model.Booking
	var events []model.BookingEvent
	var service *model.Service
	var address *model.Address
	var tName, tPhone, tPhoto, tGender string
	var tRating *float64
	var cName, cPhone, cPhoto, cGender string
	var promoCode string
	actorRole, _ := middleware.GetUserRole(r)
	var err error

	// Try parsing as numeric ID
	if bookingID, parseErr := strconv.ParseInt(idParam, 10, 64); parseErr == nil {
		booking, events, service, address, tName, tPhone, tPhoto, tGender, tRating, cName, cPhone, cPhoto, cGender, promoCode, err = h.bookingService.GetBookingWithTimeline(r.Context(), bookingID, clientID, actorRole)
	} else {
		// Treat as reference code
		booking, events, service, address, tName, tPhone, tPhoto, tGender, tRating, cName, cPhone, cPhoto, cGender, promoCode, err = h.bookingService.GetBookingByCodeWithTimeline(r.Context(), idParam, clientID, actorRole)
	}

	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch payment info
	payment, _ := h.paymentService.GetByBookingID(r.Context(), booking.BookingID) // ignore error (might not exist)

	resp := toBookingResponse(booking, service, address, payment, tName, tPhone, tPhoto, tGender, tRating, cName, cPhone, cPhoto, cGender, promoCode)
	resp.Timeline = events

	// Presign payment proof URL if it exists (to avoid S3 CORS/403 errors)
	if resp.Payment != nil && resp.Payment.ProofURL != nil && *resp.Payment.ProofURL != "" {
		proofKey := extractS3Key(*resp.Payment.ProofURL)
		if proofKey != "" {
			if presignedURL, err := h.storageService.GetPresignedURL(r.Context(), proofKey, 15*time.Minute); err == nil {
				resp.Payment.ProofURL = &presignedURL
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *BookingHandler) AcceptOffer(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	therapistID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	if err := h.bookingService.AcceptBookingOffer(r.Context(), therapistID, bookingID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func (h *BookingHandler) DeclineOffer(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	therapistID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	if err := h.bookingService.DeclineBookingOffer(r.Context(), therapistID, bookingID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "declined"})
}


func (h *BookingHandler) UpdateBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req model.UpdateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, err := h.bookingService.Update(r.Context(), bookingID, clientID, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

func (h *BookingHandler) UpdateBookingStatus(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req model.UpdateBookingStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	log.Printf("DEBUG: UpdateBookingStatus: bookingID=%d, actorID=%d, role=%s, status=%s", bookingID, actorID, role, req.Status)

	booking, err := h.bookingService.UpdateStatus(r.Context(), bookingID, actorID, role, &req)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

// AssignTherapist allows admin to assign a therapist to a booking manually.
func (h *BookingHandler) AssignTherapist(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var payload struct {
		TherapistID int64 `json:"therapist_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, err := h.bookingService.AssignTherapist(r.Context(), bookingID, actorID, payload.TherapistID)
	if err != nil {
		// Map repository sentinel errors to HTTP-friendly responses
		switch err {
		case pgx.ErrNoRows:
			respondError(w, http.StatusNotFound, "booking not found or already assigned")
			return
		case repository.ErrTherapistNotFound:
			respondValidation(w, http.StatusBadRequest, "invalid_therapist", "specified therapist not found", map[string]string{"therapist_id": "not found"})
			return
		case repository.ErrTherapistNotAccepting:
			respondValidation(w, http.StatusBadRequest, "therapist_not_accepting", "therapist is not accepting assignments", map[string]string{"therapist_id": "accept_assignments = false"})
			return
		case repository.ErrAlreadyAssigned:
			respondValidation(w, http.StatusConflict, "cannot_assign", "therapist already assigned", map[string]string{"therapist_id": "already assigned"})
			return
		case repository.ErrBookingNotAssignable:
			respondValidation(w, http.StatusConflict, "cannot_assign", "booking not in assignable state", map[string]string{"booking_id": "not assignable"})
			return
		case repository.ErrAssignConflict:
			respondValidation(w, http.StatusConflict, "cannot_assign", "assignment failed due to concurrent change", map[string]string{"therapist_id": "race"})
			return
		default:
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	var tName, tPhone, tPhoto, tGender string
	var tRating *float64
	if booking.TherapistID != nil {
		tName, tPhone, tPhoto, tGender, tRating = h.bookingService.FetchTherapistInfo(r.Context(), booking.TherapistID)
	}

	w.Header().Set("Content-Type", "application/json")
	// Fetch client details
	cName, cPhone, cPhoto, cGender := h.bookingService.FetchClientInfo(r.Context(), booking.ClientID)
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, tName, tPhone, tPhoto, tGender, tRating, cName, cPhone, cPhoto, cGender, ""))
}

// AdminCreateBooking allows admins to create a booking on behalf of a client.
func (h *BookingHandler) AdminCreateBooking(w http.ResponseWriter, r *http.Request) {
	clientIDPtr, req, err := parseAdminCreateBookingRequest(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// ensure authenticated (role middleware on route should enforce admin)
	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	// prefer explicit client id from body when present, otherwise require it downstream
	clientID := int64(0)
	if clientIDPtr != nil {
		clientID = *clientIDPtr
	}
	booking, err := h.bookingService.CreateForAdmin(r.Context(), actorID, clientID, req)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

// StartBooking is called by client to start the session. Server enforces
// that therapist has arrived before allowing the transition to in_progress.
// Optionally accepts start_time in body for offline sync scenarios.
func (h *BookingHandler) StartBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	// Parse optional start_time from request body (for offline sync)
	var startTime *time.Time
	if r.Body != nil && r.ContentLength > 0 {
		var body struct {
			StartTime string `json:"start_time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.StartTime != "" {
			if parsed, err := time.Parse(time.RFC3339, body.StartTime); err == nil {
				startTime = &parsed
			}
		}
	}

	booking, err := h.bookingService.StartSession(r.Context(), bookingID, actorID, role, startTime)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// include timeline and client info
	_, events, _, _, _, _, _, _, _, cName, cPhone, cPhoto, cGender, promoCode, _ := h.bookingService.GetBookingWithTimeline(r.Context(), bookingID, actorID, role)
	payment, _ := h.paymentService.GetByBookingID(r.Context(), bookingID)
	resp := toBookingResponse(booking, nil, nil, payment, "", "", "", "", nil, cName, cPhone, cPhoto, cGender, promoCode)
	if events != nil {
		resp.Timeline = events
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PauseBooking allows a therapist to pause an in-progress session.
func (h *BookingHandler) PauseBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	booking, err := h.bookingService.PauseSession(r.Context(), bookingID, actorID, role)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

// ResumeBooking allows a therapist to resume a paused session.
func (h *BookingHandler) ResumeBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	booking, err := h.bookingService.ResumeSession(r.Context(), bookingID, actorID, role)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

// ExtendBooking allows client/therapist to extend an in-progress session.
// For clients, this creates an extension REQUEST that therapist must approve.
// For therapists/admins, this directly extends the session.
func (h *BookingHandler) ExtendBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	// Parse request body
	var body struct {
		AdditionalMinutes int `json:"additional_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Clients create an extension REQUEST (pending therapist approval)
	// Therapists and admins directly extend the session
	if role == "client" {
		request, err := h.bookingService.RequestExtension(r.Context(), bookingID, actorID, role, body.AdditionalMinutes)
		if err != nil {
			if ve, ok := err.(*service.ValidationError); ok {
				respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
				return
			}
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":            "pending",
			"message":           "Extension request created. Awaiting therapist approval.",
			"request_id":        request.RequestID,
			"requested_minutes": request.RequestedMinutes,
			"additional_cost":   request.AdditionalCost,
		})
		return
	}

	// Therapists and admins can directly extend
	booking, err := h.bookingService.ExtendSession(r.Context(), bookingID, actorID, role, body.AdditionalMinutes)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

// AdminListPendingBookings returns all bookings with pending status and no therapist assigned.

func (h *BookingHandler) AdminListPendingBookings(w http.ResponseWriter, r *http.Request) {
	bookings, err := h.bookingService.ListPendingBookings(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]model.BookingResponse, 0, len(bookings))
	for _, b := range bookings {
		out = append(out, toBookingResponse(&b, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// AdminGetBookingOffers returns all offers associated with a booking.
func (h *BookingHandler) AdminGetBookingOffers(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	offers, err := h.bookingService.GetOffersForBooking(r.Context(), bookingID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(offers)
}

// AdminGetBookingCandidates returns available therapist candidates for a booking.
func (h *BookingHandler) AdminGetBookingCandidates(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	candidates, err := h.bookingService.GetCandidatesForBooking(r.Context(), bookingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(candidates)
}

func parseBookingID(r *http.Request) (int64, error) {
	idParam := chi.URLParam(r, "id")
	return strconv.ParseInt(idParam, 10, 64)
}

// parseCreateBookingRequest reads a request body and tolerantly parses numeric
// fields that may be sent as strings (e.g. "service_id": "20") and accepts
// either `scheduled_at` or `scheduled_start` as the schedule field.
func parseCreateBookingRequest(body io.Reader) (model.CreateBookingRequest, error) {
	var req model.CreateBookingRequest
	data, err := io.ReadAll(body)
	if err != nil {
		return req, err
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return req, err
	}

	// helper to parse int64 from number or string
	parseInt64 := func(key string) (*int64, error) {
		if raw, ok := m[key]; ok {
			// try as number
			var n int64
			if err := json.Unmarshal(raw, &n); err == nil {
				return &n, nil
			}
			// try as string
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				if s == "" {
					return nil, nil
				}
				if v, err := strconv.ParseInt(s, 10, 64); err == nil {
					return &v, nil
				}
				if f, err := strconv.ParseFloat(s, 64); err == nil {
					v := int64(f)
					return &v, nil
				}
			}
		}
		return nil, nil
	}

	// helper to parse float64 from number or string
	parseFloat64 := func(key string) (*float64, error) {
		if raw, ok := m[key]; ok {
			var f float64
			if err := json.Unmarshal(raw, &f); err == nil {
				return &f, nil
			}
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				if s == "" {
					return nil, nil
				}
				if v, err := strconv.ParseFloat(s, 64); err == nil {
					return &v, nil
				}
			}
		}
		return nil, nil
	}

	// helper for string fields
	parseString := func(key string) string {
		if raw, ok := m[key]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				return s
			}
		}
		return ""
	}

	// fill fields
	if v, _ := parseInt64("therapist_id"); v != nil {
		req.TherapistID = v
	}
	if v, _ := parseInt64("service_id"); v != nil {
		req.ServiceID = v
	}
	if v, _ := parseInt64("address_id"); v != nil {
		req.AddressID = v
	}
	if v, _ := parseInt64("promo_id"); v != nil {
		req.PromoID = v
	}

	// duration
	if raw, ok := m["duration_minutes"]; ok {
		var dm int
		if err := json.Unmarshal(raw, &dm); err == nil {
			req.DurationMinutes = dm
		} else {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				if s != "" {
					if v, err := strconv.Atoi(s); err == nil {
						req.DurationMinutes = v
					}
				}
			}
		}
	}

	// scheduled: accept scheduled_at or scheduled_start
	if raw, ok := m["scheduled_at"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			req.ScheduledStart = s
		}
	} else if raw, ok := m["scheduled_start"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			req.ScheduledStart = s
		}
	}

	req.GenderPref = parseString("gender_preference")
	req.PressurePref = parseString("pressure_preference")
	req.Notes = parseString("notes")
	req.PaymentMethod = parseString("payment_method")
	req.VoucherCode = parseString("voucher_code")

	if f, _ := parseFloat64("raw_total"); f != nil {
		req.RawTotal = f
	}
	if f, _ := parseFloat64("discount"); f != nil {
		req.Discount = f
	}
	if f, _ := parseFloat64("total"); f != nil {
		req.Total = f
	}

	return req, nil
}

// parseAdminCreateBookingRequest extracts optional client_id and the
// same tolerant CreateBookingRequest from the provided body.
func parseAdminCreateBookingRequest(body io.Reader) (*int64, *model.CreateBookingRequest, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, err
	}

	var clientID *int64
	if raw, ok := m["client_id"]; ok {
		var n int64
		if err := json.Unmarshal(raw, &n); err == nil {
			clientID = &n
		} else {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				if s != "" {
					if v, err := strconv.ParseInt(s, 10, 64); err == nil {
						clientID = &v
					}
				}
			}
		}
	}

	// reuse parseCreateBookingRequest but it expects an io.Reader; give it the
	// original bytes via a reader
	req, err := parseCreateBookingRequest(io.NopCloser(bytesNewReader(data)))
	if err != nil {
		return nil, nil, err
	}
	return clientID, &req, nil
}

// minimal bytes reader wrapper to avoid importing bytes package globally
type bytesReader struct{ b []byte }
func bytesNewReader(b []byte) io.ReadCloser { return io.NopCloser(&bytesReader{b: b}) }
func (r *bytesReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

func toBookingResponse(b *model.Booking, service *model.Service, address *model.Address, payment *model.Payment, therapistName, therapistPhone, therapistPhoto, therapistGender string, therapistRating *float64, clientName, clientPhone, clientPhoto, clientGender, promoCode string) model.BookingResponse {
	out := model.BookingResponse{
		BookingID:       b.BookingID,
		ReferenceCode:   b.ReferenceCode,
		ClientID:        b.ClientID,
		TherapistID:     b.TherapistID,
		AssignedAt:      b.AssignedAt,
		ServiceID:       b.ServiceID,
		Service:         service,
		AddressID:       b.AddressID,
		Address:         address,
		PromoID:         b.PromoID,
		PromoCode:       promoCode,
		PaymentMethod:   b.PaymentMethod,
		GenderPref:      b.GenderPref,
		PressurePref:    b.PressurePref,
		Notes:           b.Notes,
		DurationMinutes: b.DurationMinutes,
		ScheduledStart:  b.ScheduledStart,
		ActualStart:     b.ActualStart,
		ActualEnd:       b.ActualEnd,
		TherapistArrivedAt: b.TherapistArrivedAt,
		CancelledBy:     b.CancelledBy,
		CancelledAt:     b.CancelledAt,
		CancellationReason: b.CancellationReason,
		RawTotal:        b.RawTotal,
		Discount:        b.Discount,
		FinalTotal:      b.FinalTotal,
		Status:          b.Status,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
		ServerTime:      time.Now().UTC(),
		IsRated:         b.IsRated,
		TotalPausedSeconds: b.TotalPausedSeconds,
		CurrentPauseStart: b.CurrentPauseStart,
		ExtensionWaitSeconds: b.ExtensionWaitSeconds,
		// Populate structured Client object
		Client: &model.ClientInfo{
			ClientID: b.ClientID,
			Name:     clientName,
			Phone:    clientPhone,
			Photo:    clientPhoto,
			Gender:   clientGender,
		},
	}

	// Populate Payment
	if payment != nil {
		out.Payment = &model.PaymentResponse{
			PaymentID:     payment.PaymentID,
			BookingID:     payment.BookingID,
			Amount:        payment.Amount,
			Gateway:       payment.Gateway,
			TransactionID: payment.TransactionID,
			Status:        payment.Status,
			WebhookID:     payment.WebhookID,
			ProofURL:      payment.ProofURL,
			VerifiedAt:    payment.VerifiedAt,
			VerifiedBy:    payment.VerifiedBy,
			TransactionAt: payment.TransactionAt,
			PaidAt:        payment.PaidAt,
			RefundedAt:    payment.RefundedAt,
			CreatedAt:     payment.CreatedAt,
			UpdatedAt:     payment.UpdatedAt,
		}
	}

	// Populate structured Therapist object if therapist info is available
	if b.TherapistID != nil && therapistName != "" {
		out.Therapist = &model.TherapistInfo{
			TherapistID: *b.TherapistID,
			Name:         therapistName,
			Phone:        therapistPhone,
			Photo:        therapistPhoto,
			Gender:       therapistGender,
			Rating:       therapistRating,
		}
	}

	// Populate PaymentBreakdown if available
	if len(b.PaymentBreakdownJSON) > 0 {
		var breakdown model.PaymentBreakdown
		if json.Unmarshal(b.PaymentBreakdownJSON, &breakdown) == nil {
			out.PaymentBreakdown = &breakdown
		}
	}
	
	return out
}

func (h *BookingHandler) UploadPaymentProof(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	// Verify storage is configured
	if h.storageService == nil || !h.storageService.IsConfigured() {
		respondError(w, http.StatusInternalServerError, "storage not configured")
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid form data")
		return
	}

	file, header, err := r.FormFile("proof_file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing proof_file")
		return
	}
	defer file.Close()

	// Generate storage key
	key := h.storageService.GenerateKey(fmt.Sprintf("payment-proofs/booking_%d", bookingID), header.Filename)

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Check for existing payment proof to clean up later (avoid orphans)
	var existingProofURL string
	if existingPayment, err := h.paymentService.GetByBookingID(r.Context(), bookingID); err == nil && existingPayment != nil {
		if existingPayment.ProofURL != nil && *existingPayment.ProofURL != "" {
			existingProofURL = *existingPayment.ProofURL
		}
	}

	// Upload to storage
	proofURL, err := h.storageService.UploadFile(r.Context(), key, file, contentType)
	if err != nil {
		log.Printf("Storage upload error: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to upload file")
		return
	}

	// Attempt cleanup of old file if upload succeeded
	if existingProofURL != "" {
		if oldKey := extractS3Key(existingProofURL); oldKey != "" {
			// Best effort deletion, log error if fails but don't block
			if err := h.storageService.DeleteFile(r.Context(), oldKey); err != nil {
				log.Printf("Failed to delete old proof file %s: %v", oldKey, err)
			}
		}
	}

	// Role-based booking lookup:
	// - Admin: can upload for any booking (use unsafe lookup)
	// - Therapist: can upload for their assigned booking
	// - Client: can upload for their own booking
	var booking *model.Booking
	if role == "admin" {
		booking, err = h.bookingService.GetByBookingID(r.Context(), bookingID)
	} else {
		// For therapist and client, use the user-scoped lookup
		booking, err = h.bookingService.GetByID(r.Context(), bookingID, actorID)
	}
	if err != nil {
		respondError(w, http.StatusNotFound, "booking not found or access denied")
		return
	}

	// Additional check for therapist: must be assigned to this booking
	if role == "therapist" && (booking.TherapistID == nil || *booking.TherapistID != actorID) {
		respondError(w, http.StatusForbidden, "therapist not assigned to this booking")
		return
	}

	amount := 0.0
	if booking.FinalTotal != nil {
		amount = *booking.FinalTotal
	}

	// Store proof in payments table
	// PaymentService.UploadProof will create a payment record if one doesn't exist
	if _, err := h.paymentService.UploadProof(r.Context(), bookingID, proofURL, amount, "manual"); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "uploaded",
		"proof_url":  proofURL,
	})
}

// CancelPaymentProof allows a user (client/admin) to cancel/remove their uploaded payment proof.
// This is only allowed if the payment is verified/rejected/pending?
// User request: "cancel the payment proof submitted... delete in s3, and be able to upload again"
func (h *BookingHandler) CancelPaymentProof(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	// Authorization Check
	if role == "client" {
		// Ensure client owns the booking
		booking, err := h.bookingService.GetByID(r.Context(), bookingID, actorID)
		if err != nil {
			respondError(w, http.StatusNotFound, "booking not found or access denied")
			return
		}
		if booking.ClientID != actorID {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}
	} else if role == "therapist" {
		// Ensure therapist is assigned to the booking
		booking, err := h.bookingService.GetByBookingID(r.Context(), bookingID)
		if err != nil {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		if booking.TherapistID == nil {
			respondError(w, http.StatusForbidden, "therapist not assigned to this booking (nil)")
			return
		}
		if *booking.TherapistID != actorID {
			respondError(w, http.StatusForbidden, fmt.Sprintf("therapist not assigned to this booking (mismatch: booking=%d, actor=%d)", *booking.TherapistID, actorID))
			return
		}
	} else if role != "admin" {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	// Get existing payment
	payment, err := h.paymentService.GetByBookingID(r.Context(), bookingID)
	if err != nil {
		respondError(w, http.StatusNotFound, "payment record not found")
		return
	}

	// Cleanup S3 file
	if payment.ProofURL != nil && *payment.ProofURL != "" {
		if key := extractS3Key(*payment.ProofURL); key != "" {
			if err := h.storageService.DeleteFile(r.Context(), key); err != nil {
				log.Printf("Failed to delete proof file from storage: %v", err)
				// Continue to clear DB even if storage delete fails (avoid locking user)
			}
		}
	}

	// Clear Proof in DB
	if err := h.paymentService.ClearProof(r.Context(), bookingID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to clear proof record")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "cancelled",
		"message": "Payment proof cancelled and removed",
	})
}

// UnassignBooking allows a therapist to cancel their assignment.
// The booking is reset and re-queued for a new therapist.
func (h *BookingHandler) UnassignBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)
	
	// If admin, we need to fetch the booking to find out who the therapist is
	// to pass the correct ID to the service (which expects the assigned therapist ID).
	var targetTherapistID int64
	if role == "admin" {
		// Fetch booking to get assigned therapist
		booking, err := h.bookingService.GetByBookingID(r.Context(), bookingID)
		if err != nil {
			if err == pgx.ErrNoRows {
				respondError(w, http.StatusNotFound, "booking not found")
				return
			}
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if booking.TherapistID == nil {
			respondError(w, http.StatusBadRequest, "booking has no assigned therapist")
			return
		}
		targetTherapistID = *booking.TherapistID
	} else if role == "therapist" {
		targetTherapistID = userID
	} else {
		respondError(w, http.StatusForbidden, "only therapists or admins can unassign")
		return
	}

	// Parse optional reason from body
	var body struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	var reason *string
	if body.Reason != "" {
		reason = &body.Reason
	}

	if err := h.bookingService.UnassignTherapist(r.Context(), bookingID, targetTherapistID, reason); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "unassigned",
		"message": "Booking unassigned and re-queued for a new therapist",
	})
}

// AcceptExtensionRequest allows a therapist to accept a pending extension request.
func (h *BookingHandler) AcceptExtensionRequest(w http.ResponseWriter, r *http.Request) {
	requestIDStr := chi.URLParam(r, "requestId")
	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	// Parse optional note from body
	var body model.RespondExtensionRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	var note *string
	if body.Note != "" {
		note = &body.Note
	}

	booking, err := h.bookingService.AcceptExtension(r.Context(), requestID, actorID, role, note)
	if err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, nil, "", "", "", "", nil, "", "", "", "", ""))
}

// RejectExtensionRequest allows a therapist to reject a pending extension request.
func (h *BookingHandler) RejectExtensionRequest(w http.ResponseWriter, r *http.Request) {
	requestIDStr := chi.URLParam(r, "requestId")
	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	// Parse optional note from body
	var body model.RespondExtensionRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	var note *string
	if body.Note != "" {
		note = &body.Note
	}

	if err := h.bookingService.RejectExtension(r.Context(), requestID, actorID, role, note); err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "rejected",
		"message": "Extension request rejected",
	})
}

// CancelExtensionRequest allows a client to cancel their own pending extension request.
func (h *BookingHandler) CancelExtensionRequest(w http.ResponseWriter, r *http.Request) {
	requestIDStr := chi.URLParam(r, "requestId")
	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)

	if err := h.bookingService.CancelExtension(r.Context(), requestID, actorID, role); err != nil {
		if ve, ok := err.(*service.ValidationError); ok {
			respondValidation(w, http.StatusBadRequest, ve.Code, ve.Message, ve.Details)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "cancelled",
		"message": "Extension request cancelled",
	})
}

// GetPendingExtensionRequest returns the pending extension request for a booking.
func (h *BookingHandler) GetPendingExtensionRequest(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	ext, err := h.bookingService.GetPendingExtensionRequest(r.Context(), bookingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// No pending extension is not an error, just return null/empty
			w.WriteHeader(http.StatusNoContent)
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if ext == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ext)
}

// VerifyPayment allows therapist/admin to verify or reject a payment proof.
func (h *BookingHandler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	actorID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	// Parse request body
	var body struct {
		Approved bool    `json:"approved"`
		Note     *string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var payment *model.Payment
	if body.Approved {
		payment, err = h.paymentService.Verify(r.Context(), bookingID, actorID, body.Note)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		payment, err = h.paymentService.Reject(r.Context(), bookingID, actorID, body.Note)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	status := "paid"
	if !body.Approved {
		status = "rejected"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  status,
		"message": "Payment proof " + status,
		"payment": model.PaymentResponse{
			PaymentID:     payment.PaymentID,
			BookingID:     payment.BookingID,
			Amount:        payment.Amount,
			Gateway:       payment.Gateway,
			TransactionID: payment.TransactionID,
			Status:        payment.Status,
			WebhookID:     payment.WebhookID,
			ProofURL:      payment.ProofURL,
			VerifiedAt:    payment.VerifiedAt,
			VerifiedBy:    payment.VerifiedBy,
			Notes:         payment.Notes,
			TransactionAt: payment.TransactionAt,
			PaidAt:        payment.PaidAt,
			RefundedAt:    payment.RefundedAt,
			CreatedAt:     payment.CreatedAt,
			UpdatedAt:     payment.UpdatedAt,
		},
	})
}

// extractS3Key extracts the S3 key (object path) from a full S3 URL.
// Example: https://bucket.s3.region.amazonaws.com/payment-proofs/booking_123/file.jpg
// Returns: payment-proofs/booking_123/file.jpg
func extractS3Key(s3URL string) string {
	// Parse as URL
	parsed, err := url.Parse(s3URL)
	if err != nil {
		return ""
	}
	// Return the path without leading slash
	return strings.TrimPrefix(parsed.Path, "/")
}
