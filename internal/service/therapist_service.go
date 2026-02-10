package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type TherapistService struct {
	repo     repository.TherapistRepository
	userRepo repository.UserRepository
}

func NewTherapistService(repo repository.TherapistRepository, userRepo repository.UserRepository) *TherapistService {
	return &TherapistService{repo: repo, userRepo: userRepo}
}

func (s *TherapistService) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	return s.repo.GetProfile(ctx, therapistID)
}

func (s *TherapistService) UpdateProfile(ctx context.Context, therapistID int64, req *model.UpdateTherapistProfileRequest) (*model.TherapistProfile, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	// Check if user is suspended/banned before allowing AcceptAssignments toggle
	if req.AcceptAssignments != nil && *req.AcceptAssignments && s.userRepo != nil {
		user, err := s.userRepo.FindUserByID(ctx, int(therapistID))
		if err == nil && user != nil {
			if user.AccountStatus == "suspended" || user.AccountStatus == "banned" {
				return nil, fmt.Errorf("your account is currently suspended. Please contact support")
			}
		}
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
	if req.AcceptAssignments != nil {
		updates["accept_assignments"] = *req.AcceptAssignments
	}
	if req.BranchID != nil {
		updates["branch_id"] = req.BranchID
	}
	if req.IsVerified != nil {
		updates["is_verified"] = *req.IsVerified
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	if err := s.repo.UpdateProfile(ctx, therapistID, updates); err != nil {
		if err == pgx.ErrNoRows {
			// create a profile row and retry the update
			if err2 := s.repo.CreateProfile(ctx, therapistID); err2 != nil {
				return nil, err2
			}
			if err2 := s.repo.UpdateProfile(ctx, therapistID, updates); err2 != nil {
				return nil, err2
			}
		} else {
			return nil, err
		}
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

func (s *TherapistService) AddService(ctx context.Context, therapistID int64, req *model.AddServiceWithPressuresRequest) error {
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

	if err := s.repo.AddService(ctx, ts); err != nil {
		// If therapist profile doesn't exist (FK violation), create it and retry
		if strings.Contains(err.Error(), "therapist_services_therapist_id_fkey") {
			if err := s.repo.CreateProfile(ctx, therapistID); err != nil {
				return err
			}
			// Retry adding service
			if err := s.repo.AddService(ctx, ts); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// If client supplied pressures, persist them in therapist_service_pressures
	if len(req.Pressures) > 0 {
		if err := s.repo.SetServicePressures(ctx, therapistID, req.ServiceID, req.Pressures); err != nil {
			return err
		}
	}

	return nil
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

func (s *TherapistService) GetServicesWithPressures(ctx context.Context, therapistID int64) (map[int64][]string, error) {
	return s.repo.GetServicesWithPressures(ctx, therapistID)
}

func (s *TherapistService) BatchUpdateServices(ctx context.Context, therapistID int64, services []model.AddServiceWithPressuresRequest) error {
	// Optional: validate service IDs exist and pressures are valid
	return s.repo.SetBatchServices(ctx, therapistID, services)
}

// SetAtBranch updates the therapist's location status.
// atBranch=true means they're at their assigned branch.
// atBranch=false means they're in the field (on assignment).
func (s *TherapistService) SetAtBranch(ctx context.Context, therapistID int64, atBranch bool) error {
	return s.repo.SetAtBranch(ctx, therapistID, atBranch)
}
