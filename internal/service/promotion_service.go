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
	if req.DiscountPct < 1 || req.DiscountPct > 100 {
		return nil, fmt.Errorf("discount_percent must be 1-100")
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
		Code:        code,
		DiscountPct: req.DiscountPct,
		ValidFrom:   validFrom,
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
