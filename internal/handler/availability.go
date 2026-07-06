package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

const (
	defaultAvailabilityDurationMin = 60
	minAvailabilityDurationMin     = 30
	maxAvailabilityDurationMin     = 240
	defaultAvailabilityQuantity    = 1
	minAvailabilityQuantity        = 1
	maxAvailabilityQuantity        = 10
	maxAvailabilityAlternatives    = 3
)

// AvailabilityHandler answers the public availability check used by the chat
// agent (Ansu): "can a therapist serve this date/time?" It intentionally
// exposes only a boolean + a human note — never therapist identities.
type AvailabilityHandler struct {
	matchingService service.TherapistMatchingService
}

// NewAvailabilityHandler creates a new AvailabilityHandler.
func NewAvailabilityHandler(matchingService service.TherapistMatchingService) *AvailabilityHandler {
	return &AvailabilityHandler{matchingService: matchingService}
}

// availabilityResponse is the shape the agent's booking client expects.
type availabilityResponse struct {
	Available    bool                              `json:"available"`
	Note         string                            `json:"note"`
	Alternatives []availabilityAlternativeResponse `json:"alternatives,omitempty"`
}

type availabilityAlternativeResponse struct {
	Date  string `json:"date"`
	Time  string `json:"time"`
	Label string `json:"label,omitempty"`
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
	durationMin, ok := parseAvailabilityInt(
		q.Get("duration_min"),
		defaultAvailabilityDurationMin,
		minAvailabilityDurationMin,
		maxAvailabilityDurationMin,
	)
	if !ok {
		respondError(w, http.StatusBadRequest, "duration_min must be between 30 and 240")
		return
	}
	quantity, ok := parseAvailabilityInt(
		q.Get("quantity"),
		defaultAvailabilityQuantity,
		minAvailabilityQuantity,
		maxAvailabilityQuantity,
	)
	if !ok {
		respondError(w, http.StatusBadRequest, "quantity must be between 1 and 10")
		return
	}

	available, err := h.matchingService.IsSlotAvailable(r.Context(), start, durationMin, quantity)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check availability")
		return
	}

	var alternatives []availabilityAlternativeResponse
	note := "A therapist is available for your requested time."
	if !available {
		slots, err := h.matchingService.FindAlternativeSlots(
			r.Context(),
			start,
			durationMin,
			quantity,
			maxAvailabilityAlternatives,
		)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to find alternative slots")
			return
		}
		alternatives = formatAvailabilityAlternatives(slots)
		if len(alternatives) > 0 {
			note = "Fully booked around that time. Offer the returned alternatives and ask which one the customer prefers."
		} else {
			note = "Fully booked around that time — ask the customer for another preferred time or wider time window."
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(availabilityResponse{
		Available:    available,
		Note:         note,
		Alternatives: alternatives,
	})
}

func formatAvailabilityAlternatives(slots []service.AvailabilitySlot) []availabilityAlternativeResponse {
	alternatives := make([]availabilityAlternativeResponse, 0, len(slots))
	for _, slot := range slots {
		local := slot.Start.In(manilaLoc)
		alternatives = append(alternatives, availabilityAlternativeResponse{
			Date:  local.Format("2006-01-02"),
			Time:  local.Format("15:04"),
			Label: local.Format("3:04 PM"),
		})
	}
	return alternatives
}

func parseAvailabilityInt(raw string, fallback, min, max int) (int, bool) {
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, false
	}
	return value, true
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
