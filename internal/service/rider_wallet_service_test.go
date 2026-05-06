package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRiderWalletService_GetWallet_InitializesMissingRows(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)
	ctx := context.Background()
	riderID := int64(2)
	now := time.Now()

	firstRow := new(MockRow)
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_wallets")
	}), []interface{}{riderID}).Return(firstRow).Once()
	firstRow.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(pgx.ErrNoRows).Once()

	mockTx := new(MockTx)
	mockDB.On("Begin", mock.Anything).Return(mockTx, nil).Once()
	mockTx.On("Rollback", mock.Anything).Return(nil).Once()
	mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "INSERT INTO rider_wallets")
	}), []interface{}{riderID}).Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Once()
	mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "INSERT INTO rider_performance_metrics")
	}), []interface{}{riderID}).Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Once()
	mockTx.On("Commit", mock.Anything).Return(nil).Once()

	secondRow := new(MockRow)
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_wallets")
	}), []interface{}{riderID}).Return(secondRow).Once()
	secondRow.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*args.Get(0).(*int64) = riderID
			*args.Get(1).(*int) = 0
			*args.Get(2).(*int) = 0
			*args.Get(3).(*int) = 0
			*args.Get(4).(*time.Time) = now
			*args.Get(5).(*time.Time) = now
		}).Return(nil).Once()

	wallet, err := svc.GetWallet(ctx, riderID)
	assert.NoError(t, err)
	assert.NotNil(t, wallet)
	assert.Equal(t, riderID, wallet.RiderID)
	mockDB.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestRiderWalletService_RequestPayout_RespectsPendingReservations(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)
	ctx := context.Background()
	riderID := int64(1)

	mockTx := new(MockTx)
	mockDB.On("Begin", mock.Anything).Return(mockTx, nil).Once()
	mockTx.On("Rollback", mock.Anything).Return(nil).Once()

	mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "INSERT INTO rider_wallets")
	}), []interface{}{riderID}).Return(pgconn.NewCommandTag("INSERT 0 0"), nil).Once()
	mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "INSERT INTO rider_performance_metrics")
	}), []interface{}{riderID}).Return(pgconn.NewCommandTag("INSERT 0 0"), nil).Once()

	rowWallet := new(MockRow)
	mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_wallets") && contains(sql, "FOR UPDATE")
	}), []interface{}{riderID}).Return(rowWallet).Once()
	rowWallet.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int) = 20000
	}).Return(nil).Once()

	rowMethod := new(MockRow)
	mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_payout_methods")
	}), []interface{}{1, riderID}).Return(rowMethod).Once()
	rowMethod.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	rowReserved := new(MockRow)
	mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "SUM(-amount_cents)")
	}), []interface{}{riderID}).Return(rowReserved).Once()
	rowReserved.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int) = 15000
	}).Return(nil).Once()

	err := svc.RequestPayout(ctx, riderID, 10000, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient available balance")
	mockDB.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestRiderWalletService_ApprovePayout_InsufficientBalance(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)
	ctx := context.Background()
	txID := 100

	mockTx := new(MockTx)
	mockDB.On("Begin", mock.Anything).Return(mockTx, nil).Once()
	mockTx.On("Rollback", mock.Anything).Return(nil).Once()

	rowPayout := new(MockRow)
	mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_transactions") && contains(sql, "FOR UPDATE")
	}), []interface{}{txID}).Return(rowPayout).Once()
	rowPayout.On("Scan", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*args.Get(0).(*int64) = int64(1)
			*args.Get(1).(*int) = -25000
			*args.Get(2).(*string) = "pending"
		}).Return(nil).Once()

	rowWallet := new(MockRow)
	mockTx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_wallets") && contains(sql, "FOR UPDATE")
	}), []interface{}{int64(1)}).Return(rowWallet).Once()
	rowWallet.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int) = 10000
	}).Return(nil).Once()

	err := svc.ApprovePayout(ctx, txID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient wallet balance")
	mockDB.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestRiderWalletService_GetPerformanceMetrics_InitializesMissingRows(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)
	ctx := context.Background()
	riderID := int64(7)
	now := time.Now()

	rowMissing := new(MockRow)
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_performance_metrics")
	}), []interface{}{riderID}).Return(rowMissing).Once()
	rowMissing.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(pgx.ErrNoRows).Once()

	mockTx := new(MockTx)
	mockDB.On("Begin", mock.Anything).Return(mockTx, nil).Once()
	mockTx.On("Rollback", mock.Anything).Return(nil).Once()
	mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "INSERT INTO rider_wallets")
	}), []interface{}{riderID}).Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Once()
	mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "INSERT INTO rider_performance_metrics")
	}), []interface{}{riderID}).Return(pgconn.NewCommandTag("INSERT 0 1"), nil).Once()
	mockTx.On("Commit", mock.Anything).Return(nil).Once()

	rowMetrics := new(MockRow)
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_performance_metrics")
	}), []interface{}{riderID}).Return(rowMetrics).Once()
	rowMetrics.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*args.Get(0).(*int64) = riderID
			*args.Get(10).(*time.Time) = now
		}).Return(nil).Once()

	rowEarnings := new(MockRow)
	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_transactions") && contains(sql, "ride_earning")
	}), []interface{}{riderID}).Return(rowEarnings).Once()
	rowEarnings.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int) = 0
	}).Return(nil).Once()

	metrics, err := svc.GetPerformanceMetrics(ctx, riderID)
	assert.NoError(t, err)
	assert.Equal(t, riderID, metrics.RiderID)
	mockDB.AssertExpectations(t)
	mockTx.AssertExpectations(t)
}

func TestRiderWalletService_ListPendingRiderPayouts_PositiveAmounts(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)
	ctx := context.Background()
	now := time.Now()

	rows := new(MockRows)
	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return contains(sql, "FROM rider_transactions rt")
	}), []interface{}(nil)).Return(rows, nil).Once()

	rows.On("Next").Return(true).Once()
	rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*args.Get(0).(*int) = 10
			*args.Get(1).(*int64) = 5
			*args.Get(2).(*string) = "Rider"
			*args.Get(3).(*int) = -12000
			*args.Get(4).(*string) = "pending"
			*args.Get(6).(*time.Time) = now
		}).Return(nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()

	items, err := svc.ListPendingRiderPayouts(ctx)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, 12000, items[0].AmountCents)
	assert.Equal(t, 120.0, items[0].AmountPHP)
}

func TestNormalizePhilippinePhone(t *testing.T) {
	testCases := []struct {
		input      string
		want       string
		shouldFail bool
	}{
		{input: "09171234567", want: "+639171234567"},
		{input: "639171234567", want: "+639171234567"},
		{input: "+639171234567", want: "+639171234567"},
		{input: "0917-123-4567", want: "+639171234567"},
		{input: "12345", shouldFail: true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("phone_%s", tc.input), func(t *testing.T) {
			got, err := normalizePhilippinePhone(tc.input)
			if tc.shouldFail {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
