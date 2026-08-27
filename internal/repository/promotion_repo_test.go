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

// The Vouchers page shows this count as "real usage", so the query must derive
// it from bookings rather than the promotions.current_uses counter, and must
// collapse a group booking's child rows into a single redemption.
func TestListAllCountsRealUsageFromBookings(t *testing.T) {
	db := new(MockDBTX)
	repo := NewPromotionRepository(db)

	rows := new(MockRows)
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil)
	rows.On("Close").Return()

	db.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "from bookings") &&
			// One redemption per group, not per guest.
			strings.Contains(normalized, "count(distinct coalesce('g' || group_id, 'b' || booking_id))") &&
			// A called-off booking is not a use.
			strings.Contains(normalized, "status not in ('cancelled', 'cancelled_by_therapist', 'cancelled_by_client')") &&
			// The stale counter must not be what gets reported.
			!strings.Contains(normalized, "p.current_uses")
	}), mock.Anything).Return(rows, nil).Once()

	out, err := repo.ListAll(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, out)
	db.AssertExpectations(t)
}
