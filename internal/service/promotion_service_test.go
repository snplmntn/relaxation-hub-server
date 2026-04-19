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
