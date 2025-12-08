package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type TherapistService struct {
	repo repository.TherapistRepository
}

func NewTherapistService(repo repository.TherapistRepository) *TherapistService {
	return &TherapistService{repo: repo}
}

func (s *TherapistService) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	return s.repo.GetProfile(ctx, therapistID)
}

func (s *TherapistService) UpdateProfile(ctx context.Context, therapistID int64, req *model.UpdateTherapistProfileRequest) (*model.TherapistProfile, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	updates := make(map[string]interface{})
	if req.Bio != nil {
		updates["bio"] = strings.TrimSpace(*req.Bio)
	}
	if req.Specialization != nil {
		updates["specialization"] = strings.TrimSpace(*req.Specialization)
	}
	if req.YearsExperience != nil {
		if *req.YearsExperience < 0 {
			return nil, fmt.Errorf("years_experience must be non-negative")
		}
		updates["years_experience"] = *req.YearsExperience
	}
	if req.IsAvailable != nil {
		updates["is_available"] = *req.IsAvailable
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	if err := s.repo.UpdateProfile(ctx, therapistID, updates); err != nil {
		return nil, err
	}
	return s.repo.GetProfile(ctx, therapistID)
}

func (s *TherapistService) List(ctx context.Context, availableOnly bool) ([]model.TherapistProfile, error) {
	return s.repo.List(ctx, availableOnly)
}

func (s *TherapistService) UploadDocument(ctx context.Context, therapistID int64, req *model.UploadDocumentRequest) (*model.TherapistDocument, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if strings.TrimSpace(req.DocumentType) == "" {
		return nil, fmt.Errorf("document_type is required")
	}
	if strings.TrimSpace(req.DocumentURL) == "" {
		return nil, fmt.Errorf("document_url is required")
	}

	doc := &model.TherapistDocument{
		TherapistID:  therapistID,
		DocumentType: strings.TrimSpace(req.DocumentType),
		DocumentURL:  strings.TrimSpace(req.DocumentURL),
		Status:       "pending",
	}

	if err := s.repo.UploadDocument(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *TherapistService) GetDocuments(ctx context.Context, therapistID int64) ([]model.TherapistDocument, error) {
	return s.repo.GetDocuments(ctx, therapistID)
}

func (s *TherapistService) VerifyDocument(ctx context.Context, documentID, verifierID int64, req *model.VerifyDocumentRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		return fmt.Errorf("status is required")
	}
	if status != "approved" && status != "rejected" {
		return fmt.Errorf("status must be 'approved' or 'rejected'")
	}

	return s.repo.VerifyDocument(ctx, documentID, verifierID, status)
}

func (s *TherapistService) AddService(ctx context.Context, therapistID int64, req *model.AddServiceRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if req.ServiceID == 0 {
		return fmt.Errorf("service_id is required")
	}

	ts := &model.TherapistService{
		TherapistID: therapistID,
		ServiceID:   req.ServiceID,
	}

	return s.repo.AddService(ctx, ts)
}

func (s *TherapistService) RemoveService(ctx context.Context, therapistID, serviceID int64) error {
	if serviceID == 0 {
		return fmt.Errorf("service_id is required")
	}
	return s.repo.RemoveService(ctx, therapistID, serviceID)
}

func (s *TherapistService) GetServices(ctx context.Context, therapistID int64) ([]int64, error) {
	return s.repo.GetServices(ctx, therapistID)
}
