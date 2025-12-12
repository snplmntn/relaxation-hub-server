package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type ReviewHandler struct {
	reviewService *service.ReviewService
	bookingRepo   repository.BookingRepository
}

func NewReviewHandler(reviewService *service.ReviewService, bookingRepo repository.BookingRepository) *ReviewHandler {
	return &ReviewHandler{reviewService: reviewService, bookingRepo: bookingRepo}
}

func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	var req model.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		http.Error(w, "user not found in context", http.StatusUnauthorized)
		return
	}

	booking, err := h.bookingRepo.GetByID(r.Context(), req.BookingID, clientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "booking not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rev, err := h.reviewService.Create(r.Context(), clientID, &req, booking)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toReviewResponse(rev))
}

func (h *ReviewHandler) ListReviewsForTherapist(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "therapist_id")
	tid, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid therapist id", http.StatusBadRequest)
		return
	}

	reviews, err := h.reviewService.ListByTherapist(r.Context(), tid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]model.ReviewResponse, 0, len(reviews))
	for i := range reviews {
		out = append(out, toReviewResponse(&reviews[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func toReviewResponse(rw *model.Review) model.ReviewResponse {
	return model.ReviewResponse{
		ReviewID:        rw.ReviewID,
		BookingID:       rw.BookingID,
		ClientID:        rw.ClientID,
		TherapistID:     rw.TherapistID,
		ServiceID:       rw.ServiceID,
		TherapistRating: rw.TherapistRating,
		TherapistReview: rw.TherapistReview,
		ServiceRating:   rw.ServiceRating,
		ServiceReview:   rw.ServiceReview,
		PlatformRating:  rw.PlatformRating,
		PlatformReview:  rw.PlatformReview,
		CreatedAt:       rw.CreatedAt,
		UpdatedAt:       rw.UpdatedAt,
	}
}
