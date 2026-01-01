package handler

import (
	"encoding/json"
	"log"
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
	userRepo            repository.UserRepository
}

func NewReviewHandler(reviewService *service.ReviewService, clientReviewService *service.ClientReviewService, bookingRepo repository.BookingRepository, serviceRepo repository.ServiceRepository, userRepo repository.UserRepository) *ReviewHandler {
	return &ReviewHandler{reviewService: reviewService, clientReviewService: clientReviewService, bookingRepo: bookingRepo, serviceRepo: serviceRepo, userRepo: userRepo}
}

func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	var req model.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("CreateReview: failed to decode request body: %v", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.Printf("CreateReview: received request: booking_id=%d, therapist_rating=%d, service_rating=%d, platform_rating=%d",
		req.BookingID, req.TherapistRating, req.ServiceRating, req.PlatformRating)

	clientID, ok := middleware.GetUserID(r)
	if !ok {
		log.Printf("CreateReview: user not found in context")
		respondError(w, http.StatusUnauthorized, "user not found in context")
		return
	}

	log.Printf("CreateReview: clientID=%d, fetching booking_id=%d", clientID, req.BookingID)

	booking, err := h.bookingRepo.GetByID(r.Context(), req.BookingID, clientID)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("CreateReview: booking not found: booking_id=%d, client_id=%d", req.BookingID, clientID)
			respondError(w, http.StatusNotFound, "booking not found")
			return
		}
		log.Printf("CreateReview: error fetching booking: %v", err)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("CreateReview: found booking: id=%d, status=%s, therapist_id=%v", booking.BookingID, booking.Status, booking.TherapistID)

	rev, err := h.reviewService.Create(r.Context(), clientID, &req, booking)
	if err != nil {
		if err == service.ErrReviewExists {
			respondError(w, http.StatusConflict, "You have already reviewed this booking")
			return
		}
		log.Printf("CreateReview: reviewService.Create failed: %v", err)
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

	out := toReviewResponse(rev, svc, nil)

	log.Printf("CreateReview: success, review_id=%d", rev.ReviewID)
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
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	out := toReviewResponse(rev, nil, nil)
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

	rev, err := h.reviewService.GetByBookingID(r.Context(), bookingID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, http.StatusNotFound, "review not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := toReviewResponse(rev, nil, nil)
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

	reviews, total, err := h.reviewService.ListByClient(r.Context(), clientID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Batch fetch therapist info
	therapistIDs := make([]int64, 0)
	idMap := make(map[int64]bool)
	for _, rev := range reviews {
		if rev.TherapistID != 0 && !idMap[rev.TherapistID] {
			therapistIDs = append(therapistIDs, rev.TherapistID)
			idMap[rev.TherapistID] = true
		}
	}

	therapistMap := make(map[int64]*repository.UserInfo)
	if len(therapistIDs) > 0 && h.userRepo != nil {
		if infoMap, err := h.userRepo.GetUserInfoBatch(r.Context(), therapistIDs); err == nil {
			therapistMap = infoMap
		}
	}

	out := make([]model.ReviewResponse, 0, len(reviews))
	for i := range reviews {
		out = append(out, toReviewResponse(&reviews[i], nil, therapistMap[reviews[i].TherapistID]))
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

	reviews, total, err := h.reviewService.ListByTherapist(r.Context(), tid, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Batch fetch services to avoid N+1
	serviceIDs := make([]int64, 0)
	idMap := make(map[int64]bool)
	for _, rev := range reviews {
		if rev.ServiceID != 0 && !idMap[rev.ServiceID] {
			serviceIDs = append(serviceIDs, rev.ServiceID)
			idMap[rev.ServiceID] = true
		}
	}

	serviceMap := make(map[int64]*model.Service)
	if len(serviceIDs) > 0 && h.serviceRepo != nil {
		services, err := h.serviceRepo.GetByIDs(r.Context(), serviceIDs)
		if err == nil {
			for i := range services {
				serviceMap[services[i].ServiceID] = &services[i]
			}
		}
	}

	out := make([]model.ReviewResponse, 0, len(reviews))
	for i := range reviews {
		out = append(out, toReviewResponse(&reviews[i], serviceMap[reviews[i].ServiceID], nil))
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

func toReviewResponse(rw *model.Review, svc *model.Service, therapist *repository.UserInfo) model.ReviewResponse {
	resp := model.ReviewResponse{
		ReviewID:        rw.ReviewID,
		BookingID:       rw.BookingID,
		ClientID:        rw.ClientID,
		TherapistID:     rw.TherapistID,
		ServiceID:       rw.ServiceID,
		Service:         svc,
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
