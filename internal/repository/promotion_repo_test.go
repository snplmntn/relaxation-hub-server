package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTryIncrementGlobalUsageTxTreatsZeroAsUnlimited(t *testing.T) {
	tx := new(MockTx)
	repo := NewPromotionRepository(new(MockDBTX))
	const promoID int64 = 55

	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "max_uses is null or max_uses <= 0 or current_uses < max_uses")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == promoID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	incremented, err := repo.TryIncrementGlobalUsageTx(context.Background(), tx, promoID)

	assert.NoError(t, err)
	assert.True(t, incremented)
	tx.AssertExpectations(t)
}

func TestTryIncrementGlobalUsageTxReportsExhaustedVoucher(t *testing.T) {
	tx := new(MockTx)
	repo := NewPromotionRepository(new(MockDBTX))
	const promoID int64 = 56

	tx.On("Exec", mock.Anything, mock.Anything, mock.Anything).
		Return(pgconn.NewCommandTag("UPDATE 0"), nil).
		Once()

	incremented, err := repo.TryIncrementGlobalUsageTx(context.Background(), tx, promoID)

	assert.NoError(t, err)
	assert.False(t, incremented)
	tx.AssertExpectations(t)
}
