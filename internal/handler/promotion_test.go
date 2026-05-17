package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type promotionRepoStub struct{}

func (promotionRepoStub) Create(context.Context, *model.Promotion) error { return nil }
func (promotionRepoStub) ListActive(context.Context, time.Time) ([]model.Promotion, error) {
	return nil, nil
}
func (promotionRepoStub) GetByCode(context.Context, string) (*model.Promotion, error) {
	return nil, nil
}
func (promotionRepoStub) TryIncrementGlobalUsageTx(context.Context, pgx.Tx, int64) (bool, error) {
	return false, nil
}
func (promotionRepoStub) TryIncrementUserPromoUsageTx(context.Context, pgx.Tx, int64, int64) (bool, error) {
	return false, nil
}
func (promotionRepoStub) ListAll(context.Context) ([]model.Promotion, error)          { return nil, nil }
func (promotionRepoStub) Update(context.Context, int64, map[string]interface{}) error { return nil }
func (promotionRepoStub) Delete(context.Context, int64) error                         { return nil }

var _ repository.PromotionRepository = promotionRepoStub{}

func TestCreatePromotion_InvalidBody_ReturnsStructuredValidationError(t *testing.T) {
	h := NewPromotionHandler((*service.PromotionService)(nil))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/promotions", bytes.NewBufferString("not-json"))

	h.CreatePromotion(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode response: %v; body: %s", err, rr.Body.String())
	}

	if er.Code != "invalid_request_body" {
		t.Errorf("expected Code 'invalid_request_body', got %q", er.Code)
	}
	if er.Message != "invalid request body" {
		t.Errorf("expected Message 'invalid request body', got %q", er.Message)
	}
}

func TestUpdatePromotion_InvalidAppliesTo_ReturnsStructuredValidationError(t *testing.T) {
	h := NewPromotionHandler(service.NewPromotionService(promotionRepoStub{}))

	body := bytes.NewBufferString(`{"applies_to":"everything"}`)
	req := httptest.NewRequest("PATCH", "/promotions/12", body)
	rr := httptest.NewRecorder()

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	h.UpdatePromotion(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	var er ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&er); err != nil {
		t.Fatalf("failed to decode response: %v; body: %s", err, rr.Body.String())
	}

	if er.Code != "invalid_applies_to" {
		t.Errorf("expected Code 'invalid_applies_to', got %q", er.Code)
	}
	if er.Message != "applies_to must be full_basket or services_only" {
		t.Errorf("expected validation message, got %q", er.Message)
	}
}
