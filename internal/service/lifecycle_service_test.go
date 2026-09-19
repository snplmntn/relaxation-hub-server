package service

import (
	"context"
	"errors"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type lifecycleUserRepo struct {
	nilUserRepo
	user    *model.User
	err     error
	updates map[string]interface{}
}

func (r *lifecycleUserRepo) FindUserByID(ctx context.Context, userID int) (*model.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.user == nil {
		return nil, errors.New("user not found")
	}
	return r.user, nil
}

func (r *lifecycleUserRepo) UpdateUser(ctx context.Context, userID int64, updates map[string]interface{}) error {
	r.updates = updates
	if r.user != nil {
		if status, ok := updates["account_status"].(string); ok {
			r.user.AccountStatus = status
		}
	}
	return nil
}

type lifecycleTherapistRepo struct {
	nilTherapistRepo
	profileUpdates         map[string]interface{}
	lifecycleStatusUpdates []string
	lifecycleAcceptUpdates []bool
}

func (r *lifecycleTherapistRepo) UpdateProfile(ctx context.Context, therapistID int64, updates map[string]interface{}) error {
	r.profileUpdates = updates
	return nil
}

func (r *lifecycleTherapistRepo) GetProfile(ctx context.Context, therapistID int64) (*model.TherapistProfile, error) {
	acceptAssignments, _ := r.profileUpdates["accept_assignments"].(bool)
	status := ""
	if len(r.lifecycleStatusUpdates) > 0 {
		status = r.lifecycleStatusUpdates[len(r.lifecycleStatusUpdates)-1]
	}
	return &model.TherapistProfile{TherapistID: therapistID, Status: status, AcceptAssignments: acceptAssignments}, nil
}

func (r *lifecycleTherapistRepo) SetLifecycleStatus(ctx context.Context, therapistID int64, accountStatus string, acceptAssignments bool) error {
	r.lifecycleStatusUpdates = append(r.lifecycleStatusUpdates, accountStatus)
	r.lifecycleAcceptUpdates = append(r.lifecycleAcceptUpdates, acceptAssignments)
	r.profileUpdates = map[string]interface{}{"accept_assignments": acceptAssignments}
	return nil
}

func TestUserService_DeactivateClientSetsInactive(t *testing.T) {
	repo := &lifecycleUserRepo{user: &model.User{UserID: 42, Role: model.RoleClient, AccountStatus: "active"}}
	svc := NewUserService(repo, &fakeAddressRepo{}, &fakeRideRepo{})

	user, err := svc.DeactivateClient(context.Background(), 42)

	assert.NoError(t, err)
	assert.Equal(t, "inactive", repo.updates["account_status"])
	assert.Equal(t, "inactive", user.AccountStatus)
}

func TestUserService_ReactivateClientSetsActive(t *testing.T) {
	repo := &lifecycleUserRepo{user: &model.User{UserID: 42, Role: model.RoleClient, AccountStatus: "inactive"}}
	svc := NewUserService(repo, &fakeAddressRepo{}, &fakeRideRepo{})

	user, err := svc.ReactivateClient(context.Background(), 42)

	assert.NoError(t, err)
	assert.Equal(t, "active", repo.updates["account_status"])
	assert.Equal(t, "active", user.AccountStatus)
}

func TestTherapistService_DeactivateTherapistConflictsWithActiveBookings(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(77)
	userRepo := &lifecycleUserRepo{user: &model.User{UserID: int(therapistID), Role: model.RoleTherapist, AccountStatus: "active"}}
	therapistRepo := &lifecycleTherapistRepo{}
	bookingRepo := new(MockBookingRepository)
	bookingRepo.On("HasActiveNonFinalBookings", ctx, therapistID).Return(true, nil).Once()
	svc := NewTherapistService(therapistRepo, userRepo, bookingRepo)

	profile, err := svc.DeactivateTherapist(ctx, therapistID)

	assert.Nil(t, profile)
	assert.ErrorIs(t, err, ErrTherapistHasActiveBookings)
	assert.Nil(t, userRepo.updates)
	assert.Nil(t, therapistRepo.profileUpdates)
	bookingRepo.AssertExpectations(t)
}

func TestTherapistService_DeactivateTherapistSetsInactiveAndStopsAssignments(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(77)
	userRepo := &lifecycleUserRepo{user: &model.User{UserID: int(therapistID), Role: model.RoleTherapist, AccountStatus: "active"}}
	therapistRepo := &lifecycleTherapistRepo{}
	bookingRepo := new(MockBookingRepository)
	bookingRepo.On("HasActiveNonFinalBookings", ctx, therapistID).Return(false, nil).Once()
	svc := NewTherapistService(therapistRepo, userRepo, bookingRepo)

	profile, err := svc.DeactivateTherapist(ctx, therapistID)

	assert.NoError(t, err)
	assert.Nil(t, userRepo.updates)
	assert.Equal(t, []string{"inactive"}, therapistRepo.lifecycleStatusUpdates)
	assert.Equal(t, []bool{false}, therapistRepo.lifecycleAcceptUpdates)
	assert.Equal(t, "inactive", profile.Status)
	assert.False(t, profile.AcceptAssignments)
	bookingRepo.AssertExpectations(t)
}

func TestTherapistService_ReactivateTherapistSetsActiveAndAcceptsAssignments(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(77)
	userRepo := &lifecycleUserRepo{user: &model.User{UserID: int(therapistID), Role: model.RoleTherapist, AccountStatus: "inactive"}}
	therapistRepo := &lifecycleTherapistRepo{}
	svc := NewTherapistService(therapistRepo, userRepo)

	profile, err := svc.ReactivateTherapist(ctx, therapistID)

	assert.NoError(t, err)
	assert.Nil(t, userRepo.updates)
	assert.Equal(t, []string{"active"}, therapistRepo.lifecycleStatusUpdates)
	assert.Equal(t, []bool{true}, therapistRepo.lifecycleAcceptUpdates)
	assert.Equal(t, "active", profile.Status)
	assert.True(t, profile.AcceptAssignments)
}

func TestTherapistService_UpdateProfileRejectsAcceptAssignmentsForInactiveAccount(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(77)
	acceptAssignments := true
	userRepo := &lifecycleUserRepo{user: &model.User{UserID: int(therapistID), Role: model.RoleTherapist, AccountStatus: "inactive"}}
	therapistRepo := &lifecycleTherapistRepo{}
	svc := NewTherapistService(therapistRepo, userRepo)

	profile, err := svc.UpdateProfile(ctx, therapistID, &model.UpdateTherapistProfileRequest{AcceptAssignments: &acceptAssignments})

	assert.Nil(t, profile)
	assert.Error(t, err)
	assert.Nil(t, therapistRepo.profileUpdates)
}
func TestTherapistService_UpdateProfileRejectsAcceptAssignmentsWhenUserLookupErrors(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(77)
	acceptAssignments := true
	userRepo := &lifecycleUserRepo{err: errors.New("lookup failed")}
	therapistRepo := &lifecycleTherapistRepo{}
	svc := NewTherapistService(therapistRepo, userRepo)

	profile, err := svc.UpdateProfile(ctx, therapistID, &model.UpdateTherapistProfileRequest{AcceptAssignments: &acceptAssignments})

	assert.Nil(t, profile)
	assert.Error(t, err)
	assert.Nil(t, therapistRepo.profileUpdates)
}

func TestTherapistService_UpdateProfileNormalizesGender(t *testing.T) {
	ctx := context.Background()
	gender := " Female "
	userRepo := &lifecycleUserRepo{user: &model.User{UserID: 77, Role: model.RoleTherapist, AccountStatus: "active"}}
	therapistRepo := &lifecycleTherapistRepo{}
	svc := NewTherapistService(therapistRepo, userRepo)

	_, err := svc.UpdateProfile(ctx, 77, &model.UpdateTherapistProfileRequest{Gender: &gender})

	assert.NoError(t, err)
	assert.Equal(t, "female", userRepo.updates["gender"])
}

func TestTherapistService_UpdateProfileRejectsUnknownGender(t *testing.T) {
	gender := "unknown"
	svc := NewTherapistService(&lifecycleTherapistRepo{}, &lifecycleUserRepo{})

	profile, err := svc.UpdateProfile(context.Background(), 77, &model.UpdateTherapistProfileRequest{Gender: &gender})

	assert.Nil(t, profile)
	assert.EqualError(t, err, "gender must be 'male' or 'female'")
}

type lifecycleBranchRepo struct {
	branch  *model.Branch
	updates map[string]interface{}
}

func (r *lifecycleBranchRepo) Create(ctx context.Context, branch *model.Branch) error { return nil }
func (r *lifecycleBranchRepo) GetByID(ctx context.Context, branchID int64) (*model.Branch, error) {
	if r.branch == nil {
		return nil, errors.New("branch not found")
	}
	return r.branch, nil
}
func (r *lifecycleBranchRepo) List(ctx context.Context, activeOnly bool) ([]model.Branch, error) {
	return nil, nil
}
func (r *lifecycleBranchRepo) Update(ctx context.Context, branchID int64, updates map[string]interface{}) error {
	r.updates = updates
	if active, ok := updates["is_active"].(bool); ok && r.branch != nil {
		r.branch.IsActive = &active
	}
	return nil
}

func TestBranchService_DeactivateBranchSetsInactive(t *testing.T) {
	active := true
	repo := &lifecycleBranchRepo{branch: &model.Branch{BranchID: 9, IsActive: &active}}
	svc := NewBranchService(repo)

	branch, err := svc.DeactivateBranch(context.Background(), 9)

	assert.NoError(t, err)
	assert.Equal(t, false, repo.updates["is_active"])
	assert.False(t, *branch.IsActive)
}

func TestBranchService_ReactivateBranchSetsActive(t *testing.T) {
	active := false
	repo := &lifecycleBranchRepo{branch: &model.Branch{BranchID: 9, IsActive: &active}}
	svc := NewBranchService(repo)

	branch, err := svc.ReactivateBranch(context.Background(), 9)

	assert.NoError(t, err)
	assert.Equal(t, true, repo.updates["is_active"])
	assert.True(t, *branch.IsActive)
}

func TestTherapistService_DeactivateTherapistDoesNotUnassignBookings(t *testing.T) {
	ctx := context.Background()
	therapistID := int64(77)
	userRepo := &lifecycleUserRepo{user: &model.User{UserID: int(therapistID), Role: model.RoleTherapist, AccountStatus: "active"}}
	therapistRepo := &lifecycleTherapistRepo{}
	bookingRepo := new(MockBookingRepository)
	bookingRepo.On("HasActiveNonFinalBookings", ctx, therapistID).Return(false, nil).Once()
	bookingRepo.AssertNotCalled(t, "UnassignTherapist", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	svc := NewTherapistService(therapistRepo, userRepo, bookingRepo)

	_, err := svc.DeactivateTherapist(ctx, therapistID)

	assert.NoError(t, err)
	bookingRepo.AssertNotCalled(t, "UnassignTherapist", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	bookingRepo.AssertExpectations(t)
}
