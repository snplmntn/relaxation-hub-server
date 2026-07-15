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
		ClientID:             999,
		ServiceID:            &serviceID,
		PromoID:              &promoID,
		PaymentMethod:        "cash",
		DurationMinutes:      60,
		ScheduledStart:       &now,
		RawTotal:             &rawTotal,
		Discount:             &discount,
		FinalTotal:           &finalTotal,
		Status:               model.BookingStatusPending,
		ReferenceCode:        &referenceCode,
		GroupID:              &groupID,
		GuestName:            "Guest 1",
		SequenceNumber:       1,
		StartCondition:       "after_previous",
		IsTherapistRequested: true,
		IsLocked:             true,
	}

	tx.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "insert into bookings") &&
			strings.Contains(lower, "group_id") &&
			strings.Contains(lower, "guest_name") &&
			strings.Contains(lower, "sequence_number") &&
			strings.Contains(lower, "start_condition")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 25 &&
			args[17] == booking.GroupID &&
			args[18] == booking.GuestName &&
			args[19] == booking.SequenceNumber &&
			args[20] == booking.StartCondition &&
			args[23] == booking.IsTherapistRequested &&
			args[24] == booking.IsLocked
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
			strings.Contains(lower, "discount =") &&
			strings.Contains(lower, "status =") &&
			strings.Contains(lower, "assigned_at =")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 21 &&
			args[14] == booking.Status &&
			args[15] == booking.AssignedAt &&
			args[16] == booking.IsTherapistRequested &&
			args[17] == booking.IsLocked &&
			args[18] == booking.BookingID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	err := repo.UpdateAdmin(context.Background(), booking)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBookingRepoUpdate_PersistsDiscountAndFinalTotal(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB)

	bookingID := int64(122)
	clientID := int64(7)
	serviceID := int64(12)
	addressID := int64(34)
	rawTotal := 570.0
	discount := 57.0
	finalTotal := 513.0
	booking := &model.Booking{
		BookingID:       bookingID,
		ClientID:        clientID,
		ServiceID:       &serviceID,
		AddressID:       &addressID,
		DurationMinutes: 60,
		RawTotal:        &rawTotal,
		Discount:        &discount,
		FinalTotal:      &finalTotal,
	}

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings target") &&
			strings.Contains(lower, "discount = $12") &&
			strings.Contains(lower, "final_total = $13")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 17 &&
			args[10] == booking.RawTotal &&
			args[11] == booking.Discount &&
			args[12] == booking.FinalTotal &&
			args[13] == booking.IsTherapistRequested &&
			args[14] == booking.IsLocked &&
			args[15] == booking.BookingID &&
			args[16] == booking.ClientID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	err := repo.Update(context.Background(), booking)

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

func TestBookingRepoUpdateAdmin_AssignmentConflictCheckCastsScheduledStart(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB)

	bookingID := int64(121)
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
		normalized := strings.Join(strings.Fields(strings.ToLower(sql)), " ")
		return strings.Contains(normalized, "other.scheduled_start::timestamp < ($8::timestamp + ($7::int * interval '1 minute'))") &&
			strings.Contains(normalized, "$8::timestamp < (other.scheduled_start::timestamp + (other.duration_minutes * interval '1 minute'))")
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
		if len(args) != 16 {
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
			args[14] == model.BookingStatusOnTheWay &&
			args[15] == model.RoleSuperAdmin
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

func TestBookingRepoUpdateStatus_AllowsSuperAdminOverride(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB)

	bookingID := int64(224)
	actorID := int64(19)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update bookings") &&
			strings.Contains(lower, "$7::text in ($13, $16)")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 16 &&
			args[0] == model.BookingStatusOnTheWay &&
			args[2] == bookingID &&
			args[3] == actorID &&
			args[6] == model.RoleSuperAdmin &&
			args[12] == model.RoleAdmin &&
			args[15] == model.RoleSuperAdmin
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "insert into booking_events")
	}), mock.Anything).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()

	err := repo.UpdateStatus(context.Background(), bookingID, actorID, model.RoleSuperAdmin, model.BookingStatusOnTheWay, nil, nil)

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
		if len(args) != 14 {
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
			args[12] == model.RoleAdmin &&
			args[13] == model.RoleSuperAdmin
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

func TestBookingRepoListDueInProgressBookings_FiltersOnlyDueUnpausedStartedRowsInSQL(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewBookingRepository(mockDB)
	now := time.Date(2026, time.May, 11, 10, 30, 0, 0, time.UTC)
	limit := 50

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from bookings") &&
			strings.Contains(lower, "status = 'in_progress'") &&
			strings.Contains(lower, "actual_start is not null") &&
			strings.Contains(lower, "current_pause_start is null") &&
			strings.Contains(lower, "actual_start + (duration_minutes * interval '1 minute') + (total_paused_seconds * interval '1 second') <= $1") &&
			strings.Contains(lower, "order by actual_start asc, booking_id asc") &&
			strings.Contains(lower, "limit $2")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == now && args[1] == limit
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	bookings, err := repo.ListDueInProgressBookings(context.Background(), now, limit)

	assert.NoError(t, err)
	assert.Empty(t, bookings)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestBookingRepoEnqueueReminderJobs_IdempotentlyUpsertsTwoReminderRows(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB).(*bookingRepoImpl)
	bookingID := int64(701)
	now := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "insert into booking_reminder_jobs") &&
			strings.Contains(lower, "reminder_24h") &&
			strings.Contains(lower, "reminder_2h") &&
			strings.Contains(lower, "on conflict (booking_id, event_type) do update") &&
			strings.Contains(lower, "due_at = excluded.due_at") &&
			strings.Contains(lower, "processed_at = case") &&
			strings.Contains(lower, "booking_reminder_jobs.scheduled_start is distinct from excluded.scheduled_start") &&
			strings.Contains(lower, "then null") &&
			strings.Contains(lower, "else booking_reminder_jobs.processed_at") &&
			strings.Contains(lower, "where b.booking_id = $1") &&
			strings.Contains(lower, "b.status = 'assigned'") &&
			strings.Contains(lower, "b.scheduled_start is not null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == bookingID && args[1] == now
	})).Return(pgconn.NewCommandTag("INSERT 2"), nil).Once()

	err := repo.EnqueueReminderJobs(context.Background(), bookingID, now)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestBookingRepoClaimDueReminderJobs_UsesBoundedSkipLockedQuery(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewBookingRepository(mockDB).(*bookingRepoImpl)
	now := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)
	limit := 25

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from booking_reminder_jobs brj") &&
			strings.Contains(lower, "brj.booking_id as reminder_booking_id") &&
			strings.Contains(lower, "b.booking_id = dj.reminder_booking_id") &&
			strings.Contains(lower, "join bookings b") &&
			strings.Contains(lower, "brj.processed_at is null") &&
			strings.Contains(lower, "brj.due_at <= $1") &&
			strings.Contains(lower, "order by brj.due_at asc, brj.job_id asc") &&
			strings.Contains(lower, "for update skip locked") &&
			strings.Contains(lower, "limit $2")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 2 && args[0] == now && args[1] == limit
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	jobs, err := repo.ClaimDueReminderJobs(context.Background(), now, limit)

	assert.NoError(t, err)
	assert.Empty(t, jobs)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestBookingRepoClaimDueReminderJobs_SkipsProcessedJobsInSQL(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewBookingRepository(mockDB).(*bookingRepoImpl)
	now := time.Date(2026, time.May, 11, 10, 0, 0, 0, time.UTC)

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(strings.ToLower(sql), "brj.processed_at is null")
	}), mock.Anything).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	jobs, err := repo.ClaimDueReminderJobs(context.Background(), now, 10)

	assert.NoError(t, err)
	assert.Empty(t, jobs)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestBookingRepoMarkReminderJobProcessed_MarksOnlyUnprocessedJob(t *testing.T) {
	mockDB := new(MockDBTX)
	repo := NewBookingRepository(mockDB).(*bookingRepoImpl)
	jobID := int64(901)

	mockDB.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "update booking_reminder_jobs") &&
			strings.Contains(lower, "processed_at = now()") &&
			strings.Contains(lower, "where job_id = $1") &&
			strings.Contains(lower, "processed_at is null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 1 && args[0] == jobID
	})).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	err := repo.MarkReminderJobProcessed(context.Background(), jobID)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
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

func TestSelectBookingDetailsRidePrefersAssignedRideOverNewerPendingDuplicate(t *testing.T) {
	riderID := int64(77)
	olderAccepted := &model.Ride{
		RideID:    1,
		RideType:  "outbound",
		RiderID:   &riderID,
		Status:    "accepted",
		CreatedAt: time.Date(2026, time.May, 10, 9, 0, 0, 0, time.UTC),
	}
	newerPending := &model.Ride{
		RideID:    2,
		RideType:  "outbound",
		Status:    "pending",
		CreatedAt: time.Date(2026, time.May, 10, 10, 0, 0, 0, time.UTC),
	}

	selected := selectBookingDetailsRide(newerPending, olderAccepted)

	assert.NotNil(t, selected)
	assert.Equal(t, int64(1), selected.RideID)
}

func TestBookingRepoListRiderDispatchCandidates_UsesBoundedNoRideQuery(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewBookingRepository(mockDB).(*bookingRepoImpl)
	start := time.Date(2026, time.May, 11, 9, 0, 0, 0, time.UTC)
	end := start.Add(12 * time.Hour)
	limit := 50

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from bookings b") &&
			strings.Contains(lower, "join addresses a") &&
			strings.Contains(lower, "left join therapist_profiles tp") &&
			strings.Contains(lower, "left join rides r") &&
			strings.Contains(lower, "join services s") &&
			strings.Contains(lower, "b.scheduled_start between $1 and $2") &&
			strings.Contains(lower, "r.ride_id is null") &&
			strings.Contains(lower, "order by b.scheduled_start asc, b.booking_id asc") &&
			strings.Contains(lower, "limit $3")
	}), mock.MatchedBy(func(args []interface{}) bool {
		return len(args) == 3 && args[0] == start && args[1] == end && args[2] == limit
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	candidates, err := repo.ListRiderDispatchCandidates(context.Background(), start, end, limit)

	assert.NoError(t, err)
	assert.Empty(t, candidates)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestBookingRepoGetPreviousBookingDropoffs_UsesSingleBatchQuery(t *testing.T) {
	mockDB := new(MockDBTX)
	rows := new(MockRows)
	repo := NewBookingRepository(mockDB).(*bookingRepoImpl)
	scheduledStart := time.Date(2026, time.May, 11, 9, 0, 0, 0, time.UTC)
	lookups := []PreviousDropoffLookup{{BookingID: 100, TherapistID: 20, ScheduledStart: scheduledStart}}

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "with candidates as") &&
			strings.Contains(lower, "unnest($1::bigint[], $2::bigint[], $3::timestamptz[])") &&
			strings.Contains(lower, "distinct on (c.booking_id)") &&
			strings.Contains(lower, "join bookings b") &&
			strings.Contains(lower, "b.scheduled_start < c.scheduled_start") &&
			strings.Contains(lower, "order by c.booking_id, b.scheduled_start desc")
	}), mock.MatchedBy(func(args []interface{}) bool {
		bookingIDs, okBooking := args[0].([]int64)
		therapistIDs, okTherapist := args[1].([]int64)
		starts, okStarts := args[2].([]time.Time)
		return len(args) == 3 && okBooking && okTherapist && okStarts &&
			assert.ObjectsAreEqual([]int64{100}, bookingIDs) &&
			assert.ObjectsAreEqual([]int64{20}, therapistIDs) &&
			assert.ObjectsAreEqual([]time.Time{scheduledStart}, starts)
	})).Return(rows, nil).Once()
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return().Once()
	rows.On("Err").Return(nil).Once()

	locations, err := repo.GetPreviousBookingDropoffs(context.Background(), lookups)

	assert.NoError(t, err)
	assert.Empty(t, locations)
	mockDB.AssertExpectations(t)
	rows.AssertExpectations(t)
}

func TestBookingRepoGetBranchAndAddressLocations_UseAnyBatchQueries(t *testing.T) {
	mockDB := new(MockDBTX)
	branchRows := new(MockRows)
	addressRows := new(MockRows)
	repo := NewBookingRepository(mockDB).(*bookingRepoImpl)
	branchIDs := []int64{44, 45}
	addressIDs := []int64{55, 56}

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from branches") &&
			strings.Contains(lower, "branch_id = any($1)") &&
			strings.Contains(lower, "deleted_at is null") &&
			strings.Contains(lower, "latitude is not null") &&
			strings.Contains(lower, "longitude is not null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		ids, ok := args[0].([]int64)
		return len(args) == 1 && ok && assert.ObjectsAreEqual(branchIDs, ids)
	})).Return(branchRows, nil).Once()
	branchRows.On("Next").Return(false).Once()
	branchRows.On("Close").Return().Once()
	branchRows.On("Err").Return(nil).Once()

	mockDB.On("Query", mock.Anything, mock.MatchedBy(func(sql string) bool {
		lower := strings.ToLower(sql)
		return strings.Contains(lower, "from addresses") &&
			strings.Contains(lower, "address_id = any($1)") &&
			strings.Contains(lower, "deleted_at is null") &&
			strings.Contains(lower, "latitude is not null") &&
			strings.Contains(lower, "longitude is not null")
	}), mock.MatchedBy(func(args []interface{}) bool {
		ids, ok := args[0].([]int64)
		return len(args) == 1 && ok && assert.ObjectsAreEqual(addressIDs, ids)
	})).Return(addressRows, nil).Once()
	addressRows.On("Next").Return(false).Once()
	addressRows.On("Close").Return().Once()
	addressRows.On("Err").Return(nil).Once()

	branches, err := repo.GetBranchLocations(context.Background(), branchIDs)
	assert.NoError(t, err)
	assert.Empty(t, branches)
	addresses, err := repo.GetAddressLocations(context.Background(), addressIDs)
	assert.NoError(t, err)
	assert.Empty(t, addresses)
	mockDB.AssertExpectations(t)
	branchRows.AssertExpectations(t)
	addressRows.AssertExpectations(t)
}
