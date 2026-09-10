package repository

import (
	"context"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"time"
)

type HotelDayViewRepository interface {
	ListBranches(ctx context.Context) ([]model.HotelDayViewBranch, error)
	ListSchedule(ctx context.Context, start, end time.Time, hotelID int64) ([]model.HotelDayViewTherapist, error)
}

type hotelDayViewRepo struct{ db db.DBTX }

func NewHotelDayViewRepository(database db.DBTX) HotelDayViewRepository {
	return &hotelDayViewRepo{db: database}
}

func (r *hotelDayViewRepo) ListBranches(ctx context.Context) ([]model.HotelDayViewBranch, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `SELECT branch_id,branch_name FROM branches WHERE is_active AND deleted_at IS NULL ORDER BY branch_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	branches := make([]model.HotelDayViewBranch, 0)
	for rows.Next() {
		var branch model.HotelDayViewBranch
		if err := rows.Scan(&branch.BranchID, &branch.Name); err != nil {
			return nil, err
		}
		branches = append(branches, branch)
	}
	return branches, rows.Err()
}

func (r *hotelDayViewRepo) ListSchedule(ctx context.Context, start, end time.Time, hotelID int64) ([]model.HotelDayViewTherapist, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	// Client details are selected only for bookings belonging to the caller's hotel.
	// Booking timestamps are stored as UTC wall times, as in the admin day view.
	rows, err := r.db.Query(ctx, `
		SELECT tp.therapist_id,
			COALESCE(NULLIF(TRIM(u.nickname),''),NULLIF(split_part(TRIM(u.full_name),' ',1),''),'Therapist'),
			tp.branch_id, COALESCE(tp.accept_assignments,FALSE) AND u.account_status='active',
			b.scheduled_start, b.scheduled_start + b.duration_minutes * INTERVAL '1 minute',
 CASE WHEN hs.partner_hotel_id=$3 THEN b.booking_id ELSE 0 END,
 CASE WHEN hs.partner_hotel_id=$3 THEN h.hotel_name ELSE '' END,
 CASE WHEN hs.partner_hotel_id=$3 THEN COALESCE(b.guest_name,'') ELSE '' END,
 CASE WHEN hs.partner_hotel_id=$3 THEN COALESCE(b.notes,'') ELSE '' END
		FROM therapist_profiles tp
		JOIN users u ON u.user_id=tp.therapist_id
		LEFT JOIN branches br ON br.branch_id=tp.branch_id
		LEFT JOIN bookings b ON b.therapist_id=tp.therapist_id
			AND b.status NOT IN ('cancelled','cancelled_by_client','cancelled_by_therapist','rescheduled')
			AND b.scheduled_start < $2::timestamp
			AND b.scheduled_start + b.duration_minutes * INTERVAL '1 minute' > $1::timestamp
 LEFT JOIN partner_hotel_staff hs ON hs.user_id=b.client_id
 LEFT JOIN partner_hotels h ON h.partner_hotel_id=hs.partner_hotel_id
		WHERE tp.deleted_at IS NULL AND u.deleted_at IS NULL
			AND (tp.branch_id IS NULL OR (br.is_active AND br.deleted_at IS NULL))
			AND ((tp.accept_assignments AND u.account_status='active') OR b.scheduled_start IS NOT NULL)
		ORDER BY tp.therapist_id,b.scheduled_start`, start.UTC(), end.UTC(), hotelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	therapists := make([]model.HotelDayViewTherapist, 0)
	for rows.Next() {
		var therapist model.HotelDayViewTherapist
		var slotStart, slotEnd *time.Time
		var slot model.HotelBookedSlot
		if err := rows.Scan(&therapist.TherapistID, &therapist.Name, &therapist.BranchID, &therapist.AcceptingBookings, &slotStart, &slotEnd, &slot.BookingID, &slot.HotelName, &slot.GuestName, &slot.Notes); err != nil {
			return nil, err
		}
		if len(therapists) == 0 || therapists[len(therapists)-1].TherapistID != therapist.TherapistID {
			therapist.BookedSlots = make([]model.HotelBookedSlot, 0)
			therapists = append(therapists, therapist)
		}
		if slotStart != nil && slotEnd != nil {
			last := &therapists[len(therapists)-1]
			slot.Start = *slotStart
			slot.End = *slotEnd
			last.BookedSlots = append(last.BookedSlots, slot)
		}
	}
	return therapists, rows.Err()
}
