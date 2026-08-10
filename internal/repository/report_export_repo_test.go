package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
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
		return strings.Contains(lower, "business_day(b.scheduled_start) = $1") &&
			// Attribution is by the day the session started, not the calendar
			// date it finished on, so a booking running past midnight stays on
			// the sheet for the day it belongs to.
			!strings.Contains(lower, "at time zone 'asia/manila'") &&
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

func TestAllocateBookingExportAmountsSplitsByServicePriceAndPreservesTotals(t *testing.T) {
	items := []model.ReportBookingExportRow{
		{BookingID: 253, DurationMinutes: 60, ServiceDurationWeight: 60, ServicePriceWeight: 549, ServiceCommissionRate: 190, FinalTotal: 1198, TherapistEarnings: 380},
		{BookingID: 253, DurationMinutes: 60, ServiceDurationWeight: 60, ServicePriceWeight: 649, ServiceCommissionRate: 240, AdditionalService: true, FinalTotal: 1198, TherapistEarnings: 380},
	}

	allocateBookingExportAmounts(items)

	assert.Equal(t, 549.0, items[0].FinalTotal)
	assert.Equal(t, 649.0, items[1].FinalTotal)
	assert.Equal(t, 190.0, items[0].TherapistEarnings)
	assert.Equal(t, 240.0, items[1].TherapistEarnings)
	assert.Equal(t, 1198.0, items[0].FinalTotal+items[1].FinalTotal)
	assert.Equal(t, 430.0, items[0].TherapistEarnings+items[1].TherapistEarnings)
}

func TestNormalizeBookingExportDurationsUsesFullBookingDurationForLegacyServices(t *testing.T) {
	items := []model.ReportBookingExportRow{
		{BookingID: 254, DurationMinutes: 60, BookingDurationMinutes: 120, ServiceDurationWeight: 60},
		{BookingID: 300, DurationMinutes: 60, BookingDurationMinutes: 180, ServiceDurationWeight: 60},
		{BookingID: 300, DurationMinutes: 60, BookingDurationMinutes: 180, ServiceDurationWeight: 60, AdditionalService: true},
		{BookingID: 301, DurationMinutes: 60, BookingDurationMinutes: 120, ServiceDurationWeight: 60, DurationAllocated: true},
		{BookingID: 301, DurationMinutes: 60, BookingDurationMinutes: 120, ServiceDurationWeight: 60, DurationAllocated: true, AdditionalService: true},
	}

	normalizeBookingExportDurations(items)

	assert.Equal(t, 120, items[0].DurationMinutes)
	assert.Equal(t, 90, items[1].DurationMinutes)
	assert.Equal(t, 90, items[2].DurationMinutes)
	assert.Equal(t, 60, items[3].DurationMinutes)
	assert.Equal(t, 60, items[4].DurationMinutes)
}
