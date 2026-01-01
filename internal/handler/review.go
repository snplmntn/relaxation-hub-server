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
	reviewService       *service.ReviewService
	clientReviewService *service.ClientReviewService
	bookingRepo         repository.BookingRepository
	serviceRepo         repository.ServiceRepository
}

func NewReviewHandler(reviewService *service.ReviewService, clientReviewService *service.ClientReviewService, bookingRepo repository.BookingRepository, serviceRepo repository.ServiceRepository) *ReviewHandler {
	return &ReviewHandler{reviewService: reviewService, clientReviewService: clientReviewService, bookingRepo: bookingRepo, serviceRepo: serviceRepo}
}

func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	var req model.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	booking, err := h.bookingRepo.GetByID(r.Context(), req.BookingID, clientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	rev, err := h.reviewService.Create(r.Context(), clientID, &req, booking)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Try to enrich with service details
	var svc *model.Service
	if rev.ServiceID != 0 && h.serviceRepo != nil {
		if s, serr := h.serviceRepo.GetByID(r.Context(), rev.ServiceID); serr == nil {
			svc = s
		}
	}

	out := model.ReviewResponse{
		ReviewID:        rev.ReviewID,
		BookingID:       rev.BookingID,
		ClientID:        rev.ClientID,
		TherapistID:     rev.TherapistID,
		ServiceID:       rev.ServiceID,
		Service:         svc,
		TherapistRating: rev.TherapistRating,
		TherapistReview: rev.TherapistReview,
		ServiceRating:   rev.ServiceRating,
		ServiceReview:   rev.ServiceReview,
		PlatformRating:  rev.PlatformRating,
		PlatformReview:  rev.PlatformReview,
		CreatedAt:       rev.CreatedAt,
		UpdatedAt:       rev.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(out)
}

func (h *ReviewHandler) CreateClientReview(w http.ResponseWriter, r *http.Request) {
	if h.clientReviewService == nil {
		respondError(w, http.StatusInternalServerError, "client review service not configured")
		return
	}

	var req model.CreateClientReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	therapistID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	role, _ := middleware.GetUserRole(r)
	if role != "therapist" {
		respondError(w, http.StatusForbidden, "only therapists can review clients")
		return
	}

	booking, err := h.bookingRepo.GetByBookingID(r.Context(), req.BookingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	review, err := h.clientReviewService.Create(r.Context(), therapistID, &req, booking)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := toClientReviewResponse(review)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *ReviewHandler) ListClientReviews(w http.ResponseWriter, r *http.Request) {
	if h.clientReviewService == nil {
		respondError(w, http.StatusInternalServerError, "client review service not configured")
		return
	}

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	role, _ := middleware.GetUserRole(r)
	if role != "client" {
		respondError(w, http.StatusForbidden, "only clients can view client reviews")
		return
	}

	reviews, err := h.clientReviewService.ListByClient(r.Context(), clientID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]model.ClientReviewResponse, 0, len(reviews))
	for i := range reviews {
		out = append(out, toClientReviewResponse(&reviews[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *ReviewHandler) ListReviewsForTherapist(w http.ResponseWriter, r *http.Request) {
	therapistIDStr := chi.URLParam(r, "therapist_id")
	tid, err := strconv.ParseInt(therapistIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid therapist id")
		return
	}

	reviews, err := h.reviewService.ListByTherapist(r.Context(), tid)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]model.ReviewResponse, 0, len(reviews))
	for i := range reviews {
		var svc *model.Service
		if reviews[i].ServiceID != 0 && h.serviceRepo != nil {
			if s, serr := h.serviceRepo.GetByID(r.Context(), reviews[i].ServiceID); serr == nil {
				svc = s
			}
		}
		out = append(out, model.ReviewResponse{
			ReviewID:        reviews[i].ReviewID,
			BookingID:       reviews[i].BookingID,
			ClientID:        reviews[i].ClientID,
			TherapistID:     reviews[i].TherapistID,
			ServiceID:       reviews[i].ServiceID,
			Service:         svc,
			TherapistRating: reviews[i].TherapistRating,
			TherapistReview: reviews[i].TherapistReview,
			ServiceRating:   reviews[i].ServiceRating,
			ServiceReview:   reviews[i].ServiceReview,
			PlatformRating:  reviews[i].PlatformRating,
			PlatformReview:  reviews[i].PlatformReview,
			CreatedAt:       reviews[i].CreatedAt,
			UpdatedAt:       reviews[i].UpdatedAt,
		})
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

func toClientReviewResponse(rw *model.ClientReview) model.ClientReviewResponse {
	return model.ClientReviewResponse{
		ClientReviewID: rw.ClientReviewID,
		BookingID:      rw.BookingID,
		TherapistID:    rw.TherapistID,
		ClientID:       rw.ClientID,
		ClientRating:   rw.ClientRating,
		ClientReview:   rw.ClientReview,
		CreatedAt:      rw.CreatedAt,
		UpdatedAt:      rw.UpdatedAt,
	}
}
