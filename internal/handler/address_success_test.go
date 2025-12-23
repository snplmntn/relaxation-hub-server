package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type mockAddressRepo struct {
	createFunc func(ctx context.Context, address *model.Address) error
}

func (m *mockAddressRepo) Create(ctx context.Context, address *model.Address) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, address)
	}
	return nil
}
func (m *mockAddressRepo) GetByID(ctx context.Context, addressID, userID int64) (*model.Address, error) {
	return nil, nil
}
func (m *mockAddressRepo) ListForUser(ctx context.Context, userID int64, includeDeleted bool) ([]model.Address, error) {
	return nil, nil
}
func (m *mockAddressRepo) Update(ctx context.Context, address *model.Address) error      { return nil }
func (m *mockAddressRepo) SetDefault(ctx context.Context, addressID, userID int64) error { return nil }
func (m *mockAddressRepo) SoftDelete(ctx context.Context, addressID, userID int64) error { return nil }

func TestCreateAddress_Success(t *testing.T) {
	m := &mockAddressRepo{
		createFunc: func(ctx context.Context, address *model.Address) error {
			address.AddressID = 99
			address.CreatedAt = time.Now()
			address.UpdatedAt = time.Now()
			return nil
		},
	}

	svc := service.NewAddressService(m, nil)
	h := NewAddressHandler(svc)

	// wrap with auth middleware to inject user id via JWT
	wrapped := middleware.AuthMiddleware(http.HandlerFunc(h.CreateAddress), "tests-secret")

	claims := &model.Claims{UserID: 7, Role: "client"}
	tokenStr, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("tests-secret"))

	body := `{"street_address":"Street 1","city":"City","label":"Home"}`
	req := httptest.NewRequest("POST", "/addresses", bytesFromString(body))
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var resp model.AddressResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AddressID != 99 {
		t.Fatalf("expected address id 99, got %d", resp.AddressID)
	}
}
