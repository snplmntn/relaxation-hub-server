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

func TestRiderWalletService_GetWallet(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		riderID := int64(1)
		now := time.Now()

		row := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "FROM rider_wallets")
		}), []interface{}{riderID}).Return(row).Once()

		row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = riderID
				*args.Get(1).(*int) = 1000
				*args.Get(2).(*int) = 5000
				*args.Get(3).(*int) = 4000
				*args.Get(4).(*time.Time) = now
				*args.Get(5).(*time.Time) = now
			}).Return(nil).Once()

		wallet, err := svc.GetWallet(ctx, riderID)

		assert.NoError(t, err)
		assert.NotNil(t, wallet)
		assert.Equal(t, riderID, wallet.RiderID)
		assert.Equal(t, 1000, wallet.BalanceCents)
		mockDB.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		ctx := context.Background()
		riderID := int64(2)

		row := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.Anything, []interface{}{riderID}).Return(row).Once()
		row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(pgx.ErrNoRows).Once()

		wallet, err := svc.GetWallet(ctx, riderID)

		assert.Error(t, err)
		assert.Nil(t, wallet)
		mockDB.AssertExpectations(t)
	})
}

func TestRiderWalletService_GetTransactions(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		riderID := int64(1)
		limit, offset := 10, 0
		now := time.Now()

		rows := new(MockRows)
		mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "FROM rider_transactions")
		}), []interface{}{riderID, limit, offset}).Return(rows, nil).Once()

		rows.On("Next").Return(true).Once()
		rows.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int) = 1
				*args.Get(1).(*int64) = riderID
				*args.Get(2).(*string) = "ride_earning"
				*args.Get(3).(*int) = 500
				*args.Get(7).(*time.Time) = now
			}).Return(nil).Once()
		rows.On("Next").Return(false).Once()
		rows.On("Close").Return(nil).Once()

		txs, err := svc.GetTransactions(ctx, riderID, limit, offset)

		assert.NoError(t, err)
		assert.Len(t, txs, 1)
		mockDB.AssertExpectations(t)
	})
}

func TestRiderWalletService_RequestPayout(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		riderID := int64(1)
		amount := 15000 // 150 PHP
		now := time.Now()

		// 1. GetWallet
		rowWallet := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "FROM rider_wallets")
		}), []interface{}{riderID}).Return(rowWallet).Once()
		rowWallet.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = riderID
				*args.Get(1).(*int) = 20000
				*args.Get(4).(*time.Time) = now
				*args.Get(5).(*time.Time) = now
			}).Return(nil).Once()

		// 2. Insert transaction
		rowReq := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "INSERT INTO rider_transactions")
		}), []interface{}{riderID, -amount, fmt.Sprintf("Payout request for ₱%.2f", float64(amount)/100)}).Return(rowReq).Once()
		rowReq.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
			*args.Get(0).(*int) = 100
		}).Return(nil).Once()

		err := svc.RequestPayout(ctx, riderID, amount)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestRiderWalletService_ApprovePayout(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		txID := 100
		riderID := int64(1)
		amount := -15000

		// 1. Get transaction
		rowTx := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "FROM rider_transactions")
		}), []interface{}{txID}).Return(rowTx).Once()
		rowTx.On("Scan", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = riderID
				*args.Get(1).(*int) = amount
				*args.Get(2).(*string) = "pending"
			}).Return(nil).Once()

		// 2. Begin transaction
		mockTx := new(MockTx)
		mockDB.On("Begin", mock.Anything).Return(mockTx, nil).Once()
		mockTx.On("Rollback", mock.Anything).Return(nil).Once()

		// 3. Update wallet
		mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE rider_wallets")
		}), []interface{}{amount, -amount, riderID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		// 4. Update transaction
		mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE rider_transactions")
		}), []interface{}{txID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		mockTx.On("Commit", mock.Anything).Return(nil).Once()

		err := svc.ApprovePayout(ctx, txID)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
	})
}

func TestRiderWalletService_RecordEarnings(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		rideID, riderID := int64(100), int64(1)
		earnings := 500

		mockTx := new(MockTx)
		mockDB.On("Begin", mock.Anything).Return(mockTx, nil).Once()
		mockTx.On("Rollback", mock.Anything).Return(nil).Once()

		mockTx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "UPDATE rides")
		}), []interface{}{earnings, rideID, riderID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		mockTx.On("Commit", mock.Anything).Return(nil).Once()

		err := svc.RecordEarnings(ctx, rideID, riderID, earnings)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
	})
}

func TestRiderWalletService_GetPerformanceMetrics(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		riderID := int64(1)
		now := time.Now()

		row := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "FROM rider_performance_metrics")
		}), []interface{}{riderID}).Return(row).Once()

		row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				*args.Get(0).(*int64) = riderID
				*args.Get(1).(*int) = 100
				*args.Get(10).(*time.Time) = now
			}).Return(nil).Once()

		rowEarnings := new(MockRow)
		mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "FROM rider_transactions") && contains(sql, "ride_earning")
		}), []interface{}{riderID}).Return(rowEarnings).Once()

		rowEarnings.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
			*args.Get(0).(*int) = 5000 // 50.00
		}).Return(nil).Once()

		metrics, err := svc.GetPerformanceMetrics(ctx, riderID)

		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		assert.Equal(t, riderID, metrics.RiderID)
		assert.Equal(t, 5000, metrics.TodayEarnedCents)

		mockDB.AssertExpectations(t)
	})
}

func TestRiderWalletService_IncrementOffersReceived(t *testing.T) {
	mockDB := new(MockDBTX)
	svc := NewRiderWalletService(mockDB)

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		riderID := int64(1)

		mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return contains(sql, "total_offers_received = total_offers_received + 1")
		}), []interface{}{riderID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		err := svc.IncrementOffersReceived(ctx, riderID)

		assert.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}
