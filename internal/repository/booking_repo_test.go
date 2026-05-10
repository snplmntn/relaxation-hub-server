package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBookingRepoCreateTx_PersistsGroupFields(t *testing.T) {
	tx := new(MockTx)
	row := new(MockRow)
	repo := NewBookingRepository(new(MockDBTX))

	groupID := int64(77)
	serviceID := int64(12)
	promoID := int64(55)
	rawTotal := 150.0
	discount := 10.0
	finalTotal := 140.0
	referenceCode := "RH-123"
	now := time.Now().UTC()

	booking := &model.Booking{
		ClientID:        999,
		ServiceID:       &serviceID,
		PromoID:         &promoID,
		PaymentMethod:   "cash",
		DurationMinutes: 60,
		ScheduledStart:  &now,
		RawTotal:        &rawTotal,
		Discount:        &discount,
		FinalTotal:      &finalTotal,
		Status:          model.BookingStatusPending,
		ReferenceCode:   &referenceCode,
		GroupID:         &groupID,
		GuestName:       "Guest 1",
		SequenceNumber:  1,
		StartCondition:  "after_previous",
	}

	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "insert into bookings") &&
			strings.Contains(lower, "group_id") &&
			strings.Contains(lower, "guest_name") &&
			strings.Contains(lower, "sequence_number") &&
			strings.Contains(lower, "start_condition")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) >= 21 &&
			args[17] == booking.GroupID &&
			args[18] == booking.GuestName &&
			args[19] == booking.SequenceNumber &&
			args[20] == booking.StartCondition
	})).Return(row).Once()

	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 101
		*args.Get(1).(*time.Time) = now
		*args.Get(2).(*time.Time) = now
	}).Return(nil).Once()

	err := repo.CreateTx(context.Background(), tx, booking)
	assert.NoError(t, err)
	assert.Equal(t, int64(101), booking.BookingID)

	tx.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestBookingRepoCreateTx_DefaultsEmptyStartCondition(t *testing.T) {
	tx := new(MockTx)
	row := new(MockRow)
	repo := NewBookingRepository(new(MockDBTX))

	serviceID := int64(12)
	referenceCode := "RH-123"
	now := time.Now().UTC()

	booking := &model.Booking{
		ClientID:        999,
		ServiceID:       &serviceID,
		PaymentMethod:   "cash",
		DurationMinutes: 60,
		ScheduledStart:  &now,
		Status:          model.BookingStatusPending,
		ReferenceCode:   &referenceCode,
	}

	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into bookings")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) >= 21 && args[20] == "fixed_time"
	})).Return(row).Once()

	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 101
		*args.Get(1).(*time.Time) = now
		*args.Get(2).(*time.Time) = now
	}).Return(nil).Once()

	err := repo.CreateTx(context.Background(), tx, booking)
	assert.NoError(t, err)
	assert.Equal(t, "fixed_time", booking.StartCondition)

	tx.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestBookingRepoFindNextReturnDestinationBooking_UsesBoundedNonTerminalQuery(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewBookingRepository(mockDB)

	therapistID := int64(22)
	excludeBookingID := int64(11)
	addressID := int64(44)
	after := time.Date(2026, time.May, 10, 17, 0, 0, 0, time.UTC)
	scheduledStart := after.Add(2 * time.Hour)
	now := time.Date(2026, time.May, 10, 12, 0, 0, 0, time.UTC)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from bookings b") &&
			strings.Contains(lower, "join addresses a") &&
			strings.Contains(lower, "b.therapist_id = $1") &&
			strings.Contains(lower, "b.booking_id <> $2") &&
			strings.Contains(lower, "b.scheduled_start > $3") &&
			strings.Contains(lower, "b.status not in") &&
			strings.Contains(lower, "completed") &&
			strings.Contains(lower, "cancelled") &&
			strings.Contains(lower, "no_show") &&
			strings.Contains(lower, "paid") &&
			strings.Contains(lower, "rescheduled") &&
			strings.Contains(lower, "a.latitude is not null") &&
			strings.Contains(lower, "a.longitude is not null") &&
			strings.Contains(lower, "order by b.scheduled_start asc") &&
			strings.Contains(lower, "limit 1")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 3 && args[0] == therapistID && args[1] == excludeBookingID && args[2] == after
	})).Return(row).Once()

	row.On("Scan", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = 99
		*args.Get(2).(*int64) = 77
		*args.Get(3).(**int64) = &therapistID
		*args.Get(6).(**int64) = &addressID
		*args.Get(13).(*int) = 90
		*args.Get(14).(**time.Time) = &scheduledStart
		*args.Get(25).(*string) = model.BookingStatusAssigned
		*args.Get(26).(*time.Time) = now
		*args.Get(27).(*time.Time) = now
		*args.Get(28).(*int) = 0
		*args.Get(30).(*int) = 0
		*args.Get(32).(*string) = "Self"
		*args.Get(33).(*int) = 1
		*args.Get(34).(*string) = "fixed_time"
		*args.Get(35).(*int64) = addressID
		*args.Get(36).(*int64) = 77
		*args.Get(37).(*string) = "Next Client"
		*args.Get(38).(*string) = "Next Street"
		*args.Get(39).(*string) = "Makati"
		lat := 14.55
		lng := 121.02
		*args.Get(40).(**float64) = &lat
		*args.Get(41).(**float64) = &lng
		*args.Get(42).(*bool) = false
		*args.Get(43).(*time.Time) = now
		*args.Get(44).(*time.Time) = now
	}).Return(nil).Once()

	result, err := repo.FindNextReturnDestinationBooking(context.Background(), therapistID, excludeBookingID, after)

	assert.NoError(t, err)
	if assert.NotNil(t, result) {
		assert.Equal(t, int64(99), result.Booking.BookingID)
		assert.Equal(t, addressID, result.Address.AddressID)
	}
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}

func TestBookingRepoUpdateAdmin_PersistsAssignmentStatusAndAssignedAt(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB)

	bookingID := int64(120)
	serviceID := int64(12)
	addressID := int64(34)
	therapistID := int64(56)
	assignedAt := time.Now().UTC().Truncate(time.Second)
	booking := &model.Booking{
		BookingID:       bookingID,
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		TherapistID:     &therapistID,
		DurationMinutes: 60,
		Status:          model.BookingStatusAssigned,
		AssignedAt:      &assignedAt,
	}

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings target") &&
			strings.Contains(lower, "status =") &&
			strings.Contains(lower, "assigned_at =")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) >= 16 &&
			args[13] == booking.Status &&
			args[14] == booking.AssignedAt &&
			args[15] == booking.BookingID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	err := repo.UpdateAdmin(context.Background(), booking)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBookingRepoUpdateAdmin_AssignmentWriteRequiresEligibleTherapist(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB)

	bookingID := int64(120)
	serviceID := int64(12)
	therapistID := int64(56)
	scheduledStart := time.Now().UTC().Add(time.Hour)
	booking := &model.Booking{
		BookingID:       bookingID,
		ServiceID:       &serviceID,
		TherapistID:     &therapistID,
		DurationMinutes: 60,
		ScheduledStart:  &scheduledStart,
		Status:          model.BookingStatusAssigned,
	}

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings target") &&
			strings.Contains(lower, "from therapist_profiles") &&
			strings.Contains(lower, "join users") &&
			strings.Contains(lower, "tp.accept_assignments = true") &&
			strings.Contains(lower, "u.account_status = 'active'") &&
			strings.Contains(lower, "u.deleted_at is null") &&
			strings.Contains(lower, "from therapist_services") &&
			strings.Contains(lower, "not exists")
	}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	err := repo.UpdateAdmin(context.Background(), booking)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBookingRepoUpdateStatus_PersistsNoShowTimestampReasonAndEventMetadata(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB)

	bookingID := int64(222)
	actorID := int64(17)
	reason := "guest unavailable"

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings") &&
			strings.Contains(lower, "no_show_at = case when") &&
			strings.Contains(lower, "cancellation_reason = case when $1::text in ($11, $12)")
	}), mock.MatchedBy(func(args []interface{}) bool {
		if len(args) != 15 {
			return false
		}
		cancelledBy, _ := args[4].(*string)
		cancellationReason, _ := args[5].(*string)
		return args[0] == model.BookingStatusNoShow &&
			func() bool { _, ok := args[1].(time.Time); return ok }() &&
			args[2] == bookingID &&
			args[3] == actorID &&
			cancelledBy == nil &&
			cancellationReason != nil && *cancellationReason == reason &&
			args[6] == model.RoleAdmin &&
			args[7] == model.BookingStatusArrived &&
			args[8] == model.BookingStatusInProgress &&
			args[9] == model.BookingStatusCompleted &&
			args[10] == model.BookingStatusNoShow &&
			args[11] == model.BookingStatusCancelled &&
			args[12] == model.RoleAdmin &&
			args[13] == model.BookingStatusAssigned &&
			args[14] == model.BookingStatusOnTheWay
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into booking_events")
	}), mock.MatchedBy(func(args []interface{}) bool {
		if len(args) != 4 {
			return false
		}
		metadata, ok := args[3].(map[string]any)
		actor, okActor := args[2].(*int64)
		return ok && okActor && actor != nil && *actor == actorID && args[0] == bookingID && args[1] == model.BookingStatusNoShow && metadata["reason"] == reason && metadata["status"] == model.BookingStatusNoShow
	})).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

	err := repo.UpdateStatus(context.Background(), bookingID, actorID, model.RoleAdmin, model.BookingStatusNoShow, nil, &reason)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBookingRepoUpdateStatusWithTime_PersistsNoShowTimestampReasonAndEventMetadata(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB)

	bookingID := int64(223)
	actorID := int64(18)
	reason := "guest unavailable"
	when := time.Date(2026, time.May, 10, 16, 4, 5, 0, time.UTC)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings") &&
			strings.Contains(lower, "no_show_at = case when")
	}), mock.MatchedBy(func(args []interface{}) bool {
		if len(args) != 13 {
			return false
		}
		return args[0] == model.BookingStatusNoShow &&
			args[1] == when &&
			args[2] == bookingID &&
			args[3] == actorID &&
			args[6] == model.RoleAdmin &&
			args[7] == model.BookingStatusArrived &&
			args[8] == model.BookingStatusInProgress &&
			args[9] == model.BookingStatusCompleted &&
			args[10] == model.BookingStatusNoShow &&
			args[11] == model.BookingStatusCancelled &&
			args[12] == model.RoleAdmin
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into booking_events")
	}), mock.MatchedBy(func(args []interface{}) bool {
		if len(args) != 4 {
			return false
		}
		metadata, ok := args[3].(map[string]any)
		actor, okActor := args[2].(*int64)
		return ok && okActor && actor != nil && *actor == actorID && args[0] == bookingID && args[1] == model.BookingStatusNoShow && metadata["reason"] == reason && metadata["status"] == model.BookingStatusNoShow
	})).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

	err := repo.UpdateStatusWithTime(context.Background(), bookingID, actorID, model.RoleAdmin, model.BookingStatusNoShow, nil, &reason, &when)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBookingRepoRevertOnTheWayToAssigned_UpdatesBookingRideAndEventInOneTransaction(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	bookingRow := new(MockRow)
	rideRow := new(MockRow)
	repo := NewBookingRepository(mockDB)
	bookingID := int64(300)
	actorID := int64(1)
	rideID := int64(400)
	riderID := int64(500)
	passengerID := int64(600)

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings") &&
			strings.Contains(lower, "status = $1") &&
			strings.Contains(lower, "status = $3") &&
			strings.Contains(lower, "returning booking_id")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 3 &&
			args[0] == model.BookingStatusAssigned &&
			args[1] == bookingID &&
			args[2] == model.BookingStatusOnTheWay
	})).Return(bookingRow).Once()
	bookingRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = bookingID
	}).Return(nil).Once()

	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update rides") &&
			strings.Contains(lower, "rider_id = null") &&
			strings.Contains(lower, "status = $2") &&
			strings.Contains(lower, "accepted_at = null") &&
			strings.Contains(lower, "offered_at = null") &&
			strings.Contains(lower, "status in ($4, $5, $6)") &&
			strings.Contains(lower, "updated_at") &&
			strings.Contains(lower, "returning target.ride_id") &&
			strings.Contains(lower, "old_rider_id") &&
			strings.Contains(lower, "passenger_id")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 6 &&
			args[0] == bookingID &&
			args[1] == "pending" &&
			args[2] == "outbound" &&
			args[3] == "offered" &&
			args[4] == "accepted" &&
			args[5] == "arrived_pickup"
	})).Return(rideRow).Once()
	rideRow.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = rideID
		*args.Get(1).(*int64) = riderID
		*args.Get(2).(*int64) = passengerID
	}).Return(nil).Once()

	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into booking_events")
	}), mock.MatchedBy(func(args []interface{}) bool {
		actor, ok := args[2].(*int64)
		return len(args) == 4 && ok && actor != nil && *actor == actorID && args[0] == bookingID && args[1] == model.BookingStatusAssigned
	})).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()
	tx.On("Commit", mock.Anything).Return(nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Maybe()

	result, err := repo.RevertOnTheWayToAssigned(context.Background(), bookingID, actorID)

	assert.NoError(t, err)
	if assert.NotNil(t, result) {
		assert.True(t, result.ClearedOutbound)
		assert.Equal(t, rideID, result.ClearedRideID)
		assert.Equal(t, riderID, result.ClearedRiderID)
		assert.Equal(t, passengerID, result.PassengerID)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
	bookingRow.AssertExpectations(t)
	rideRow.AssertExpectations(t)
}

func TestBookingRepoRevertOnTheWayToAssigned_ClearsOnlyActiveOutboundRideWhenStaleRowsExist(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	bookingRow := new(MockRow)
	rideRow := new(MockRow)
	repo := NewBookingRepository(mockDB)
	bookingID := int64(300)
	actorID := int64(1)
	rideID := int64(400)
	riderID := int64(500)
	passengerID := int64(600)

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings") &&
			strings.Contains(lower, "status = $1") &&
			strings.Contains(lower, "status = $3") &&
			strings.Contains(lower, "returning booking_id")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 3 &&
			args[0] == model.BookingStatusAssigned &&
			args[1] == bookingID &&
			args[2] == model.BookingStatusOnTheWay
	})).Return(bookingRow).Once()
	bookingRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = bookingID
	}).Return(nil).Once()

	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update rides") &&
			strings.Contains(lower, "rider_id = null") &&
			strings.Contains(lower, "status = $2") &&
			strings.Contains(lower, "accepted_at = null") &&
			strings.Contains(lower, "offered_at = null") &&
			strings.Contains(lower, "status in ($4, $5, $6)") &&
			strings.Contains(lower, "order by ride_id") &&
			strings.Contains(lower, "limit 1") &&
			strings.Contains(lower, "for update")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 6 &&
			args[0] == bookingID &&
			args[1] == "pending" &&
			args[2] == "outbound" &&
			args[3] == "offered" &&
			args[4] == "accepted" &&
			args[5] == "arrived_pickup"
	})).Return(rideRow).Once()
	rideRow.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = rideID
		*args.Get(1).(*int64) = riderID
		*args.Get(2).(*int64) = passengerID
	}).Return(nil).Once()

	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into booking_events")
	}), mock.MatchedBy(func(args []interface{}) bool {
		actor, ok := args[2].(*int64)
		return len(args) == 4 && ok && actor != nil && *actor == actorID && args[0] == bookingID && args[1] == model.BookingStatusAssigned
	})).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()
	tx.On("Commit", mock.Anything).Return(nil).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Maybe()

	result, err := repo.RevertOnTheWayToAssigned(context.Background(), bookingID, actorID)

	assert.NoError(t, err)
	if assert.NotNil(t, result) {
		assert.True(t, result.ClearedOutbound)
		assert.Equal(t, rideID, result.ClearedRideID)
		assert.Equal(t, riderID, result.ClearedRiderID)
		assert.Equal(t, passengerID, result.PassengerID)
	}
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
	bookingRow.AssertExpectations(t)
	rideRow.AssertExpectations(t)
}

func TestBookingRepoRevertOnTheWayToAssigned_RollsBackWhenRideCleanupFails(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	bookingRow := new(MockRow)
	rideRow := new(MockRow)
	repo := NewBookingRepository(mockDB)
	bookingID := int64(301)
	actorID := int64(1)
	expectedErr := errors.New("ride cleanup failed")

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "update bookings")
	}), mock.Anything).Return(bookingRow).Once()
	bookingRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = bookingID
	}).Return(nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "update rides")
	}), mock.Anything).Return(rideRow).Once()
	rideRow.On("Scan", mock.Anything, mock.Anything, mock.Anything).Return(expectedErr).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	result, err := repo.RevertOnTheWayToAssigned(context.Background(), bookingID, actorID)

	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, result)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
	tx.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestBookingRepoRevertOnTheWayToAssigned_RollsBackWhenEventInsertFails(t *testing.T) {
	mockDB := new(MockDBTX)
	tx := new(MockTx)
	bookingRow := new(MockRow)
	rideRow := new(MockRow)
	repo := NewBookingRepository(mockDB)
	bookingID := int64(302)
	actorID := int64(1)
	expectedErr := errors.New("event insert failed")

	mockDB.On("Begin", mock.Anything).Return(tx, nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "update bookings")
	}), mock.Anything).Return(bookingRow).Once()
	bookingRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = bookingID
	}).Return(nil).Once()
	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "update rides")
	}), mock.Anything).Return(rideRow).Once()
	rideRow.On("Scan", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*int64) = int64(401)
		*args.Get(1).(*int64) = int64(501)
		*args.Get(2).(*int64) = int64(601)
	}).Return(nil).Once()
	tx.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into booking_events")
	}), mock.Anything).Return(pgconn.NewCommandTag(""), expectedErr).Once()
	tx.On("Rollback", mock.Anything).Return(nil).Once()

	result, err := repo.RevertOnTheWayToAssigned(context.Background(), bookingID, actorID)

	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, result)
	tx.AssertNotCalled(t, "Commit", mock.Anything)
	mockDB.AssertExpectations(t)
	tx.AssertExpectations(t)
}

func TestBookingRepoHasAssignedOutboundRiderCoverage(t *testing.T) {
	mockDB := new(MockDBTX)
	row := new(MockRow)
	repo := NewBookingRepository(mockDB)
	bookingID := int64(224)

	mockDB.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "select exists") &&
			strings.Contains(lower, "from rides") &&
			strings.Contains(lower, "booking_id = $1") &&
			strings.Contains(lower, "ride_type = $2") &&
			strings.Contains(lower, "rider_id is not null") &&
			strings.Contains(lower, "status in ($3, $4, $5)")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 5 &&
			args[0] == bookingID &&
			args[1] == "outbound" &&
			args[2] == "offered" &&
			args[3] == "accepted" &&
			args[4] == "arrived_pickup"
	})).Return(row).Once()
	row.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*bool) = true
	}).Return(nil).Once()

	hasCoverage, err := repo.HasAssignedOutboundRiderCoverage(context.Background(), bookingID)

	assert.NoError(t, err)
	assert.True(t, hasCoverage)
	mockDB.AssertExpectations(t)
	row.AssertExpectations(t)
}
