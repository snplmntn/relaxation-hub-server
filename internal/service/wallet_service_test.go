package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks removed - now using shared mocks in booking_service_mocks_test.go

// --- Repository Mocks ---

type MockWalletRepo struct {
	mock.Mock
}

func (m *MockWalletRepo) GetByTherapistID(ctx context.Context, therapistID int64) (*model.Wallet, error) {
	args := m.Called(ctx, therapistID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Wallet), args.Error(1)
}

func (m *MockWalletRepo) CreateWallet(ctx context.Context, therapistID int64) (*model.Wallet, error) {
	args := m.Called(ctx, therapistID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Wallet), args.Error(1)
}

func (m *MockWalletRepo) UpdateBalances(ctx context.Context, walletID int64, availableDelta, pendingDelta float64) error {
	args := m.Called(ctx, walletID, availableDelta, pendingDelta)
	return args.Error(0)
}

func (m *MockWalletRepo) CreateTransaction(ctx context.Context, txn *model.WalletTransaction) error {
	args := m.Called(ctx, txn)
	return args.Error(0)
}

func (m *MockWalletRepo) ListTransactions(ctx context.Context, walletID int64, limit, offset int) ([]model.WalletTransaction, int, error) {
	args := m.Called(ctx, walletID, limit, offset)
	return args.Get(0).([]model.WalletTransaction), args.Int(1), args.Error(2)
}

func (m *MockWalletRepo) CreatePayoutRequest(ctx context.Context, req *model.PayoutRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockWalletRepo) GetPayoutRequest(ctx context.Context, requestID int64) (*model.PayoutRequest, error) {
	args := m.Called(ctx, requestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.PayoutRequest), args.Error(1)
}

func (m *MockWalletRepo) ListPayoutRequestsByTherapist(ctx context.Context, therapistID int64) ([]model.PayoutRequest, error) {
	args := m.Called(ctx, therapistID)
	return args.Get(0).([]model.PayoutRequest), args.Error(1)
}

func (m *MockWalletRepo) ListPendingPayoutRequests(ctx context.Context) ([]model.PayoutRequest, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.PayoutRequest), args.Error(1)
}

func (m *MockWalletRepo) UpdatePayoutRequestStatus(ctx context.Context, requestID int64, status string, processedBy int64, reason, txnRef *string) error {
	args := m.Called(ctx, requestID, status, processedBy, reason, txnRef)
	return args.Error(0)
}

func (m *MockWalletRepo) CreateCashAdvance(ctx context.Context, adv *model.CashAdvance) error {
	args := m.Called(ctx, adv)
	return args.Error(0)
}

func (m *MockWalletRepo) GetCashAdvance(ctx context.Context, advanceID int64) (*model.CashAdvance, error) {
	args := m.Called(ctx, advanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CashAdvance), args.Error(1)
}

func (m *MockWalletRepo) GetActiveAdvanceByTherapist(ctx context.Context, therapistID int64) (*model.CashAdvance, error) {
	args := m.Called(ctx, therapistID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CashAdvance), args.Error(1)
}

func (m *MockWalletRepo) UpdateAdvanceBalance(ctx context.Context, advanceID int64, repaymentAmount float64) error {
	args := m.Called(ctx, advanceID, repaymentAmount)
	return args.Error(0)
}

func (m *MockWalletRepo) MarkAdvancePaidOff(ctx context.Context, advanceID int64) error {
	args := m.Called(ctx, advanceID)
	return args.Error(0)
}

type mockWalletBookingRepo struct {
	repository.BookingRepository
}

// --- Tests ---

func TestWalletService_GetWalletSummary(t *testing.T) {
	mockDB := new(MockDBTX)
	mockWalletRepo := new(MockWalletRepo)
	mockBookingRepo := new(MockBookingRepository)

	svc := NewWalletService(mockDB, mockWalletRepo, mockBookingRepo)

	t.Run("success", func(t *testing.T) {
		therapistID := int64(1)
		wallet := &model.Wallet{WalletID: 10, TherapistID: therapistID, AvailableBalance: 500}
		txns := []model.WalletTransaction{{TransactionID: 100, Amount: 100}}
		payouts := []model.PayoutRequest{{RequestID: 200, Status: "pending"}}

		mockWalletRepo.On("GetByTherapistID", mock.Anything, therapistID).Return(wallet, nil).Once()
		mockWalletRepo.On("GetActiveAdvanceByTherapist", mock.Anything, therapistID).Return(nil, nil).Once()
		mockWalletRepo.On("ListTransactions", mock.Anything, int64(10), 5, 0).Return(txns, 1, nil).Once()
		mockWalletRepo.On("ListPayoutRequestsByTherapist", mock.Anything, therapistID).Return(payouts, nil).Once()

		summary, err := svc.GetWalletSummary(context.Background(), therapistID)

		assert.NoError(t, err)
		assert.Equal(t, wallet, summary.Wallet)
		assert.Equal(t, txns, summary.RecentTransactions)
		assert.Equal(t, 1, summary.PendingPayouts)
		mockWalletRepo.AssertExpectations(t)
	})

	t.Run("wallet not found", func(t *testing.T) {
		therapistID := int64(2)
		mockWalletRepo.On("GetByTherapistID", mock.Anything, therapistID).Return(nil, errors.New("db error")).Once()

		summary, err := svc.GetWalletSummary(context.Background(), therapistID)

		assert.Error(t, err)
		assert.Nil(t, summary)
		assert.Equal(t, "wallet not found", err.Error())
	})
}

func TestWalletService_RequestPayout(t *testing.T) {
	mockDB := new(MockDBTX)
	mockWalletRepo := new(MockWalletRepo)
	mockBookingRepo := new(MockBookingRepository)

	svc := NewWalletService(mockDB, mockWalletRepo, mockBookingRepo)

	t.Run("success", func(t *testing.T) {
		therapistID := int64(1)
		amount := 100.0
		method := "gcash"
		details := []byte(`{"number": "09123456789"}`)
		wallet := &model.Wallet{
			WalletID:         10,
			AvailableBalance: 500,
			MinimumPayout:    50,
		}

		mockWalletRepo.On("GetByTherapistID", mock.Anything, therapistID).Return(wallet, nil).Once()
		mockWalletRepo.On("CreatePayoutRequest", mock.Anything, mock.MatchedBy(func(req *model.PayoutRequest) bool {
			return req.Amount == amount && req.TherapistID == therapistID && req.PayoutMethod == method
		})).Return(nil).Once()

		req, err := svc.RequestPayout(context.Background(), therapistID, amount, method, details)

		assert.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, amount, req.Amount)
		mockWalletRepo.AssertExpectations(t)
	})

	t.Run("insufficient balance", func(t *testing.T) {
		therapistID := int64(1)
		amount := 1000.0
		wallet := &model.Wallet{
			WalletID:         10,
			AvailableBalance: 500,
			MinimumPayout:    50,
		}

		mockWalletRepo.On("GetByTherapistID", mock.Anything, therapistID).Return(wallet, nil).Once()

		req, err := svc.RequestPayout(context.Background(), therapistID, amount, "gcash", nil)

		assert.Error(t, err)
		assert.Nil(t, req)
		assert.Contains(t, err.Error(), "Insufficient balance")
	})
}

func TestWalletService_CreditEarning(t *testing.T) {
	mockDB := new(MockDBTX)
	mockWalletRepo := new(MockWalletRepo)
	mockBookingRepo := new(MockBookingRepository)

	svc := NewWalletService(mockDB, mockWalletRepo, mockBookingRepo)

	t.Run("success with no cash advance", func(t *testing.T) {
		ctx := context.Background()
		therapistID := int64(1)
		bookingID := int64(100)
		amount := 1000.0
		walletID := int64(10)

		mockTx := new(MockTx)
		mockDB.On("Begin", ctx).Return(mockTx, nil).Once()
		mockTx.On("Rollback", mock.Anything).Return(nil).Once()

		rowWallet := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "FROM therapist_wallets") }), []interface{}{therapistID}).Return(rowWallet).Once()
		rowWallet.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = walletID
				*args.Get(1).(*int64) = therapistID
			}).Return(nil).Once()

		rowAdvance := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "FROM cash_advances") }), []interface{}{therapistID}).Return(rowAdvance).Once()
		rowAdvance.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(pgx.ErrNoRows).Once()

		mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "UPDATE therapist_wallets") }), []interface{}{0.0, amount, walletID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		rowTxn := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "INSERT INTO wallet_transactions") }), mock.Anything).Return(rowTxn).Once()
		rowTxn.On("Scan", mock.Anything, mock.Anything).Return(nil).Once()

		mockTx.On("Commit", mock.Anything).Return(nil).Once()

		err := svc.CreditEarning(ctx, therapistID, bookingID, amount, nil)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
	})
}

func TestWalletService_ReleaseEarning(t *testing.T) {
	mockDB := new(MockDBTX)
	mockWalletRepo := new(MockWalletRepo)
	mockBookingRepo := new(MockBookingRepository)

	svc := NewWalletService(mockDB, mockWalletRepo, mockBookingRepo)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		therapistID := int64(1)
		amount := 500.0
		walletID := int64(10)

		mockTx := new(MockTx)
		mockDB.On("Begin", ctx).Return(mockTx, nil).Once()
		mockTx.On("Rollback", mock.Anything).Return(nil).Once()

		rowWallet := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "FROM therapist_wallets") }), []interface{}{therapistID}).Return(rowWallet).Once()
		rowWallet.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = walletID
				*args.Get(2).(*float64) = 1000.0
				*args.Get(3).(*float64) = 1500.0
			}).Return(nil).Once()

		// 2. UpdateBalances (availableDelta: amount, pendingDelta: -amount)
		mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE therapist_wallets")
		}), []interface{}{amount, -amount, walletID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		// 3. CreateTransaction
		rowTxn := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "INSERT INTO wallet_transactions")
		}), mock.Anything).Return(rowTxn).Once()
		rowTxn.On("Scan", mock.Anything, mock.Anything).Return(nil).Once()

		mockTx.On("Commit", mock.Anything).Return(nil).Once()

		err := svc.ReleaseEarning(ctx, therapistID, amount)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
	})
}

func TestWalletService_ApprovePayout(t *testing.T) {
	mockDB := new(MockDBTX)
	mockWalletRepo := new(MockWalletRepo)
	mockBookingRepo := new(MockBookingRepository)

	svc := NewWalletService(mockDB, mockWalletRepo, mockBookingRepo)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		payoutID := int64(500)
		adminID := int64(7)
		txnRef := "TXN-123"
		therapistID := int64(1)
		walletID := int64(10)
		amount := 1000.0

		mockTx := new(MockTx)
		mockDB.On("Begin", ctx).Return(mockTx, nil).Once()
		mockTx.On("Rollback", mock.Anything).Return(nil).Once()

		// 1. GetPayoutRequest
		rowReq := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "FROM payout_requests") }), []interface{}{payoutID}).Return(rowReq).Once()
		rowReq.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = payoutID
				*args.Get(1).(*int64) = walletID
				*args.Get(2).(*int64) = therapistID
				*args.Get(3).(*float64) = amount
				*args.Get(6).(*string) = "pending"
			}).Return(nil).Once()

		// 2. GetByTherapistID
		rowWallet := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "FROM therapist_wallets") }), []interface{}{therapistID}).Return(rowWallet).Once()
		rowWallet.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = walletID
				*args.Get(2).(*float64) = 5000.0 // Available
			}).Return(nil).Once()

		// 3. UpdatePayoutRequestStatus
		mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "UPDATE payout_requests") }), []interface{}{"completed", adminID, (*string)(nil), &txnRef, payoutID}).
			Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		// 4. UpdateBalances (availableDelta: -amount, pendingDelta: 0)
		mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE therapist_wallets")
		}), []interface{}{-amount, 0.0, walletID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		// 5. CreateTransaction
		rowTxn := new(MockRow)
		mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool { return contains(sql, "INSERT INTO wallet_transactions") }), mock.Anything).Return(rowTxn).Once()
		rowTxn.On("Scan", mock.Anything, mock.Anything).Return(nil).Once()

		mockTx.On("Commit", mock.Anything).Return(nil).Once()

		err := svc.ApprovePayout(ctx, payoutID, adminID, &txnRef)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
	})
}

func TestWalletService_RejectPayout(t *testing.T) {
	mockDB := new(MockDBTX)
	mockWalletRepo := new(MockWalletRepo)
	mockBookingRepo := new(MockBookingRepository)

	svc := NewWalletService(mockDB, mockWalletRepo, mockBookingRepo)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		payoutID := int64(500)
		adminID := int64(7)
		reason := "invalid account"
		walletID := int64(10)
		amount := 1000.0

		// RejectPayout uses s.walletRepo directly in the current implementation.
		mockWalletRepo.On("GetPayoutRequest", mock.Anything, payoutID).Return(&model.PayoutRequest{
			RequestID:   payoutID,
			WalletID:    walletID,
			Amount:      amount,
			Status:      "pending",
			TherapistID: 1,
		}, nil).Once()

		mockWalletRepo.On("UpdatePayoutRequestStatus", mock.Anything, payoutID, "rejected", adminID, &reason, (*string)(nil)).Return(nil).Once()

		err := svc.RejectPayout(ctx, payoutID, adminID, reason)

		assert.NoError(t, err)
		mockWalletRepo.AssertExpectations(t)
	})
}
