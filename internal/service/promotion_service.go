package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type PromotionService struct {
	repo repository.PromotionRepository
}

func NewPromotionService(repo repository.PromotionRepository) *PromotionService {
	return &PromotionService{repo: repo}
}

func (s *PromotionService) Create(ctx context.Context, req *model.CreatePromotionRequest) (*model.Promotion, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	// Validation: either percentage or amount must be set
	if req.DiscountPct <= 0 && req.DiscountAmount == nil {
		return nil, fmt.Errorf("either discount_percent or discount_amount is required")
	}
	if req.DiscountPct < 0 || req.DiscountPct > 100 {
		return nil, fmt.Errorf("discount_percent must be 0-100")
	}
	if req.DiscountAmount != nil && *req.DiscountAmount < 0 {
		return nil, fmt.Errorf("discount_amount must be positive")
	}

	// convert int to pointer
	pct := req.DiscountPct
	var discountPctPtr *int
	if pct > 0 {
		discountPctPtr = &pct
	}

	usage := 1
	if req.UsageLimit != nil {
		usage = *req.UsageLimit
		if usage < 1 {
			return nil, fmt.Errorf("usage_limit must be >= 1")
		}
	}

	parseTime := func(val *string) (*time.Time, error) {
		if val == nil || *val == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339, *val)
		if err != nil {
			return nil, fmt.Errorf("invalid time format: %w", err)
		}
		return &t, nil
	}

	validFrom, err := parseTime(req.ValidFrom)
	if err != nil {
		return nil, err
	}
	validUntil, err := parseTime(req.ValidUntil)
	if err != nil {
		return nil, err
	}
	startTime, err := parseTime(req.StartTime)
	if err != nil {
		return nil, err
	}
	endTime, err := parseTime(req.EndTime)
	if err != nil {
		return nil, err
	}

	p := &model.Promotion{
		Code:           code,
		DiscountPct:    discountPctPtr,
		DiscountAmount: req.DiscountAmount,
		ValidFrom:      validFrom,
		ValidUntil:  validUntil,
		UsageLimit:  usage,
		DaysOfWeek:  req.DaysOfWeek,
		StartTime:   startTime,
		EndTime:     endTime,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *PromotionService) ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error) {
	return s.repo.ListActive(ctx, now)
}

func (s *PromotionService) GetByCode(ctx context.Context, code string) (*model.Promotion, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	return s.repo.GetByCode(ctx, code)
}

// ValidationResult holds the output of a promo validation check.
type ValidationResult struct {
	Valid          bool     `json:"valid"`
	Code           string   `json:"code"`
	DiscountAmount float64  `json:"discount_amount"` // The calculated discount value
	Message        string   `json:"message"`
	Type           string   `json:"type"` // "fixed" or "percentage"
}

func (s *PromotionService) Validate(ctx context.Context, code string, amount float64) (*ValidationResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return &ValidationResult{Valid: false, Message: "Code required"}, nil
	}

	p, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return &ValidationResult{Valid: false, Code: code, Message: "Invalid code"}, nil
	}

	now := time.Now()
	if p.ValidFrom != nil && p.ValidFrom.After(now) {
		return &ValidationResult{Valid: false, Code: code, Message: "Promotion not yet active"}, nil
	}
	if p.ValidUntil != nil && p.ValidUntil.Before(now) {
		return &ValidationResult{Valid: false, Code: code, Message: "Promotion expired"}, nil
	}

	// Check usage limit
	if p.UsageLimit > 0 && p.CurrentUses >= p.UsageLimit {
		return &ValidationResult{Valid: false, Code: code, Message: "Promotion fully redeemed"}, nil
	}

	// Calculate discount
	var discount float64
	var promoType string

	if p.DiscountAmount != nil && *p.DiscountAmount > 0 {
		discount = *p.DiscountAmount
		promoType = "fixed"
	} else if p.DiscountPct != nil && *p.DiscountPct > 0 {
		discount = amount * float64(*p.DiscountPct) / 100.0
		promoType = "percentage"
	}

	// Ensure discount doesn't exceed total amount
	if discount > amount {
		discount = amount
	}

	return &ValidationResult{
		Valid:          true,
		Code:           p.Code,
		DiscountAmount: discount,
		Message:        "Promotion applied",
		Type:           promoType,
	}, nil
}
