package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

// AvailabilityHandler answers the public availability check used by the chat
// agent (Ansu): "can a therapist serve this date/time?" It intentionally
// exposes only a boolean + a human note — never therapist identities.
type AvailabilityHandler struct {
	matchingService            service.TherapistMatchingService
	bookingAvailabilityService *service.BookingAvailabilityService
}

// NewAvailabilityHandler creates a new AvailabilityHandler.
func NewAvailabilityHandler(
	matchingService service.TherapistMatchingService,
	bookingAvailabilityService *service.BookingAvailabilityService,
) *AvailabilityHandler {
	return &AvailabilityHandler{
		matchingService:            matchingService,
		bookingAvailabilityService: bookingAvailabilityService,
	}
}

// availabilityResponse is the shape the agent's booking client expects.
type availabilityResponse struct {
	Available bool   `json:"available"`
	Note      string `json:"note"`
}

// CheckAvailability handles GET /api/v1/availability?date=YYYY-MM-DD&time=HH:MM.
// Public, read-only. The `address` param is accepted but unused: coverage lives
// in the agent's own business profile, not here (add a coverage gate if the web
// app reuses this endpoint).
func (h *AvailabilityHandler) CheckAvailability(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	start, ok := parseSlotStart(q.Get("date"), q.Get("time"))
	if !ok {
		respondError(w, http.StatusBadRequest, "date (YYYY-MM-DD) and time (HH:MM, 24h) are required")
		return
	}

	available, err := h.matchingService.IsSlotAvailable(r.Context(), start)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check availability")
		return
	}

	note := "A therapist is available for your requested time."
	if !available {
		note = "Fully booked around that time — staff can suggest the nearest open slot."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(availabilityResponse{Available: available, Note: note})
}

func (h *AvailabilityHandler) CheckBookingAvailability(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	defer r.Body.Close()
	var req service.BookingAvailabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.bookingAvailabilityService.Check(r.Context(), userID, &req)
	if err != nil {
		var validationErr *service.ValidationError
		if errors.As(err, &validationErr) {
			respondServiceError(w, http.StatusBadRequest, validationErr)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to check booking availability")
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// manilaLoc is the business timezone; bookings are scheduled in PH local time.
var manilaLoc = mustLoadManila()

func mustLoadManila() *time.Location {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		return time.FixedZone("PHT", 8*60*60) // ponytail: fallback if tzdata missing
	}
	return loc
}

// parseSlotStart combines a YYYY-MM-DD date and HH:MM time into a Manila-local
// instant. Returns false on malformed or empty input.
func parseSlotStart(date, clock string) (time.Time, bool) {
	if date == "" || clock == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", date+" "+clock, manilaLoc)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
