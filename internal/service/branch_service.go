package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type BranchService struct {
	repo repository.BranchRepository
}

func NewBranchService(repo repository.BranchRepository) *BranchService {
	return &BranchService{repo: repo}
}

func (s *BranchService) Create(ctx context.Context, req *model.CreateBranchRequest) (*model.Branch, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if strings.TrimSpace(req.BranchName) == "" {
		return nil, fmt.Errorf("branch_name is required")
	}
	if strings.TrimSpace(req.City) == "" {
		return nil, fmt.Errorf("city is required")
	}
	if strings.TrimSpace(req.Province) == "" {
		return nil, fmt.Errorf("province is required")
	}

	trimmedAddress := strings.TrimSpace(req.AddressLine)
	trimmedCity := strings.TrimSpace(req.City)
	trimmedProvince := strings.TrimSpace(req.Province)
	isActive := true

	branch := &model.Branch{
		BranchName:  strings.TrimSpace(req.BranchName),
		AddressLine: &trimmedAddress,
		Barangay:    req.Barangay,
		City:        &trimmedCity,
		Province:    &trimmedProvince,
		PostalCode:  req.PostalCode,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		ContactNo:   req.ContactNo,
		Email:       req.Email,
		IsActive:    &isActive,
	}

	if err := s.repo.Create(ctx, branch); err != nil {
		return nil, err
	}
	return branch, nil
}

func (s *BranchService) GetByID(ctx context.Context, branchID int64) (*model.Branch, error) {
	return s.repo.GetByID(ctx, branchID)
}

func (s *BranchService) List(ctx context.Context, activeOnly bool) ([]model.Branch, error) {
	return s.repo.List(ctx, activeOnly)
}

func (s *BranchService) Update(ctx context.Context, branchID int64, req *model.UpdateBranchRequest) (*model.Branch, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	updates := make(map[string]interface{})
	if req.BranchName != nil {
		updates["branch_name"] = strings.TrimSpace(*req.BranchName)
	}
	if req.AddressLine != nil {
		updates["address_line"] = strings.TrimSpace(*req.AddressLine)
	}
	if req.Barangay != nil {
		updates["barangay"] = req.Barangay
	}
	if req.City != nil {
		updates["city"] = strings.TrimSpace(*req.City)
	}
	if req.Province != nil {
		updates["province"] = strings.TrimSpace(*req.Province)
	}
	if req.PostalCode != nil {
		updates["postal_code"] = req.PostalCode
	}
	if req.Latitude != nil {
		updates["latitude"] = req.Latitude
	}
	if req.Longitude != nil {
		updates["longitude"] = req.Longitude
	}
	if req.ContactNo != nil {
		updates["contact_no"] = req.ContactNo
	}
	if req.Email != nil {
		updates["email"] = req.Email
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	if err := s.repo.Update(ctx, branchID, updates); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, branchID)
}
