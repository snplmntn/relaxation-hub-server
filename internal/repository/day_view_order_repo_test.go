package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

func TestDayViewOrderRepoGetTherapistEarningsBetweenCastsTimestampBounds(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewDayViewOrderRepository(mockDB)

	start := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "actual_end >= $2::timestamp") &&
			strings.Contains(normalized, "actual_end < $3::timestamp")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 3 &&
			args[1] == start &&
			args[2] == end
	})).Return(rows, nil).Once()

	rows.On("Close").Return().Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()

	earnings, err := repo.GetTherapistEarningsBetween(context.Background(), []int64{10, 11}, start, end)

	if err != nil {
		t.Fatalf("GetTherapistEarningsBetween returned error: %v", err)
	}
	if len(earnings) != 0 {
		t.Fatalf("expected no earnings rows, got %#v", earnings)
	}
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}
