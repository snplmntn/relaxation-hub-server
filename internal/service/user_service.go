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
}

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{repo: repo}
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
	return s.repo.FindUserByID(ctx, int(userID))
}

func (s *userService) List(ctx context.Context, role string) ([]model.User, error) {
	return s.repo.ListUsers(ctx, role)
}
