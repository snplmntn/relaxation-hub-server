package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

	type OffersHandler struct {
    bookingService *service.BookingService
}

func NewOffersHandler(bs *service.BookingService) *OffersHandler {
    return &OffersHandler{bookingService: bs}
}

// ListForTherapist returns active offers for a therapist (pending & unexpired)
// EnrichedOfferResponse includes the offer and the enriched booking
type EnrichedOfferResponse struct {
    Offer   model.BookingOffer      `json:"offer"`
    Booking model.BookingResponse   `json:"booking"`
}

func (h *OffersHandler) ListForTherapist(w http.ResponseWriter, r *http.Request) {
    tidStr := chi.URLParam(r, "id")
    tid, err := strconv.ParseInt(tidStr, 10, 64)
    if err != nil {
        respondError(w, http.StatusBadRequest, "invalid therapist id")
        return
    }

    offers, err := h.bookingService.ListOffersForTherapist(r.Context(), tid)
    if err != nil {
        respondError(w, http.StatusInternalServerError, err.Error())
        return
    }
    if offers == nil {
        offers = []model.BookingOffer{}
    }

    enriched := make([]EnrichedOfferResponse, 0, len(offers))
    for _, offer := range offers {
        // Fetch the enriched booking for each offer
        // Therapists are allowed to see bookings they have offers for
        booking, _, service, address, tName, tPhone, tPhoto, tGender, tRating, cName, cPhone, cPhoto, cGender, promoCode, err := h.bookingService.GetBookingWithTimeline(r.Context(), offer.BookingID, tid, "therapist")
        if err != nil || booking == nil {
            // If booking not found, skip enrichment but include offer
            enriched = append(enriched, EnrichedOfferResponse{Offer: offer})
            continue
        }
        
        resp := toBookingResponse(booking, service, address, tName, tPhone, tPhoto, tGender, tRating, cName, cPhone, cPhoto, cGender, promoCode)
        enriched = append(enriched, EnrichedOfferResponse{
            Offer:   offer,
            Booking: resp,
        })
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(enriched)
}
