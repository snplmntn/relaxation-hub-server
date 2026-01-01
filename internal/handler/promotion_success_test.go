package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func ptrInt(i int) *int { return &i }

type mockPromotionRepo struct {
	getFunc func(code string) (*model.Promotion, error)
}

func (m *mockPromotionRepo) Create(ctx context.Context, p *model.Promotion) error { return nil }
func (m *mockPromotionRepo) ListActive(ctx context.Context, now time.Time) ([]model.Promotion, error) {
	return nil, nil
}
func (m *mockPromotionRepo) GetByCode(ctx context.Context, code string) (*model.Promotion, error) {
	if m.getFunc != nil {
		return m.getFunc(code)
	}
	return nil, nil
}

func (m *mockPromotionRepo) TryIncrementGlobalUsageTx(ctx context.Context, tx pgx.Tx, promoID int64) (bool, error) {
	return true, nil
}

func (m *mockPromotionRepo) TryIncrementUserPromoUsageTx(ctx context.Context, tx pgx.Tx, promoID, userID int64) (bool, error) {
	return true, nil
}

func TestGetPromotionByCode_Success(t *testing.T) {
	m := &mockPromotionRepo{
		getFunc: func(code string) (*model.Promotion, error) {
			return &model.Promotion{PromoID: 5, Code: code, DiscountPct: ptrInt(10)}, nil
		},
	}

	svc := service.NewPromotionService(m)
	h := NewPromotionHandler(svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/promotions?code=SAVE10", nil)

	h.GetPromotionByCode(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp model.PromotionResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "SAVE10" {
		t.Fatalf("unexpected code: %s", resp.Code)
	}
}
