package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
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

type ReviewHandler struct {
	reviewService       *service.ReviewService
	clientReviewService *service.ClientReviewService
	bookingRepo         repository.BookingRepository
	serviceRepo         repository.ServiceRepository
	userRepo            repository.UserRepository
}

func NewReviewHandler(reviewService *service.ReviewService, clientReviewService *service.ClientReviewService, bookingRepo repository.BookingRepository, serviceRepo repository.ServiceRepository, userRepo repository.UserRepository) *ReviewHandler {
	return &ReviewHandler{reviewService: reviewService, clientReviewService: clientReviewService, bookingRepo: bookingRepo, serviceRepo: serviceRepo, userRepo: userRepo}
}

func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	var req model.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("CreateReview: failed to decode request body", "error", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	slog.Debug("CreateReview: received request", "booking_id", req.BookingID, "therapist_rating", req.TherapistRating, "service_rating", req.ServiceRating, "platform_rating", req.PlatformRating)

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		slog.Warn("CreateReview: user not found in context")
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}
	role, _ := middleware.GetUserRole(r)
	if role != model.RoleClient {
		respondError(w, http.StatusForbidden, "only clients can review bookings")
		return
	}

	slog.Debug("CreateReview: fetching booking", "client_id", clientID, "booking_id", req.BookingID)

	booking, err := h.bookingRepo.GetByID(r.Context(), req.BookingID, clientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			slog.Debug("CreateReview: booking not found", "booking_id", req.BookingID, "client_id", clientID)
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		slog.Warn("CreateReview: error fetching booking", "error", err)
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	if booking.ClientID != clientID {
		respondError(w, http.StatusForbidden, "you can only review your own booking")
		return
	}

	slog.Debug("CreateReview: found booking", "booking_id", booking.BookingID, "status", booking.Status, "therapist_id", booking.TherapistID)

	rev, err := h.reviewService.Create(r.Context(), clientID, &req, booking)
	if err != nil {
		if err == service.ErrReviewExists {
			respondError(w, http.StatusConflict, "You have already reviewed this booking")
			return
		}
		slog.Warn("CreateReview: reviewService.Create failed", "error", err)
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	// Try to enrich with service details
	var svc *model.Service
	if rev.ServiceID != 0 && h.serviceRepo != nil {
		if s, serr := h.serviceRepo.GetByID(r.Context(), rev.ServiceID); serr == nil {
			svc = s
		}
	}

	out := toReviewResponse(rev, svc, nil, nil)

	slog.Debug("CreateReview: success", "review_id", rev.ReviewID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(out)
}

func (h *ReviewHandler) UpdateReview(w http.ResponseWriter, r *http.Request) {
	reviewIDStr := chi.URLParam(r, "review_id")
	reviewID, err := strconv.ParseInt(reviewIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid review id")
		return
	}

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

	rev, err := h.reviewService.Update(r.Context(), clientID, reviewID, &req)
	if err != nil {
		if err == service.ErrEditPeriodExpired {
			respondServiceError(w, http.StatusForbidden, err)
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	out := toReviewResponse(rev, nil, nil, nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *ReviewHandler) GetReviewByBooking(w http.ResponseWriter, r *http.Request) {
	bookingIDStr := chi.URLParam(r, "booking_id")
	bookingID, err := strconv.ParseInt(bookingIDStr, 10, 64)
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
	if model.IsAdminRole(role) {
		if _, err := h.bookingRepo.GetByBookingID(r.Context(), bookingID); err != nil {
			if err == pgx.ErrNoRows {
				respondError(w, http.StatusNotFound, "booking not found")
				return
			}
			respondServiceError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		if _, err := h.bookingRepo.GetByID(r.Context(), bookingID, userID); err != nil {
			if err == pgx.ErrNoRows {
				respondError(w, http.StatusNotFound, "booking not found")
				return
			}
			respondServiceError(w, http.StatusInternalServerError, err)
			return
		}
	}

	rev, err := h.reviewService.GetByBookingID(r.Context(), bookingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "review not found")
			return
		}
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	out := toReviewResponse(rev, nil, nil, nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *ReviewHandler) ListMyReviews(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetUserID(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	page := 1
	limit := 20
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	offset := (page - 1) * limit

	results, total, err := h.reviewService.ListByClientWithDetails(r.Context(), clientID, limit, offset)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]model.ReviewResponse, 0, len(results))
	for _, res := range results {
		out = append(out, model.ReviewResponse{
			ReviewID:        res.Review.ReviewID,
			BookingID:       res.Review.BookingID,
			ClientID:        res.Review.ClientID,
			TherapistID:     res.Review.TherapistID,
			ServiceID:       res.Review.ServiceID,
			Service:         res.Service,
			BookingDate:     res.BookingDate,
			TherapistName:   res.TherapistName,
			TherapistPhoto:  res.TherapistPhoto,
			TherapistRating: res.Review.TherapistRating,
			TherapistReview: res.Review.TherapistReview,
			ServiceRating:   res.Review.ServiceRating,
			ServiceReview:   res.Review.ServiceReview,
			PlatformRating:  res.Review.PlatformRating,
			PlatformReview:  res.Review.PlatformReview,
			CreatedAt:       res.Review.CreatedAt,
			UpdatedAt:       res.Review.UpdatedAt,
		})
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}

	resp := model.PaginatedReviewsResponse{
		Reviews:    out,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	review, err := h.clientReviewService.Create(r.Context(), therapistID, &req, booking)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
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
		respondServiceError(w, http.StatusInternalServerError, err)
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

	// Default values for page and limit
	page := 1
	limit := 10

	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	offset := (page - 1) * limit

	results, total, err := h.reviewService.ListByTherapistWithDetails(r.Context(), tid, limit, offset)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]model.ReviewResponse, 0, len(results))
	for _, res := range results {
		out = append(out, model.ReviewResponse{
			ReviewID:        res.Review.ReviewID,
			BookingID:       res.Review.BookingID,
			ClientID:        res.Review.ClientID,
			TherapistID:     res.Review.TherapistID,
			ServiceID:       res.Review.ServiceID,
			Service:         res.Service,
			BookingDate:     res.BookingDate,
			TherapistName:   res.TherapistName,
			TherapistPhoto:  res.TherapistPhoto,
			ClientName:      res.ClientName,
			ClientPhoto:     res.ClientPhoto,
			TherapistRating: res.Review.TherapistRating,
			TherapistReview: res.Review.TherapistReview,
			ServiceRating:   res.Review.ServiceRating,
			ServiceReview:   res.Review.ServiceReview,
			PlatformRating:  res.Review.PlatformRating,
			PlatformReview:  res.Review.PlatformReview,
			CreatedAt:       res.Review.CreatedAt,
			UpdatedAt:       res.Review.UpdatedAt,
		})
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}

	resp := model.PaginatedReviewsResponse{
		Reviews:    out,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ReviewHandler) AdminListReviews(w http.ResponseWriter, r *http.Request) {
	page := 1
	limit := 20
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var therapistID *int64
	if therapistIDStr := strings.TrimSpace(r.URL.Query().Get("therapist_id")); therapistIDStr != "" {
		tid, err := strconv.ParseInt(therapistIDStr, 10, 64)
		if err != nil || tid <= 0 {
			respondError(w, http.StatusBadRequest, "invalid therapist_id")
			return
		}
		therapistID = &tid
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	minAvgRating := 0.0
	if minRatingStr := strings.TrimSpace(r.URL.Query().Get("min_avg_rating")); minRatingStr != "" {
		rating, err := strconv.ParseFloat(minRatingStr, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid min_avg_rating")
			return
		}
		if rating < 0 {
			rating = 0
		}
		if rating > 5 {
			rating = 5
		}
		minAvgRating = rating
	}

	results, total, err := h.reviewService.ListAllWithDetails(r.Context(), therapistID, search, minAvgRating, limit, offset)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	out := make([]model.ReviewResponse, 0, len(results))
	for _, res := range results {
		out = append(out, model.ReviewResponse{
			ReviewID:        res.Review.ReviewID,
			BookingID:       res.Review.BookingID,
			ClientID:        res.Review.ClientID,
			TherapistID:     res.Review.TherapistID,
			ServiceID:       res.Review.ServiceID,
			Service:         res.Service,
			BookingDate:     res.BookingDate,
			TherapistName:   res.TherapistName,
			TherapistPhoto:  res.TherapistPhoto,
			ClientName:      res.ClientName,
			ClientPhoto:     res.ClientPhoto,
			TherapistRating: res.Review.TherapistRating,
			TherapistReview: res.Review.TherapistReview,
			ServiceRating:   res.Review.ServiceRating,
			ServiceReview:   res.Review.ServiceReview,
			PlatformRating:  res.Review.PlatformRating,
			PlatformReview:  res.Review.PlatformReview,
			CreatedAt:       res.Review.CreatedAt,
			UpdatedAt:       res.Review.UpdatedAt,
		})
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}

	resp := model.PaginatedReviewsResponse{
		Reviews:    out,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
		HasMore:    page < totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func toReviewResponse(rw *model.Review, svc *model.Service, therapist *repository.UserInfo, bookingDate *time.Time) model.ReviewResponse {
	resp := model.ReviewResponse{
		ReviewID:        rw.ReviewID,
		BookingID:       rw.BookingID,
		ClientID:        rw.ClientID,
		TherapistID:     rw.TherapistID,
		ServiceID:       rw.ServiceID,
		Service:         svc,
		BookingDate:     bookingDate,
		TherapistRating: rw.TherapistRating,
		TherapistReview: rw.TherapistReview,
		ServiceRating:   rw.ServiceRating,
		ServiceReview:   rw.ServiceReview,
		PlatformRating:  rw.PlatformRating,
		PlatformReview:  rw.PlatformReview,
		CreatedAt:       rw.CreatedAt,
		UpdatedAt:       rw.UpdatedAt,
	}
	if therapist != nil {
		resp.TherapistName = therapist.Name
		resp.TherapistPhoto = therapist.Photo
	}
	return resp
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
