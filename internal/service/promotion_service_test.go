package service

import (
	"context"
	"testing"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPromotionServiceCreate_RequiresAppliesTo(t *testing.T) {
	repo := new(MockPromoRepository)
	svc := NewPromotionService(repo)

	_, err := svc.Create(context.Background(), &model.CreatePromotionRequest{
		Code:        "SAVE10",
		DiscountPct: 10,
	})

	var ve *ValidationError
	if assert.ErrorAs(t, err, &ve) {
		assert.Equal(t, "invalid_applies_to", ve.Code)
		assert.Equal(t, "applies_to is required", ve.Message)
	}

	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestPromotionServiceUpdate_ValidatesAppliesTo(t *testing.T) {
	repo := new(MockPromoRepository)
	svc := NewPromotionService(repo)

	_, err := svc.Update(context.Background(), 12, map[string]interface{}{
		"applies_to": "everything",
	})

	var ve *ValidationError
	if assert.ErrorAs(t, err, &ve) {
		assert.Equal(t, "invalid_applies_to", ve.Code)
		assert.Equal(t, "applies_to must be full_basket or services_only", ve.Message)
	}

	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestPromotionServiceValidateForClient_AllowsNonVIP(t *testing.T) {
	repo := new(MockPromoRepository)
	userRepo := new(MockUserRepository)
	userRepo.On("FindUserByID", mock.Anything, 42).Return(
		&model.User{UserID: 42, Role: model.RoleClient, AccountStatus: "active", IsVIP: false},
		nil,
	).Once()

	discountPct := 10
	repo.On("GetByCode", mock.Anything, "SAVE10").Return(&model.Promotion{
		PromoID:     7,
		Code:        "SAVE10",
		DiscountPct: &discountPct,
		IsPublic:    true,
	}, nil).Once()

	svc := NewPromotionService(repo, userRepo)

	result, err := svc.ValidateForClient(context.Background(), 42, "SAVE10", 1000)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Valid)
	assert.Equal(t, 100.0, result.DiscountAmount)
	repo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestPromotionServiceValidate_InternalCodeIsClientFacingOnlyForStaff(t *testing.T) {
	discountPct := 100
	internalPromo := func() *model.Promotion {
		return &model.Promotion{
			PromoID:     9,
			Code:        "PARTNERHOTEL",
			DiscountPct: &discountPct,
			IsPublic:    false,
		}
	}
	activeClient := func() *model.User {
		return &model.User{UserID: 42, Role: model.RoleClient, AccountStatus: "active"}
	}

	t.Run("client cannot redeem it", func(t *testing.T) {
		repo := new(MockPromoRepository)
		userRepo := new(MockUserRepository)
		userRepo.On("FindUserByID", mock.Anything, 42).Return(activeClient(), nil).Once()
		repo.On("GetByCode", mock.Anything, "PARTNERHOTEL").Return(internalPromo(), nil).Once()

		result, err := NewPromotionService(repo, userRepo).
			ValidateForClient(context.Background(), 42, "PARTNERHOTEL", 1000)

		assert.NoError(t, err)
		assert.False(t, result.Valid)
		// Indistinguishable from an unknown code so internal codes cannot be found by guessing.
		assert.Equal(t, "Invalid code", result.Message)
		assert.Zero(t, result.DiscountAmount)
	})

	t.Run("staff can apply it for a client", func(t *testing.T) {
		repo := new(MockPromoRepository)
		userRepo := new(MockUserRepository)
		userRepo.On("FindUserByID", mock.Anything, 42).Return(activeClient(), nil).Once()
		repo.On("GetByCode", mock.Anything, "PARTNERHOTEL").Return(internalPromo(), nil).Once()

		result, err := NewPromotionService(repo, userRepo).
			ValidateForStaff(context.Background(), 42, "PARTNERHOTEL", 1000)

		assert.NoError(t, err)
		assert.True(t, result.Valid)
		assert.Equal(t, 1000.0, result.DiscountAmount)
	})
}
