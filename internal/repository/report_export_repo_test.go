package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReportExportRepoListDailySalesBookingRows_UsesManilaBusinessDateFromUTCAndHistoricalBranch(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewReportExportRepository(mockDB).(*reportExportRepoImpl)
	businessDate := time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "date((b.actual_end at time zone 'utc') at time zone 'asia/manila')") &&
			strings.Contains(lower, "left join therapist_profiles tp on tp.therapist_id = b.therapist_id") &&
			!strings.Contains(lower, "tp.deleted_at is null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == "2026-02-10"
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Err").Return(nil).Once()
	rows.On("Close").Return().Once()

	items, err := repo.ListDailySalesBookingRows(context.Background(), businessDate)

	require.NoError(t, err)
	assert.Empty(t, items)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}
