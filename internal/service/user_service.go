package service

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type UserService interface {
	Update(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error)
	// Get returns the user profile for the given authenticated user.
	Get(ctx context.Context, userID int64) (*model.User, error)
	// List returns users optionally filtered by role (empty string for all)
	List(ctx context.Context, role string) ([]model.User, error)
	BlockUser(ctx context.Context, blockerID, blockedID int64) error
	UnblockUser(ctx context.Context, blockerID, blockedID int64) error
	GetBlockList(ctx context.Context, userID int64) ([]repository.BlockedUserEntry, error)
	// UpdateFCMToken updates the FCM token for push notifications
	UpdateFCMToken(ctx context.Context, userID int64, token string) error
}

type userService struct {
	repo        repository.UserRepository
	addressRepo repository.AddressRepository
}

func NewUserService(repo repository.UserRepository, addressRepo repository.AddressRepository) UserService {
	return &userService{
		repo:        repo,
		addressRepo: addressRepo,
	}
}

func (s *userService) Update(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	if err := s.repo.UpdateUser(ctx, userID, updates); err != nil {
		return nil, err
	}

	user, err := s.repo.FindUserByID(ctx, int(userID))
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) Get(ctx context.Context, userID int64) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, int(userID))
	if err != nil {
		return nil, err
	}

	// Fetch addresses
	addresses, err := s.addressRepo.ListForUser(ctx, userID, false)
	if err == nil {
		// Convert model.Address to model.AddressResponse
		var addrResponses []model.AddressResponse
		for _, a := range addresses {
			addrResponses = append(addrResponses, model.AddressResponse{
				AddressID:  a.AddressID,
				Label:      a.Label,
				Street:     a.Street,
				City:       a.City,
				Province:   a.Province,
				PostalCode: a.PostalCode,
				Country:    a.Country,
				Latitude:   a.Latitude,
				Longitude:  a.Longitude,
				IsDefault:  a.IsDefault,
				CreatedAt:  a.CreatedAt,
				UpdatedAt:  a.UpdatedAt,
			})
		}
		user.Addresses = addrResponses
	}

	return user, nil
}

func (s *userService) List(ctx context.Context, role string) ([]model.User, error) {
	return s.repo.ListUsers(ctx, role)
}

func (s *userService) BlockUser(ctx context.Context, blockerID, blockedID int64) error {
	if blockerID == blockedID {
		return fmt.Errorf("cannot block yourself")
	}
	return s.repo.BlockUser(ctx, blockerID, blockedID)
}

func (s *userService) UnblockUser(ctx context.Context, blockerID, blockedID int64) error {
	return s.repo.UnblockUser(ctx, blockerID, blockedID)
}

func (s *userService) GetBlockList(ctx context.Context, userID int64) ([]repository.BlockedUserEntry, error) {
	return s.repo.GetBlockList(ctx, userID)
}

func (s *userService) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	if token == "" {
		return fmt.Errorf("FCM token cannot be empty")
	}
	return s.repo.UpdateFCMToken(ctx, userID, token)
}
