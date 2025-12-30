package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
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
	serviceRepo    repository.ServiceRepository
	addressRepo    repository.AddressRepository
	therapistRepo  repository.TherapistRepository
}

func NewBookingHandler(bookingService *service.BookingService, serviceRepo repository.ServiceRepository, addressRepo repository.AddressRepository, therapistRepo repository.TherapistRepository) *BookingHandler {
	return &BookingHandler{bookingService: bookingService, serviceRepo: serviceRepo, addressRepo: addressRepo, therapistRepo: therapistRepo}
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
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, "", nil, "", "", ""))
}

func (h *BookingHandler) ListBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	role, _ := middleware.GetUserRole(r)

	var bookings []model.Booking
	var err error

	if role == "therapist" {
		bookings, err = h.bookingService.ListByTherapist(r.Context(), userID)
	} else {
		bookings, err = h.bookingService.ListByClient(r.Context(), userID)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]model.BookingResponse, 0, len(bookings))
	for i := range bookings {
		var tName string
		var tRating *float64
		if bookings[i].TherapistID != nil {
			tName, tRating = h.bookingService.FetchTherapistInfo(r.Context(), bookings[i].TherapistID)
		}
		// Fetch client details
		cName, cPhone, cPhoto := h.bookingService.FetchClientInfo(r.Context(), bookings[i].ClientID)
		out = append(out, toBookingResponse(&bookings[i], nil, nil, tName, tRating, cName, cPhone, cPhoto))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *BookingHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseBookingID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, events, service, address, therapistName, therapistRating, err := h.bookingService.GetBookingWithTimeline(r.Context(), bookingID, clientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch client details
	cName, cPhone, cPhoto := h.bookingService.FetchClientInfo(r.Context(), booking.ClientID)

	resp := toBookingResponse(booking, service, address, therapistName, therapistRating, cName, cPhone, cPhoto)
	resp.Timeline = events
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
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, "", nil, "", "", ""))
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
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, "", nil, "", "", ""))
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

	var tName string
	var tRating *float64
	if booking.TherapistID != nil {
		tName, tRating = h.bookingService.FetchTherapistInfo(r.Context(), booking.TherapistID)
	}

	w.Header().Set("Content-Type", "application/json")
	// Fetch client details
	cName, cPhone, cPhoto := h.bookingService.FetchClientInfo(r.Context(), booking.ClientID)
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, tName, tRating, cName, cPhone, cPhoto))
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
	json.NewEncoder(w).Encode(toBookingResponse(booking, nil, nil, "", nil, "", "", ""))
}

// StartBooking is called by client to start the session. Server enforces
// that therapist has arrived before allowing the transition to in_progress.
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

	booking, err := h.bookingService.StartSession(r.Context(), bookingID, actorID, role)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	cName, cPhone, cPhoto := h.bookingService.FetchClientInfo(r.Context(), booking.ClientID)
	resp := toBookingResponse(booking, nil, nil, "", nil, cName, cPhone, cPhoto)
	// include timeline
	_, events, _, _, _, _, _ := h.bookingService.GetBookingWithTimeline(r.Context(), bookingID, actorID)
	if events != nil {
		resp.Timeline = events
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

func toBookingResponse(b *model.Booking, service *model.Service, address *model.Address, therapistName string, therapistRating *float64, clientName, clientPhone, clientPhoto string) model.BookingResponse {
	return model.BookingResponse{
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
		TherapistName:   therapistName,
		TherapistRating: therapistRating,
		ClientName:      clientName,
		ClientPhone:     clientPhone,
		ClientPhoto:     clientPhoto,
	}
}
