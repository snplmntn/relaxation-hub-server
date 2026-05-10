package service

import (
	"context"
	"fmt"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// ReferralService interface defines referral-related operations
type ReferralService interface {
	Create(ctx context.Context, referrerID int64, req *model.CreateReferralRequest) (*model.Referral, error)
	GetByCode(ctx context.Context, code string) (*model.Referral, error)
	GetByReferrer(ctx context.Context, referrerID int64) ([]model.Referral, error)
	CompleteReferral(ctx context.Context, referralID int64) error
	CompleteReferralByCode(ctx context.Context, code string, referredID int64) error
	GetRewardsByUser(ctx context.Context, userID int64) ([]model.ReferralReward, error)
	RedeemReward(ctx context.Context, rewardID, userID int64) error
}

type referralService struct {
	repo repository.ReferralRepository
}

func NewReferralService(repo repository.ReferralRepository) ReferralService {
	return &referralService{repo: repo}
}

func (s *referralService) Create(ctx context.Context, referrerID int64, req *model.CreateReferralRequest) (*model.Referral, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.ReferredID == 0 {
		return nil, fmt.Errorf("referred_id is required")
	}
	if referrerID == req.ReferredID {
		return nil, fmt.Errorf("cannot refer yourself")
	}

	code := repository.GenerateReferralCode()
	ref := &model.Referral{
		ReferrerID:   referrerID,
		ReferredID:   req.ReferredID,
		ReferralCode: code,
		Status:       "pending",
	}

	if err := s.repo.Create(ctx, ref); err != nil {
		return nil, err
	}
	return ref, nil
}

func (s *referralService) GetByCode(ctx context.Context, code string) (*model.Referral, error) {
	if code == "" {
		return nil, fmt.Errorf("referral code is required")
	}
	return s.repo.GetByCode(ctx, code)
}

func (s *referralService) GetByReferrer(ctx context.Context, referrerID int64) ([]model.Referral, error) {
	return s.repo.GetByReferrer(ctx, referrerID)
}

func (s *referralService) CompleteReferral(ctx context.Context, referralID int64) error {
	if err := s.repo.UpdateStatus(ctx, referralID, "completed"); err != nil {
		return err
	}

	ref, err := s.repo.GetByID(ctx, referralID)
	if err != nil {
		return err
	}

	reward := &model.ReferralReward{
		ReferralID:   referralID,
		UserID:       ref.ReferrerID,
		RewardType:   "discount",
		RewardAmount: 100.00,
		Status:       "pending",
		ExpiresAt:    timePtr(time.Now().AddDate(0, 3, 0)),
	}

	return s.repo.CreateReward(ctx, reward)
}

// CompleteReferralByCode completes a referral when a new user signs up with a referral code
// This is called during signup when referral_code is provided
func (s *referralService) CompleteReferralByCode(ctx context.Context, code string, referredID int64) error {
	if code == "" {
		return fmt.Errorf("referral code is required")
	}
	if referredID <= 0 {
		return fmt.Errorf("invalid referred user id")
	}

	// Get the referral by code
	ref, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return fmt.Errorf("invalid referral code: %w", err)
	}

	// Verify the referred user matches
	if ref.ReferredID != referredID {
		return fmt.Errorf("referral code does not match user")
	}

	// Update referral status to completed
	if err := s.repo.UpdateStatus(ctx, ref.ReferralID, "completed"); err != nil {
		return fmt.Errorf("failed to complete referral: %w", err)
	}

	// Create reward for referrer
	reward := &model.ReferralReward{
		ReferralID:   ref.ReferralID,
		UserID:       ref.ReferrerID,
		RewardType:   "discount",
		RewardAmount: 100.00, // 100 PHP discount
		Status:       "pending",
		ExpiresAt:    timePtr(time.Now().AddDate(0, 3, 0)), // 3 months expiry
	}

	if err := s.repo.CreateReward(ctx, reward); err != nil {
		return fmt.Errorf("failed to create reward: %w", err)
	}

	return nil
}

func (s *referralService) GetRewardsByUser(ctx context.Context, userID int64) ([]model.ReferralReward, error) {
	return s.repo.GetRewardsByUser(ctx, userID)
}

func (s *referralService) RedeemReward(ctx context.Context, rewardID, userID int64) error {
	return s.repo.RedeemReward(ctx, rewardID, userID)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
