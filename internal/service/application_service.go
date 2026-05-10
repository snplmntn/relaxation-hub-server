package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/broadcaster"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type ApplicationService struct {
	appRepo       repository.ApplicationRepository
	authService   AuthService
	userRepo      repository.UserRepository
	branchRepo    repository.BranchRepository
	therapistRepo repository.TherapistRepository
	rideRepo      repository.RideRepository
}

func NewApplicationService(
	appRepo repository.ApplicationRepository,
	authService AuthService,
	userRepo repository.UserRepository,
	branchRepo repository.BranchRepository,
	therapistRepo repository.TherapistRepository,
	rideRepo repository.RideRepository,
) *ApplicationService {
	return &ApplicationService{
		appRepo:       appRepo,
		authService:   authService,
		userRepo:      userRepo,
		branchRepo:    branchRepo,
		therapistRepo: therapistRepo,
		rideRepo:      rideRepo,
	}
}

func (s *ApplicationService) Submit(ctx context.Context, req *model.CreateApplicationRequest) (*model.ApplicantApplication, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	targetRole := strings.ToLower(strings.TrimSpace(req.TargetRole))
	if targetRole != "therapist" && targetRole != "rider" {
		return nil, fmt.Errorf("target_role must be therapist or rider")
	}
	if req.PreferredBranchID <= 0 {
		return nil, fmt.Errorf("preferred_branch_id is required")
	}
	if strings.TrimSpace(req.PositionApplied) == "" {
		return nil, fmt.Errorf("position_applied is required")
	}
	if strings.TrimSpace(req.FullName) == "" {
		return nil, fmt.Errorf("full_name is required")
	}

	if _, err := s.branchRepo.GetByID(ctx, req.PreferredBranchID); err != nil {
		return nil, fmt.Errorf("invalid preferred branch")
	}

	userID, _, err := s.authService.Signup(
		ctx,
		req.Provider,
		req.ProviderKey,
		req.Password,
		targetRole,
	)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"full_name":      strings.TrimSpace(req.FullName),
		"account_status": "inactive",
		"status_reason":  model.ApplicationStatusReasonPending,
	}
	if phone := strings.TrimSpace(req.PrimaryPhone); phone != "" {
		updates["primary_phone"] = phone
	}
	if err := s.userRepo.UpdateUser(ctx, int64(userID), updates); err != nil {
		return nil, err
	}

	if targetRole == "therapist" && s.therapistRepo != nil {
		_ = s.therapistRepo.CreateProfile(ctx, int64(userID))
	}
	if targetRole == "rider" && s.rideRepo != nil {
		vehicleType := "motorcycle"
		licensePlate := ""
		if req.Answers != nil {
			if v, ok := req.Answers["vehicle_type"].(string); ok && strings.TrimSpace(v) != "" {
				vehicleType = strings.TrimSpace(v)
			}
			if v, ok := req.Answers["license_plate"].(string); ok {
				licensePlate = strings.TrimSpace(v)
			}
		}
		_ = s.rideRepo.CreateRiderProfile(ctx, int64(userID), vehicleType, licensePlate)
	}

	app := &model.ApplicantApplication{
		UserID:               int64(userID),
		TargetRole:           targetRole,
		PositionApplied:      strings.TrimSpace(req.PositionApplied),
		PreferredBranchID:    req.PreferredBranchID,
		PreferredBranchLabel: strings.TrimSpace(req.PreferredBranchLabel),
		Status:               model.ApplicationStatusPending,
		Answers:              safeJSON(req.Answers),
		Attachments:          safeJSON(req.Attachments),
	}
	if err := s.appRepo.Create(ctx, app); err != nil {
		return nil, err
	}

	_ = broadcaster.BroadcastToAdmins(ctx, "application.created", map[string]interface{}{
		"application_id": app.ApplicationID,
		"user_id":        app.UserID,
		"target_role":    app.TargetRole,
		"status":         app.Status,
	})

	return app, nil
}

func (s *ApplicationService) List(ctx context.Context, filters model.ListApplicationsFilters) ([]model.ApplicantApplication, int, error) {
	return s.appRepo.List(ctx, filters)
}

func (s *ApplicationService) GetByID(ctx context.Context, applicationID int64) (*model.ApplicantApplication, error) {
	return s.appRepo.GetByID(ctx, applicationID)
}

func (s *ApplicationService) UpdateStatus(ctx context.Context, applicationID int64, actorID int64, req model.UpdateApplicationStatusRequest) (*model.ApplicantApplication, error) {
	switch req.Status {
	case model.ApplicationStatusApproved, model.ApplicationStatusRejected, model.ApplicationStatusNeedsFollow:
	default:
		return nil, fmt.Errorf("invalid status")
	}

	app, err := s.appRepo.UpdateStatus(ctx, applicationID, req.Status, actorID, strings.TrimSpace(req.ReviewNotes))
	if err != nil {
		return nil, err
	}

	userUpdates := map[string]interface{}{}
	if req.Status == model.ApplicationStatusApproved {
		userUpdates["account_status"] = "active"
		userUpdates["status_reason"] = ""
	} else {
		userUpdates["account_status"] = "inactive"
		if strings.TrimSpace(req.ReviewNotes) != "" {
			userUpdates["status_reason"] = strings.TrimSpace(req.ReviewNotes)
		}
	}
	if len(userUpdates) > 0 {
		_ = s.userRepo.UpdateUser(ctx, app.UserID, userUpdates)
	}

	_ = broadcaster.BroadcastToAdmins(ctx, "application.updated", map[string]interface{}{
		"application_id": app.ApplicationID,
		"user_id":        app.UserID,
		"target_role":    app.TargetRole,
		"status":         app.Status,
	})

	return app, nil
}

func safeJSON(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return map[string]interface{}{}
	}
	return input
}
