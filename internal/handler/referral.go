package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type ReferralHandler struct {
	referralService service.ReferralService
}

func NewReferralHandler(referralService service.ReferralService) *ReferralHandler {
	return &ReferralHandler{referralService: referralService}
}

func (h *ReferralHandler) CreateReferral(w http.ResponseWriter, r *http.Request) {
    var req model.CreateReferralRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    userID, ok := middleware.GetUserID(r)
    if !ok {
        http.Error(w, "user not found in context", http.StatusUnauthorized)
        return
    }

    ref, err := h.referralService.Create(r.Context(), userID, &req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(toReferralResponse(ref))
}

func (h *ReferralHandler) GetReferralByCode(w http.ResponseWriter, r *http.Request) {
    code := r.URL.Query().Get("code")
    if code == "" {
        http.Error(w, "code query parameter is required", http.StatusBadRequest)
        return
    }

    ref, err := h.referralService.GetByCode(r.Context(), code)
    if err != nil {
        if err == pgx.ErrNoRows {
            http.Error(w, "referral not found", http.StatusNotFound)
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(toReferralResponse(ref))
}

func (h *ReferralHandler) ListReferrals(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserID(r)
    if !ok {
        http.Error(w, "user not found in context", http.StatusUnauthorized)
        return
    }

    refs, err := h.referralService.GetByReferrer(r.Context(), userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    var resp []model.ReferralResponse
    for _, ref := range refs {
        resp = append(resp, toReferralResponse(&ref))
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func (h *ReferralHandler) GetRewards(w http.ResponseWriter, r *http.Request) {
    userID, ok := middleware.GetUserID(r)
    if !ok {
        http.Error(w, "user not found in context", http.StatusUnauthorized)
        return
    }

    rewards, err := h.referralService.GetRewardsByUser(r.Context(), userID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    var resp []model.ReferralRewardResponse
    for _, rw := range rewards {
        resp = append(resp, toReferralRewardResponse(&rw))
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func (h *ReferralHandler) RedeemReward(w http.ResponseWriter, r *http.Request) {
    rewardIDStr := chi.URLParam(r, "reward_id")
    rewardID, err := strconv.ParseInt(rewardIDStr, 10, 64)
    if err != nil {
        http.Error(w, "invalid reward_id", http.StatusBadRequest)
        return
    }

    userID, ok := middleware.GetUserID(r)
    if !ok {
        http.Error(w, "user not found in context", http.StatusUnauthorized)
        return
    }

    if err := h.referralService.RedeemReward(r.Context(), rewardID, userID); err != nil {
        if err == pgx.ErrNoRows {
            http.Error(w, "reward not found or already redeemed", http.StatusNotFound)
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func toReferralResponse(r *model.Referral) model.ReferralResponse {
    return model.ReferralResponse{
        ReferralID:   r.ReferralID,
        ReferrerID:   r.ReferrerID,
        ReferredID:   r.ReferredID,
        ReferralCode: r.ReferralCode,
        Status:       r.Status,
        RewardEarned: r.RewardEarned,
        CreatedAt:    r.CreatedAt,
        CompletedAt:  r.CompletedAt,
    }
}

func toReferralRewardResponse(rw *model.ReferralReward) model.ReferralRewardResponse {
    return model.ReferralRewardResponse{
        RewardID:     rw.RewardID,
        ReferralID:   rw.ReferralID,
        UserID:       rw.UserID,
        RewardType:   rw.RewardType,
        RewardAmount: rw.RewardAmount,
        Status:       rw.Status,
        ExpiresAt:    rw.ExpiresAt,
        RedeemedAt:   rw.RedeemedAt,
        CreatedAt:    rw.CreatedAt,
    }
}
