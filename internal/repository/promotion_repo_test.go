package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type promotionIntRow struct {
	value int
}

func (r promotionIntRow) Scan(dest ...any) error {
	*(dest[0].(*int)) = r.value
	return nil
}

type promotionErrorRow struct {
	err error
}

func (r promotionErrorRow) Scan(...any) error {
	return r.err
}

func TestTryIncrementGlobalUsageTxTreatsZeroAsUnlimited(t *testing.T) {
	tx := new(MockTx)
	repo := NewPromotionRepository(new(MockDBTX))
	const promoID int64 = 55

	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "select coalesce(max_uses, 0)")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == promoID
	})).Return(promotionIntRow{value: 0}).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "from bookings") &&
			strings.Contains(normalized, "count(distinct coalesce('g' || group_id, 'b' || booking_id))")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == promoID
	})).Return(promotionIntRow{value: 4}).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "set current_uses = $2")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == promoID && args[1] == 5
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

	tx.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).
		Return(promotionIntRow{value: 1}).Twice()

	incremented, err := repo.TryIncrementGlobalUsageTx(context.Background(), tx, promoID)

	assert.NoError(t, err)
	assert.False(t, incremented)
	tx.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
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

func TestGetByCodeUsesRealBookingUsageForLimitChecks(t *testing.T) {
	db := new(MockDBTX)
	repo := NewPromotionRepository(db)

	db.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "from bookings") &&
			strings.Contains(normalized, "b.promo_id = p.promo_id") &&
			strings.Contains(normalized, "count(distinct coalesce('g' || b.group_id, 'b' || b.booking_id))") &&
			!strings.Contains(normalized, "p.current_uses")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == "SAVE10"
	})).Return(promotionErrorRow{err: pgx.ErrNoRows}).Once()

	_, err := repo.GetByCode(context.Background(), "SAVE10")

	assert.ErrorIs(t, err, pgx.ErrNoRows)
	db.AssertExpectations(t)
}

func TestListBookingsProvidesVoucherAuditDetails(t *testing.T) {
	db := new(MockDBTX)
	repo := NewPromotionRepository(db)
	rows := new(MockRows)
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil)
	rows.On("Close").Return()

	db.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "where b.promo_id = $1") &&
			strings.Contains(normalized, "left join users client") &&
			strings.Contains(normalized, "left join users therapist") &&
			strings.Contains(normalized, "from booking_services") &&
			!strings.Contains(normalized, "status not in")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == int64(19)
	})).Return(rows, nil).Once()

	bookings, err := repo.ListBookings(context.Background(), 19)

	assert.NoError(t, err)
	assert.Empty(t, bookings)
	db.AssertExpectations(t)
}

func TestListAllVoucherBookingsDoesNotRequireAPromotionFilter(t *testing.T) {
	db := new(MockDBTX)
	repo := NewPromotionRepository(db)
	rows := new(MockRows)
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil)
	rows.On("Close").Return()

	db.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "join promotions p on p.promo_id = b.promo_id") &&
			strings.Contains(normalized, "where b.promo_id is not null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 0
	})).Return(rows, nil).Once()

	bookings, err := repo.ListAllVoucherBookings(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, bookings)
	db.AssertExpectations(t)
}
