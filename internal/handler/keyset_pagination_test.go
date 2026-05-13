package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/middleware"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
	"github.com/stretchr/testify/assert"
)

const keysetPaginationTestJWTSecret = "test-secret"

func TestParseKeysetPaginationQuery_DefaultsLimitToTwenty(t *testing.T) {
	req := httptest.NewRequest("GET", "/notifications", nil)

	cursor, limit, err := parseKeysetPaginationQuery(req)

	assert.NoError(t, err)
	assert.Nil(t, cursor)
	assert.Equal(t, 20, limit)
}

func TestParseKeysetPaginationQuery_ClampsLimitToOneHundred(t *testing.T) {
	req := httptest.NewRequest("GET", "/wallet/transactions?limit=500", nil)

	cursor, limit, err := parseKeysetPaginationQuery(req)

	assert.NoError(t, err)
	assert.Nil(t, cursor)
	assert.Equal(t, 100, limit)
}

func TestParseKeysetPaginationQuery_ParsesCompleteCursor(t *testing.T) {
	createdAt := time.Date(2026, time.May, 11, 10, 0, 0, 123, time.UTC)
	req := httptest.NewRequest("GET", "/notifications?cursor_created_at="+createdAt.Format(time.RFC3339Nano)+"&cursor_id=42&limit=10", nil)

	cursor, limit, err := parseKeysetPaginationQuery(req)

	assert.NoError(t, err)
	assert.Equal(t, createdAt, cursor.CreatedAt)
	assert.Equal(t, int64(42), cursor.ID)
	assert.Equal(t, 10, limit)
}

func TestParseKeysetPaginationQuery_RejectsPartialCursor(t *testing.T) {
	req := httptest.NewRequest("GET", "/notifications?cursor_created_at=2026-05-11T10:00:00Z", nil)

	cursor, _, err := parseKeysetPaginationQuery(req)

	assert.Error(t, err)
	assert.Nil(t, cursor)
	assert.Contains(t, err.Error(), "cursor_created_at and cursor_id must be provided together")
}

func TestParseKeysetPaginationQuery_RejectsInvalidCursorTimestamp(t *testing.T) {
	req := httptest.NewRequest("GET", "/notifications?cursor_created_at=not-a-time&cursor_id=42", nil)

	cursor, _, err := parseKeysetPaginationQuery(req)

	assert.Error(t, err)
	assert.Nil(t, cursor)
	assert.Contains(t, err.Error(), "invalid cursor_created_at")
}

func TestParseKeysetPaginationQuery_RejectsInvalidCursorID(t *testing.T) {
	req := httptest.NewRequest("GET", "/wallet/transactions?cursor_created_at=2026-05-11T10:00:00Z&cursor_id=0", nil)

	cursor, _, err := parseKeysetPaginationQuery(req)

	assert.Error(t, err)
	assert.Nil(t, cursor)
	assert.Contains(t, err.Error(), "invalid cursor_id")
}

func TestNotificationHandlerListNotifications_InvalidCursorReturnsBadRequest(t *testing.T) {
	h := NewNotificationHandler(nil)
	req := authenticatedKeysetRequest(t, "GET", "/notifications?cursor_created_at=bad&cursor_id=42")
	rr := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(h.ListNotifications), keysetPaginationTestJWTSecret).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid cursor_created_at")
}

func TestWalletHandlerGetTransactions_InvalidCursorReturnsBadRequest(t *testing.T) {
	h := NewWalletHandler(nil)
	req := authenticatedKeysetRequest(t, "GET", "/wallet/transactions?cursor_created_at=2026-05-11T10:00:00Z&cursor_id=0")
	rr := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(h.GetTransactions), keysetPaginationTestJWTSecret).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid cursor_id")
}

func TestNotificationHandlerListNotifications_KeysetHappyPathReturnsHasMore(t *testing.T) {
	createdAt := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)
	repo := &fakeNotificationKeysetRepo{notifications: []model.Notification{
		{NotificationID: 5, UserID: 7, Type: "booking", Title: "first", CreatedAt: createdAt, UpdatedAt: createdAt},
		{NotificationID: 4, UserID: 7, Type: "booking", Title: "second", CreatedAt: createdAt, UpdatedAt: createdAt},
		{NotificationID: 3, UserID: 7, Type: "booking", Title: "third", CreatedAt: createdAt, UpdatedAt: createdAt},
	}}
	h := NewNotificationHandler(service.NewNotificationService(repo, nil, nil))
	req := authenticatedKeysetRequest(t, "GET", "/notifications?limit=2")
	rr := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(h.ListNotifications), keysetPaginationTestJWTSecret).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response model.PaginatedNotificationsResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&response))
	assert.Len(t, response.Notifications, 2)
	assert.True(t, response.HasMore)
	assert.Equal(t, 2, response.Limit)
	assert.Equal(t, int64(4), *response.NextCursorID)
	assert.Equal(t, createdAt, *response.NextCursorCreatedAt)
	assert.Equal(t, 3, repo.observedLimit)
	assert.Nil(t, repo.observedCursor)
}

func TestWalletHandlerGetTransactions_KeysetHappyPathReturnsHasMore(t *testing.T) {
	createdAt := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)
	repo := &fakeWalletKeysetRepo{
		wallet: &model.Wallet{WalletID: 10, TherapistID: 7},
		transactions: []model.WalletTransaction{
			{TransactionID: 5, WalletID: 10, CreatedAt: createdAt},
			{TransactionID: 4, WalletID: 10, CreatedAt: createdAt},
			{TransactionID: 3, WalletID: 10, CreatedAt: createdAt},
		},
	}
	h := NewWalletHandler(service.NewWalletService(nil, repo, nil))
	req := authenticatedKeysetRequest(t, "GET", "/wallet/transactions?limit=2")
	rr := httptest.NewRecorder()

	middleware.AuthMiddleware(http.HandlerFunc(h.GetTransactions), keysetPaginationTestJWTSecret).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response model.WalletTransactionsResponse
	assert.NoError(t, json.NewDecoder(rr.Body).Decode(&response))
	assert.Len(t, response.Transactions, 2)
	assert.True(t, response.HasMore)
	assert.Equal(t, 2, response.Limit)
	assert.Equal(t, int64(4), *response.NextCursorID)
	assert.Equal(t, 3, repo.observedLimit)
	assert.Nil(t, repo.observedCursor)
}

func authenticatedKeysetRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	claims := &model.Claims{UserID: 7, Role: model.RoleTherapist}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(keysetPaginationTestJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	return req
}

type fakeNotificationKeysetRepo struct {
	notifications  []model.Notification
	observedCursor *model.KeysetCursor
	observedLimit  int
}

func (r *fakeNotificationKeysetRepo) Create(ctx context.Context, n *model.Notification) error {
	return nil
}

func (r *fakeNotificationKeysetRepo) CreateMany(ctx context.Context, notifications []*model.Notification) error {
	return nil
}

func (r *fakeNotificationKeysetRepo) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Notification, int, error) {
	return nil, 0, nil
}

func (r *fakeNotificationKeysetRepo) ListByUserKeyset(ctx context.Context, userID int64, cursor *model.KeysetCursor, limit int) ([]model.Notification, error) {
	r.observedCursor = cursor
	r.observedLimit = limit
	return r.notifications, nil
}

func (r *fakeNotificationKeysetRepo) CountUnreadByUser(ctx context.Context, userID int64) (int, error) {
	return 0, nil
}

func (r *fakeNotificationKeysetRepo) MarkAsRead(ctx context.Context, notificationID, userID int64) error {
	return nil
}

func (r *fakeNotificationKeysetRepo) MarkAllAsRead(ctx context.Context, userID int64) error {
	return nil
}

type fakeWalletKeysetRepo struct {
	wallet         *model.Wallet
	transactions   []model.WalletTransaction
	observedCursor *model.KeysetCursor
	observedLimit  int
}

func (r *fakeWalletKeysetRepo) GetByTherapistID(ctx context.Context, therapistID int64) (*model.Wallet, error) {
	return r.wallet, nil
}

func (r *fakeWalletKeysetRepo) CreateWallet(ctx context.Context, therapistID int64) (*model.Wallet, error) {
	return nil, nil
}

func (r *fakeWalletKeysetRepo) UpdateBalances(ctx context.Context, walletID int64, availableDelta, pendingDelta float64) error {
	return nil
}

func (r *fakeWalletKeysetRepo) CreateTransaction(ctx context.Context, txn *model.WalletTransaction) error {
	return nil
}

func (r *fakeWalletKeysetRepo) ListTransactions(ctx context.Context, walletID int64, limit, offset int) ([]model.WalletTransaction, int, error) {
	return nil, 0, nil
}

func (r *fakeWalletKeysetRepo) ListTransactionsKeyset(ctx context.Context, walletID int64, cursor *model.KeysetCursor, limit int) ([]model.WalletTransaction, error) {
	r.observedCursor = cursor
	r.observedLimit = limit
	return r.transactions, nil
}

func (r *fakeWalletKeysetRepo) CreatePayoutRequest(ctx context.Context, req *model.PayoutRequest) error {
	return nil
}

func (r *fakeWalletKeysetRepo) GetPayoutRequest(ctx context.Context, requestID int64) (*model.PayoutRequest, error) {
	return nil, pgx.ErrNoRows
}

func (r *fakeWalletKeysetRepo) ListPayoutRequestsByTherapist(ctx context.Context, therapistID int64) ([]model.PayoutRequest, error) {
	return nil, nil
}

func (r *fakeWalletKeysetRepo) ListPendingPayoutRequests(ctx context.Context) ([]model.PayoutRequest, error) {
	return nil, nil
}

func (r *fakeWalletKeysetRepo) UpdatePayoutRequestStatus(ctx context.Context, requestID int64, status string, processedBy int64, reason, txnRef *string) error {
	return nil
}

func (r *fakeWalletKeysetRepo) CreateCashAdvance(ctx context.Context, adv *model.CashAdvance) error {
	return nil
}

func (r *fakeWalletKeysetRepo) GetCashAdvance(ctx context.Context, advanceID int64) (*model.CashAdvance, error) {
	return nil, pgx.ErrNoRows
}

func (r *fakeWalletKeysetRepo) GetActiveAdvanceByTherapist(ctx context.Context, therapistID int64) (*model.CashAdvance, error) {
	return nil, nil
}

func (r *fakeWalletKeysetRepo) UpdateAdvanceBalance(ctx context.Context, advanceID int64, repaymentAmount float64) error {
	return nil
}

func (r *fakeWalletKeysetRepo) MarkAdvancePaidOff(ctx context.Context, advanceID int64) error {
	return nil
}
