package repository

import (
	"context"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"time"
)

type HotelBookingRepository interface {
	List(context.Context, int64, int, int, string) ([]model.HotelBooking, error)
	ListOptions(context.Context) ([]model.HotelBookingOption, error)
	ListAnalyticsOptions(context.Context) ([]model.HotelAnalyticsOption, error)
	Analytics(context.Context, int64, time.Time, time.Time) (*model.HotelAnalytics, error)
	Owner(context.Context, int64, int64) (int64, error)
	HotelNames(context.Context, []int64) (map[int64]string, error)
}

func (r *hotelBookingRepo) ListAnalyticsOptions(ctx context.Context) ([]model.HotelAnalyticsOption, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `
		SELECT partner_hotel_id, hotel_name, city, is_active
		FROM partner_hotels
		ORDER BY is_active DESC, lower(hotel_name), partner_hotel_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]model.HotelAnalyticsOption, 0)
	for rows.Next() {
		var option model.HotelAnalyticsOption
		if err := rows.Scan(&option.PartnerHotelID, &option.HotelName, &option.City, &option.IsActive); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

func (r *hotelBookingRepo) Analytics(ctx context.Context, hotelID int64, from, until time.Time) (*model.HotelAnalytics, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	start := from.UTC().Format("2006-01-02 15:04:05")
	end := until.UTC().Format("2006-01-02 15:04:05")
	result := &model.HotelAnalytics{
		Currency:       "PHP",
		Daily:          make([]model.HotelAnalyticsDaily, 0),
		Statuses:       make([]model.HotelAnalyticsStatus, 0),
		TopServices:    make([]model.HotelAnalyticsService, 0),
		RecentBookings: make([]model.HotelAnalyticsRecentBooking, 0),
	}

	err := r.db.QueryRow(ctx, `
		SELECT h.hotel_name,
			COUNT(b.booking_id),
			COUNT(b.booking_id) FILTER (WHERE b.status='completed'),
			COUNT(b.booking_id) FILTER (WHERE b.status IN ('pending','assigned','on_the_way','arrived','in_progress') AND b.scheduled_start >= (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')),
			COUNT(b.booking_id) FILTER (WHERE b.status LIKE 'cancelled%' OR b.status='rescheduled'),
			COALESCE(SUM(b.final_total) FILTER (WHERE b.status='completed'),0),
			COALESCE(SUM(b.final_total) FILTER (WHERE b.status NOT LIKE 'cancelled%' AND b.status NOT IN ('no_show','rescheduled')),0)
		FROM partner_hotels h
		LEFT JOIN bookings b ON b.client_id IN (
			SELECT hs.user_id FROM partner_hotel_staff hs WHERE hs.partner_hotel_id=h.partner_hotel_id
		) AND b.scheduled_start >= $2::timestamp AND b.scheduled_start < $3::timestamp
		WHERE h.partner_hotel_id=$1
		GROUP BY h.partner_hotel_id,h.hotel_name`, hotelID, start, end).Scan(
		&result.HotelName,
		&result.Summary.TotalBookings,
		&result.Summary.CompletedBookings,
		&result.Summary.UpcomingBookings,
		&result.Summary.CancelledBookings,
		&result.Summary.Revenue,
		&result.Summary.BookedValue,
	)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT TO_CHAR(business_day(b.scheduled_start),'YYYY-MM-DD'),
			COUNT(*), COALESCE(SUM(b.final_total) FILTER (WHERE b.status='completed'),0)
		FROM bookings b
		WHERE b.client_id IN (SELECT user_id FROM partner_hotel_staff WHERE partner_hotel_id=$1)
			AND b.scheduled_start >= $2::timestamp AND b.scheduled_start < $3::timestamp
		GROUP BY 1 ORDER BY 1`, hotelID, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var point model.HotelAnalyticsDaily
		if err := rows.Scan(&point.Date, &point.Bookings, &point.Revenue); err != nil {
			rows.Close()
			return nil, err
		}
		result.Daily = append(result.Daily, point)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = r.db.Query(ctx, `
		SELECT b.status,COUNT(*)
		FROM bookings b
		WHERE b.client_id IN (SELECT user_id FROM partner_hotel_staff WHERE partner_hotel_id=$1)
			AND b.scheduled_start >= $2::timestamp AND b.scheduled_start < $3::timestamp
		GROUP BY b.status ORDER BY COUNT(*) DESC,b.status`, hotelID, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var status model.HotelAnalyticsStatus
		if err := rows.Scan(&status.Status, &status.Count); err != nil {
			rows.Close()
			return nil, err
		}
		result.Statuses = append(result.Statuses, status)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = r.db.Query(ctx, `
		SELECT COALESCE(NULLIF(TRIM(s.name),''),'Unspecified service'),COUNT(*),
			COALESCE(SUM(b.final_total) FILTER (WHERE b.status='completed'),0)
		FROM bookings b
		LEFT JOIN services s ON s.service_id=b.service_id
		WHERE b.client_id IN (SELECT user_id FROM partner_hotel_staff WHERE partner_hotel_id=$1)
			AND b.scheduled_start >= $2::timestamp AND b.scheduled_start < $3::timestamp
		GROUP BY 1 ORDER BY COUNT(*) DESC,1 LIMIT 5`, hotelID, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var service model.HotelAnalyticsService
		if err := rows.Scan(&service.ServiceName, &service.Bookings, &service.Revenue); err != nil {
			rows.Close()
			return nil, err
		}
		result.TopServices = append(result.TopServices, service)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = r.db.Query(ctx, `
		SELECT b.booking_id,COALESCE(b.guest_name,''),COALESCE(s.name,''),b.status,b.scheduled_start,COALESCE(b.final_total,0)
		FROM bookings b LEFT JOIN services s ON s.service_id=b.service_id
		WHERE b.client_id IN (SELECT user_id FROM partner_hotel_staff WHERE partner_hotel_id=$1)
			AND b.scheduled_start >= $2::timestamp AND b.scheduled_start < $3::timestamp
		ORDER BY b.created_at DESC,b.booking_id DESC LIMIT 6`, hotelID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var booking model.HotelAnalyticsRecentBooking
		if err := rows.Scan(&booking.BookingID, &booking.GuestName, &booking.ServiceName, &booking.Status, &booking.ScheduledStart, &booking.FinalTotal); err != nil {
			return nil, err
		}
		result.RecentBookings = append(result.RecentBookings, booking)
	}
	return result, rows.Err()
}

func (r *hotelBookingRepo) ListOptions(ctx context.Context) ([]model.HotelBookingOption, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `
		SELECT h.partner_hotel_id, h.hotel_name, h.address_line, h.city,
			account.user_id, address.address_id
		FROM partner_hotels h
		LEFT JOIN LATERAL (
			SELECT hs.user_id
			FROM partner_hotel_staff hs
			JOIN users u ON u.user_id=hs.user_id
			WHERE hs.partner_hotel_id=h.partner_hotel_id AND hs.is_active
				AND u.account_status='active' AND u.deleted_at IS NULL
			ORDER BY CASE hs.access_role WHEN 'hotel_admin' THEN 0 ELSE 1 END,
				hs.partner_hotel_staff_id
			LIMIT 1
		) account ON TRUE
		LEFT JOIN LATERAL (
			SELECT a.address_id
			FROM addresses a
			WHERE a.user_id=account.user_id AND a.deleted_at IS NULL
				AND a.disabled_at IS NULL
			ORDER BY a.is_default DESC, a.address_id DESC
			LIMIT 1
		) address ON TRUE
		WHERE h.is_active
		ORDER BY lower(h.hotel_name), h.partner_hotel_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]model.HotelBookingOption, 0)
	for rows.Next() {
		var option model.HotelBookingOption
		if err := rows.Scan(&option.PartnerHotelID, &option.HotelName, &option.AddressLine, &option.City, &option.BookingClientID, &option.DefaultAddressID); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

type hotelBookingRepo struct{ db db.DBTX }

func NewHotelBookingRepository(database db.DBTX) HotelBookingRepository {
	return &hotelBookingRepo{db: database}
}
func (r *hotelBookingRepo) List(ctx context.Context, hotelID int64, limit, offset int, status string) ([]model.HotelBooking, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `SELECT b.booking_id, b.client_id, h.hotel_name, COALESCE(b.guest_name,''), COALESCE(b.notes,''), b.status,
 b.scheduled_start, b.duration_minutes, COALESCE(s.name,''), COALESCE(t.full_name,'')
 FROM bookings b JOIN partner_hotel_staff hs ON hs.user_id=b.client_id
 JOIN partner_hotels h ON h.partner_hotel_id=hs.partner_hotel_id
 LEFT JOIN services s ON s.service_id=b.service_id LEFT JOIN users t ON t.user_id=b.therapist_id
 WHERE h.partner_hotel_id=$1 AND ($4::text = '' OR b.status = $4)
 ORDER BY CASE WHEN b.status = 'pending' THEN 0 ELSE 1 END,
 b.scheduled_start DESC NULLS LAST,b.booking_id DESC LIMIT $2 OFFSET $3`, hotelID, limit, offset, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.HotelBooking, 0)
	for rows.Next() {
		var b model.HotelBooking
		if err := rows.Scan(&b.BookingID, &b.ClientID, &b.HotelName, &b.GuestName, &b.Notes, &b.Status, &b.ScheduledStart, &b.DurationMinutes, &b.ServiceName, &b.TherapistName); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
func (r *hotelBookingRepo) Owner(ctx context.Context, hotelID, bookingID int64) (int64, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	var owner int64
	err := r.db.QueryRow(ctx, `SELECT b.client_id FROM bookings b JOIN partner_hotel_staff hs ON hs.user_id=b.client_id WHERE hs.partner_hotel_id=$1 AND b.booking_id=$2`, hotelID, bookingID).Scan(&owner)
	return owner, err
}
func (r *hotelBookingRepo) HotelNames(ctx context.Context, clients []int64) (map[int64]string, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `SELECT hs.user_id,h.hotel_name FROM partner_hotel_staff hs JOIN partner_hotels h ON h.partner_hotel_id=hs.partner_hotel_id WHERE hs.user_id=ANY($1::bigint[])`, clients)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := make(map[int64]string)
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}
