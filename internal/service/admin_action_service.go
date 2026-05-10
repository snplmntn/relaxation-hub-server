package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type AdminActionService struct {
	repo repository.AdminActionRepository
}

func NewAdminActionService(repo repository.AdminActionRepository) *AdminActionService {
	return &AdminActionService{repo: repo}
}

func (s *AdminActionService) Log(ctx context.Context, adminID int64, req *model.CreateAdminActionRequest) (*model.AdminAction, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if strings.TrimSpace(req.ActionType) == "" {
		return nil, fmt.Errorf("action_type is required")
	}

	action := &model.AdminAction{
		AdminID:     adminID,
		ActionType:  strings.TrimSpace(req.ActionType),
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Description: req.Description,
		OldValue:    req.OldValue,
		NewValue:    req.NewValue,
		IPAddress:   req.IPAddress,
	}

	if err := s.repo.Log(ctx, action); err != nil {
		return nil, err
	}
	return action, nil
}

func (s *AdminActionService) GetByAdmin(ctx context.Context, adminID int64, limit int) ([]model.AdminAction, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.repo.GetByAdmin(ctx, adminID, limit)
}

func (s *AdminActionService) GetAll(ctx context.Context, limit int) ([]model.AdminAction, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.repo.GetAll(ctx, limit)
}
