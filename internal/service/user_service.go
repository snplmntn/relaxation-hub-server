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
	// ListPaginated returns paginated users filtered by role
	ListPaginated(ctx context.Context, role string, page, limit int, search string) ([]model.User, int, error)
	// ListPaginatedFiltered returns paginated users with optional status/VIP filters
	ListPaginatedFiltered(ctx context.Context, role, status string, vip *bool, page, limit int, search string) ([]model.User, int, error)
	// CountByStatus returns roster totals broken down by account_status (plus VIP)
	CountByStatus(ctx context.Context, role, search string) (model.UserStatusCounts, error)
	BlockUser(ctx context.Context, blockerID, blockedID int64) error
	UnblockUser(ctx context.Context, blockerID, blockedID int64) error
	GetBlockList(ctx context.Context, userID int64) ([]repository.BlockedUserEntry, error)
	// Admin-mediated client→therapist blocks
	AdminBlockTherapistForClient(ctx context.Context, clientID, therapistID int64) error
	AdminUnblockTherapistForClient(ctx context.Context, clientID, therapistID int64) error
	AdminListClientBlocks(ctx context.Context, clientID int64) ([]repository.BlockedUserEntry, error)
	// UpdateFCMToken updates the FCM token for push notifications
	UpdateFCMToken(ctx context.Context, userID int64, token string) error
	DeactivateClient(ctx context.Context, userID int64) (*model.User, error)
	ReactivateClient(ctx context.Context, userID int64) (*model.User, error)

	// Favorites
	AddFavorite(ctx context.Context, userID, therapistID int64) error
	RemoveFavorite(ctx context.Context, userID, therapistID int64) error
	ListFavorites(ctx context.Context, userID int64) ([]model.User, error)
	IsFavorite(ctx context.Context, userID, therapistID int64) (bool, error)
}

type userService struct {
	repo        repository.UserRepository
	addressRepo repository.AddressRepository
	rideRepo    repository.RideRepository
}

func NewUserService(repo repository.UserRepository, addressRepo repository.AddressRepository, rideRepo repository.RideRepository) UserService {
	return &userService{
		repo:        repo,
		addressRepo: addressRepo,
		rideRepo:    rideRepo,
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

	// Enrich with Rider Profile if applicable
	if user.Role == "rider" {
		rider, err := s.rideRepo.GetRiderProfile(ctx, userID)
		if err == nil {
			user.Rider = rider
		}
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

func (s *userService) ListPaginated(ctx context.Context, role string, page, limit int, search string) ([]model.User, int, error) {
	return s.repo.ListUsersPaginated(ctx, role, page, limit, search)
}

func (s *userService) ListPaginatedFiltered(ctx context.Context, role, status string, vip *bool, page, limit int, search string) ([]model.User, int, error) {
	return s.repo.ListUsersFiltered(ctx, role, status, vip, page, limit, search)
}

func (s *userService) CountByStatus(ctx context.Context, role, search string) (model.UserStatusCounts, error) {
	return s.repo.CountUsersByStatus(ctx, role, search)
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

// AdminBlockTherapistForClient records a block on behalf of a client (blocker)
// against a therapist (blocked). Used by admins; validates that the two users
// are a client and a therapist respectively.
func (s *userService) AdminBlockTherapistForClient(ctx context.Context, clientID, therapistID int64) error {
	if clientID == therapistID {
		return fmt.Errorf("client and therapist must be different users")
	}
	client, err := s.repo.FindUserByID(ctx, int(clientID))
	if err != nil {
		return fmt.Errorf("client not found")
	}
	if client.Role != model.RoleClient {
		return fmt.Errorf("user %d is not a client", clientID)
	}
	therapist, err := s.repo.FindUserByID(ctx, int(therapistID))
	if err != nil {
		return fmt.Errorf("therapist not found")
	}
	if therapist.Role != model.RoleTherapist {
		return fmt.Errorf("user %d is not a therapist", therapistID)
	}
	return s.repo.BlockUser(ctx, clientID, therapistID)
}

func (s *userService) AdminUnblockTherapistForClient(ctx context.Context, clientID, therapistID int64) error {
	return s.repo.UnblockUser(ctx, clientID, therapistID)
}

func (s *userService) AdminListClientBlocks(ctx context.Context, clientID int64) ([]repository.BlockedUserEntry, error) {
	return s.repo.GetBlockList(ctx, clientID)
}

func (s *userService) UpdateFCMToken(ctx context.Context, userID int64, token string) error {
	if token == "" {
		return fmt.Errorf("FCM token cannot be empty")
	}
	return s.repo.UpdateFCMToken(ctx, userID, token)
}

func (s *userService) AddFavorite(ctx context.Context, userID, therapistID int64) error {
	if userID == therapistID {
		return fmt.Errorf("cannot favorite yourself")
	}
	return s.repo.AddFavoriteTherapist(ctx, userID, therapistID)
}

func (s *userService) RemoveFavorite(ctx context.Context, userID, therapistID int64) error {
	return s.repo.RemoveFavoriteTherapist(ctx, userID, therapistID)
}

func (s *userService) ListFavorites(ctx context.Context, userID int64) ([]model.User, error) {
	return s.repo.ListFavoriteTherapists(ctx, userID)
}

func (s *userService) IsFavorite(ctx context.Context, userID, therapistID int64) (bool, error) {
	return s.repo.IsTherapistFavorite(ctx, userID, therapistID)
}

func (s *userService) DeactivateClient(ctx context.Context, userID int64) (*model.User, error) {
	return s.setClientAccountStatus(ctx, userID, "inactive")
}

func (s *userService) ReactivateClient(ctx context.Context, userID int64) (*model.User, error) {
	return s.setClientAccountStatus(ctx, userID, "active")
}

func (s *userService) setClientAccountStatus(ctx context.Context, userID int64, status string) (*model.User, error) {
	user, err := s.repo.FindUserByID(ctx, int(userID))
	if err != nil {
		return nil, err
	}
	if user.Role != model.RoleClient {
		return nil, fmt.Errorf("user is not a client")
	}

	if err := s.repo.UpdateUser(ctx, userID, map[string]interface{}{"account_status": status}); err != nil {
		return nil, err
	}
	return s.repo.FindUserByID(ctx, int(userID))
}
