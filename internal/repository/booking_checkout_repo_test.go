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

// The pooler in production runs pgx in simple-protocol mode, where a []byte
// argument is rendered as a bytea hex literal and Postgres rejects it for a
// jsonb column ("invalid input syntax for type json"). Binding the payload as a
// string is what every other jsonb write in this package does.
func TestBookingCheckoutRepoCreate_BindsPayloadAsString(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewBookingCheckoutRepository(mockDB)

	checkout := &model.BookingCheckout{
		Reference:      "CS_ABC123",
		ClientID:       7930,
		Kind:           model.CheckoutKindSingle,
		Channel:        "gcash",
		RequestPayload: []byte(`{"service_id":1,"payment_method":"online"}`),
		Amount:         1500,
		ExpiresAt:      time.Date(2026, time.August, 23, 12, 0, 0, 0, time.UTC),
	}

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into booking_checkouts")
	}), mock.MatchedBy(func(args []interface{}) bool {
		payload, isString := args[4].(string)
		return len(args) == 7 && isString && payload == string(checkout.RequestPayload)
	})).Return(row).Once()

	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			*args.Get(0).(*int64) = 55
			*args.Get(1).(*string) = "paymongo"
			*args.Get(2).(*string) = model.CheckoutStatusPending
			*args.Get(3).(*time.Time) = checkout.ExpiresAt
			*args.Get(4).(*time.Time) = checkout.ExpiresAt
		}).Return(nil).Once()

	assert.NoError(t, repo.Create(context.Background(), checkout))
	mockDB.AssertExpectations(t)
}
