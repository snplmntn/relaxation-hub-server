package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

func ptrInt(i int) *int { return &i }

type mockPromotionRepo struct {
	getFunc  func(code string) (*model.Promotion, error)
	promo    *model.Promotion
	bookings []model.VoucherBooking
}

func (m *mockPromotionRepo) Create(ctx context.Context, p *model.Promotion) error { return nil }
func (m *mockPromotionRepo) ListActive(ctx context.Context, now time.Time, publicOnly bool) ([]model.Promotion, error) {
	return nil, nil
}
func (m *mockPromotionRepo) GetByID(ctx context.Context, promoID int64) (*model.Promotion, error) {
	if m.promo != nil {
		return m.promo, nil
	}
	return nil, pgx.ErrNoRows
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
func (m *mockPromotionRepo) ListAll(ctx context.Context) ([]model.Promotion, error) { return nil, nil }
func (m *mockPromotionRepo) ListBookings(ctx context.Context, promoID int64) ([]model.VoucherBooking, error) {
	return m.bookings, nil
}
func (m *mockPromotionRepo) ListAllVoucherBookings(ctx context.Context) ([]model.VoucherBooking, error) {
	return m.bookings, nil
}
func (m *mockPromotionRepo) Update(ctx context.Context, id int64, updates map[string]interface{}) error {
	return nil
}
func (m *mockPromotionRepo) Delete(ctx context.Context, id int64) error { return nil }

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

func TestGetPromotionBookings_Success(t *testing.T) {
	m := &mockPromotionRepo{
		promo: &model.Promotion{
			PromoID:     5,
			Code:        "VIP20",
			CurrentUses: 1,
		},
		bookings: []model.VoucherBooking{
			{BookingID: 42, ReferenceCode: "HK-1234", Status: "completed"},
		},
	}
	h := NewPromotionHandler(service.NewPromotionService(m))
	req := httptest.NewRequest("GET", "/promotions/5/bookings", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "5")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	h.GetPromotionBookings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var response model.VoucherBookingInventory
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != "VIP20" || response.BookingCount != 1 {
		t.Fatalf("unexpected inventory: %#v", response)
	}
}

func TestListPromotionBookings_Success(t *testing.T) {
	m := &mockPromotionRepo{
		bookings: []model.VoucherBooking{
			{PromoID: 5, VoucherCode: "VIP20", BookingID: 42, Status: "completed"},
		},
	}
	h := NewPromotionHandler(service.NewPromotionService(m))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/promotions/bookings", nil)

	h.ListPromotionBookings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var response model.VoucherBookingLedger
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.VoucherCount != 1 || response.BookingCount != 1 {
		t.Fatalf("unexpected ledger: %#v", response)
	}
}
