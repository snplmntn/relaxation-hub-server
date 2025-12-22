package service

import (
	"context"
	"fmt"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

type UserService interface {
	Update(ctx context.Context, userID int64, updates map[string]interface{}) (*model.User, error)
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
