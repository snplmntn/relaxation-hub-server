package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListByBookingIDsWithServiceCoalescesNullableCatalogFields(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewBookingServiceRepository(mockDB)
	bookingIDs := []int64{42, 43}

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		query := strings.ToLower(sql)
		return strings.Contains(query, "coalesce(s.description, '')") &&
			strings.Contains(query, "coalesce(s.category, '')") &&
			strings.Contains(query, "coalesce(s.preview_image_url, '')") &&
			strings.Contains(query, "coalesce(s.subtitle, '')") &&
			strings.Contains(query, "bs.booking_id = any($1)")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	services, err := repo.ListByBookingIDsWithService(context.Background(), bookingIDs)

	assert.NoError(t, err)
	assert.Empty(t, services)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}
