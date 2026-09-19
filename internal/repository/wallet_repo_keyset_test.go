package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWalletRepoListTransactionsKeyset_UsesDuplicateTimestampSafeCursorWithoutCount(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewWalletRepository(mockDB).(*walletRepoImpl)
	createdAt := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)
	cursor := &model.KeysetCursor{CreatedAt: createdAt, ID: 42}

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from wallet_transactions") &&
			strings.Contains(lower, "wallet_id = $1") &&
			strings.Contains(lower, "created_at < $2") &&
			strings.Contains(lower, "created_at = $2 and transaction_id < $3") &&
			strings.Contains(lower, "order by created_at desc, transaction_id desc") &&
			strings.Contains(lower, "limit $4") &&
			!strings.Contains(lower, "count(*)") &&
			!strings.Contains(lower, "offset")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 4 && args[0] == int64(10) && args[1] == createdAt && args[2] == int64(42) && args[3] == 21
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	transactions, err := repo.ListTransactionsKeyset(context.Background(), 10, cursor, 21)

	assert.NoError(t, err)
	assert.Empty(t, transactions)
	mockDB.AssertNotCalled(t, "QueryRow", mock.Anything, mock.Anything, mock.Anything)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestWalletRepoListTransactionsKeyset_FirstPageUsesBoundedOrderingWithoutCursor(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewWalletRepository(mockDB).(*walletRepoImpl)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from wallet_transactions") &&
			strings.Contains(lower, "where wallet_id = $1") &&
			strings.Contains(lower, "order by created_at desc, transaction_id desc") &&
			strings.Contains(lower, "limit $2") &&
			!strings.Contains(lower, "count(*)") &&
			!strings.Contains(lower, "offset") &&
			!strings.Contains(lower, "created_at <")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == int64(10) && args[1] == 21
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	transactions, err := repo.ListTransactionsKeyset(context.Background(), 10, nil, 21)

	assert.NoError(t, err)
	assert.Empty(t, transactions)
	mockDB.AssertNotCalled(t, "QueryRow", mock.Anything, mock.Anything, mock.Anything)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}
