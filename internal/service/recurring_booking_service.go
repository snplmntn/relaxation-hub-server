package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

const (
	recurringHorizonDays = 28
	manilaOffset         = 8 * 60 * 60 // seconds east of UTC
)

var manilaLocation = time.FixedZone("Asia/Manila", manilaOffset)

// RecurringBookingService manages recurring booking series and occurrence generation.
type RecurringBookingService struct {
	db            db.DBTX
	recurringRepo repository.RecurringBookingRepository
	bookingRepo   repository.BookingRepository
	serviceRepo   repository.ServiceRepository
	queueRepo     repository.AssignmentQueueRepository
	userRepo      repository.UserRepository
}

func NewRecurringBookingService(
	pool db.DBTX,
	recurringRepo repository.RecurringBookingRepository,
	bookingRepo repository.BookingRepository,
	serviceRepo repository.ServiceRepository,
	queueRepo repository.AssignmentQueueRepository,
	userRepo repository.UserRepository,
) *RecurringBookingService {
	return &RecurringBookingService{
		db:            pool,
		recurringRepo: recurringRepo,
		bookingRepo:   bookingRepo,
		serviceRepo:   serviceRepo,
		queueRepo:     queueRepo,
		userRepo:      userRepo,
	}
}

// CreateSeries creates a recurring booking template and materialises the initial horizon.
func (s *RecurringBookingService) CreateSeries(ctx context.Context, actorID int64, req *model.CreateRecurringBookingRequest) (*model.RecurringBooking, error) {
	if err := validateCreateRecurringRequest(req); err != nil {
		return nil, err
	}

	// Reject pinning a therapist that is blocked for this client.
	if req.TherapistID != nil {
		if berr := checkAssignmentBlock(ctx, s.userRepo, req.ClientID, *req.TherapistID); berr != nil {
			return nil, berr
		}
	}

	startDate, err := time.ParseInLocation("2006-01-02", req.StartDate, manilaLocation)
	if err != nil {
		return nil, NewValidationError("invalid_start_date", "start_date must be YYYY-MM-DD", nil)
	}

	var endDate *time.Time
	if strings.TrimSpace(req.EndDate) != "" {
		ed, err := time.ParseInLocation("2006-01-02", req.EndDate, manilaLocation)
		if err != nil {
			return nil, NewValidationError("invalid_end_date", "end_date must be YYYY-MM-DD", nil)
		}
		endDate = &ed
	}

	interval := req.IntervalValue
	if interval <= 0 {
		interval = 1
	}

	rec := &model.RecurringBooking{
		ClientID:             req.ClientID,
		CreatedBy:            &actorID,
		ServiceID:            req.ServiceID,
		AddressID:            req.AddressID,
		TherapistID:          req.TherapistID,
		IsTherapistRequested: req.IsTherapistRequested,
		DurationMinutes:      req.DurationMinutes,
		GenderPref:           strings.TrimSpace(req.GenderPref),
		PressurePref:         strings.TrimSpace(req.PressurePref),
		Notes:                strings.TrimSpace(req.Notes),
		PaymentMethod:        strings.TrimSpace(req.PaymentMethod),
		Frequency:            req.Frequency,
		IntervalValue:        interval,
		DaysOfWeek:           req.DaysOfWeek,
		DayOfMonth:           req.DayOfMonth,
		TimeOfDay:            req.TimeOfDay,
		StartDate:            startDate,
		EndDate:              endDate,
		Status:               "active",
	}

	if err := s.recurringRepo.Create(ctx, rec); err != nil {
		return nil, fmt.Errorf("failed to create recurring booking: %w", err)
	}

	// Materialise first horizon inside a transaction
	horizon := time.Now().In(manilaLocation).Add(recurringHorizonDays * 24 * time.Hour)
	if err := s.materializeHorizon(ctx, rec, horizon); err != nil {
		return nil, fmt.Errorf("failed to generate initial occurrences: %w", err)
	}

	return rec, nil
}

// GetByID returns a series with upcoming pending bookings attached.
func (s *RecurringBookingService) GetByID(ctx context.Context, id int64) (*model.RecurringBooking, error) {
	rec, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Hydrate upcoming occurrences (rough inline query — limited to next 20)
	upcoming, err := s.listUpcomingOccurrences(ctx, id, 20)
	if err == nil {
		rec.UpcomingBookings = upcoming
	}
	return rec, nil
}

// ListSeries returns paginated series optionally filtered by status.
func (s *RecurringBookingService) ListSeries(ctx context.Context, status string, clientID *int64, limit, offset int) ([]model.RecurringBooking, int, error) {
	return s.recurringRepo.List(ctx, status, clientID, limit, offset)
}

// UpdateSeries applies status / editable-field changes to a series.
func (s *RecurringBookingService) UpdateSeries(ctx context.Context, id int64, req *model.UpdateRecurringBookingRequest) (*model.RecurringBooking, error) {
	rec, err := s.recurringRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("recurring booking not found: %w", err)
	}

	if req.Status != nil {
		allowed := map[string]bool{"active": true, "paused": true, "cancelled": true}
		if !allowed[*req.Status] {
			return nil, NewValidationError("invalid_status", "status must be active, paused, or cancelled", nil)
		}
		rec.Status = *req.Status
	}
	if req.EndDate != nil {
		if strings.TrimSpace(*req.EndDate) == "" {
			rec.EndDate = nil
		} else {
			ed, err := time.ParseInLocation("2006-01-02", *req.EndDate, manilaLocation)
			if err != nil {
				return nil, NewValidationError("invalid_end_date", "end_date must be YYYY-MM-DD", nil)
			}
			rec.EndDate = &ed
		}
	}
	if req.Notes != nil {
		rec.Notes = strings.TrimSpace(*req.Notes)
	}
	if req.PaymentMethod != nil {
		rec.PaymentMethod = strings.TrimSpace(*req.PaymentMethod)
	}
	if req.TherapistID != nil {
		rec.TherapistID = req.TherapistID
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.recurringRepo.Update(ctx, rec); err != nil {
		return nil, fmt.Errorf("failed to update recurring booking: %w", err)
	}

	// When cancelling, also cancel future pending occurrences
	if req.Status != nil && *req.Status == "cancelled" {
		if err := s.recurringRepo.CancelFuturePendingTx(ctx, tx, id, time.Now().UTC()); err != nil {
			return nil, fmt.Errorf("failed to cancel future occurrences: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return rec, nil
}

// AdvanceHorizon generates new occurrences up to now+horizon for the given series.
// Called by the background worker.
func (s *RecurringBookingService) AdvanceHorizon(ctx context.Context, rec *model.RecurringBooking, now time.Time) error {
	horizon := now.In(manilaLocation).Add(recurringHorizonDays * 24 * time.Hour)
	return s.materializeHorizon(ctx, rec, horizon)
}

// currentDefaultAddressID returns the client's active default address, or nil
// when they have none or the lookup fails, so callers keep the address the
// series was created with.
func (s *RecurringBookingService) currentDefaultAddressID(ctx context.Context, clientID int64) *int64 {
	var addressID int64
	err := s.db.QueryRow(ctx, `
		SELECT address_id FROM addresses
		WHERE user_id = $1 AND is_default = TRUE
		  AND deleted_at IS NULL AND disabled_at IS NULL
		LIMIT 1`, clientID).Scan(&addressID)
	if err != nil {
		slog.Debug("recurring: no active default address for client", "client_id", clientID, "error", err)
		return nil
	}
	return &addressID
}

// materializeHorizon creates bookings from the series' current generated_until (or start) up to until.
func (s *RecurringBookingService) materializeHorizon(ctx context.Context, rec *model.RecurringBooking, until time.Time) error {
	from := rec.StartDate
	if rec.GeneratedUntil != nil && rec.GeneratedUntil.After(from) {
		from = *rec.GeneratedUntil
	}

	occurrences := computeOccurrences(rec, from, until)
	if len(occurrences) == 0 {
		return nil
	}

	// The series snapshots the address it was created with. A client who moves
	// would otherwise keep getting occurrences at the old address forever, so
	// resolve their current default and fall back to the series' address.
	addressID := rec.AddressID
	if current := s.currentDefaultAddressID(ctx, rec.ClientID); current != nil {
		addressID = current
	}

	// Look up service pricing once
	var rawTotal float64
	if rec.ServiceID != nil {
		svc, err := s.serviceRepo.GetByID(ctx, *rec.ServiceID)
		if err == nil && svc != nil {
			rawTotal = calculateServiceCost(svc, rec.DurationMinutes)
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var createdIDs []int64
	for _, occ := range occurrences {
		// bookings.scheduled_start is TIMESTAMP (without time zone) and the table's
		// convention is to store UTC wall-clock values (normal bookings come from
		// JS toISOString(), i.e. UTC). Occurrences are computed in Asia/Manila, so we
		// must convert to UTC before storing — otherwise Postgres keeps the Manila
		// wall-clock and it reads back 8h off (3 PM Manila -> stored 15:00 -> 11 PM).
		t := occ.UTC()
		booking := &model.Booking{
			ClientID:             rec.ClientID,
			TherapistID:          rec.TherapistID,
			IsTherapistRequested: rec.IsTherapistRequested,
			IsLocked:             rec.IsTherapistRequested,
			ServiceID:            rec.ServiceID,
			AddressID:            addressID,
			GenderPref:           rec.GenderPref,
			PressurePref:         rec.PressurePref,
			Notes:                rec.Notes,
			DurationMinutes:      rec.DurationMinutes,
			ScheduledStart:       &t,
			RawTotal:             float64PtrVal(rawTotal),
			Discount:             float64PtrVal(0),
			FinalTotal:           float64PtrVal(rawTotal),
			PaymentMethod:        rec.PaymentMethod,
			Status:               "pending",
			RecurringID:          &rec.RecurringID,
			StartCondition:       "fixed_time",
		}
		if err := s.bookingRepo.CreateTx(ctx, tx, booking); err != nil {
			// Unique constraint violation means occurrence already exists — skip and continue.
			if isUniqueConstraintError(err) {
				continue
			}
			return fmt.Errorf("failed to create occurrence at %v: %w", occ, err)
		}
		createdIDs = append(createdIDs, booking.BookingID)
	}

	if len(createdIDs) > 0 {
		if err := s.queueRepo.EnqueueManyTx(ctx, tx, createdIDs); err != nil {
			return fmt.Errorf("failed to enqueue occurrences: %w", err)
		}
	}

	if err := s.recurringRepo.SetGeneratedUntilTx(ctx, tx, rec.RecurringID, until); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// computeOccurrences returns all occurrence datetimes in [from, until] for the series.
//
// IMPORTANT: rec.StartDate / rec.EndDate may arrive in different zones depending
// on the caller — the create path parses them in Asia/Manila, but pgx scans the
// PG date columns back as UTC midnight in the worker path. We therefore use only
// their calendar Y/M/D and rebuild every boundary explicitly in Asia/Manila, so
// occurrences land on the correct local day/time regardless of the source zone.
func computeOccurrences(rec *model.RecurringBooking, from, until time.Time) []time.Time {
	h, m := parseTimeOfDay(rec.TimeOfDay)

	var out []time.Time

	// Normalize start_date to local midnight in Manila (calendar date only).
	startCal := time.Date(rec.StartDate.Year(), rec.StartDate.Month(), rec.StartDate.Day(), 0, 0, 0, 0, manilaLocation)

	// Effective scan start: the later of start_date and `from` (an instant).
	scanStart := startCal
	if from.After(scanStart) {
		scanStart = from.In(manilaLocation)
	}

	// Effective end: the horizon, capped at end_date (inclusive, end of that local day).
	scanEnd := until.In(manilaLocation)
	if rec.EndDate != nil {
		endOfDay := time.Date(rec.EndDate.Year(), rec.EndDate.Month(), rec.EndDate.Day(), 23, 59, 59, 0, manilaLocation)
		if endOfDay.Before(scanEnd) {
			scanEnd = endOfDay
		}
	}

	switch rec.Frequency {
	case "daily":
		// Start from the first occurrence on or after scanStart
		cursor := firstOccurrenceOnOrAfter(startCal, scanStart, h, m, manilaLocation)
		for !cursor.After(scanEnd) {
			out = append(out, cursor)
			cursor = cursor.AddDate(0, 0, rec.IntervalValue)
		}

	case "weekly":
		if len(rec.DaysOfWeek) == 0 {
			break
		}
		// Walk week by week starting from the week containing start_date
		weekStart := weekStartMonday(startCal)
		for {
			weekEnd := weekStart.AddDate(0, 0, 7*rec.IntervalValue)
			for _, dow := range rec.DaysOfWeek {
				// Map 0=Sun…6=Sat onto time.Weekday
				occ := occurrenceForWeekday(weekStart, time.Weekday(dow), h, m, manilaLocation)
				if !occ.Before(scanStart) && !occ.After(scanEnd) {
					out = append(out, occ)
				}
			}
			weekStart = weekEnd
			if weekStart.After(scanEnd) {
				break
			}
		}

	case "monthly":
		dom := 1
		if rec.DayOfMonth != nil {
			dom = *rec.DayOfMonth
		}
		// Build from the series start month
		cursor := time.Date(startCal.Year(), startCal.Month(), 1, 0, 0, 0, 0, manilaLocation)
		for {
			occ := clampToMonth(cursor.Year(), cursor.Month(), dom, h, m, manilaLocation)
			if occ.After(scanEnd) {
				break
			}
			if !occ.Before(scanStart) {
				out = append(out, occ)
			}
			cursor = cursor.AddDate(0, rec.IntervalValue, 0)
		}
	}

	return out
}

// firstOccurrenceOnOrAfter finds the first daily occurrence >= target.
func firstOccurrenceOnOrAfter(base, target time.Time, h, m int, loc *time.Location) time.Time {
	occ := time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, loc)
	for occ.Before(target) {
		occ = occ.AddDate(0, 0, 1)
	}
	return occ
}

// weekStartMonday returns the Monday on or before the given date (ISO week start),
// at midnight in t's own location. We rebuild the date with time.Date rather than
// using Truncate(24h), because Truncate operates in UTC and would shift a non-UTC
// (e.g. Asia/Manila) midnight onto the wrong calendar day.
func weekStartMonday(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return midnight.AddDate(0, 0, -(wd - 1))
}

// occurrenceForWeekday returns the occurrence in the week starting at weekStart for the given weekday.
func occurrenceForWeekday(weekStart time.Time, wd time.Weekday, h, m int, loc *time.Location) time.Time {
	daysFromMonday := int(wd) - 1
	if wd == time.Sunday {
		daysFromMonday = 6
	}
	d := weekStart.AddDate(0, 0, daysFromMonday)
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, loc)
}

// clampToMonth returns the occurrence for month/year at day dom, clamped to the last day of the month.
func clampToMonth(year int, month time.Month, dom, h, m int, loc *time.Location) time.Time {
	// Find last day of month
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if dom > lastDay {
		dom = lastDay
	}
	return time.Date(year, month, dom, h, m, 0, 0, loc)
}

func parseTimeOfDay(s string) (h, m int) {
	if len(s) >= 5 {
		fmt.Sscanf(s[:5], "%d:%d", &h, &m)
	}
	return
}

func calculateServiceCost(svc *model.Service, durationMinutes int) float64 {
	if svc.DurationMinutes <= 0 {
		return svc.BasePrice
	}
	extra := 0.0
	if durationMinutes > svc.DurationMinutes {
		diff := durationMinutes - svc.DurationMinutes
		extra = (svc.BasePrice / float64(svc.DurationMinutes)) * float64(diff)
	}
	return math.Round((svc.BasePrice+extra)*100) / 100
}

func float64PtrVal(v float64) *float64 {
	return &v
}

// isUniqueConstraintError returns true for PostgreSQL unique constraint violations (code 23505).
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	type pgErr interface {
		SQLState() string
	}
	if pe, ok := err.(pgErr); ok {
		return pe.SQLState() == "23505"
	}
	return strings.Contains(err.Error(), "23505")
}

func (s *RecurringBookingService) listUpcomingOccurrences(ctx context.Context, recurringID int64, limit int) ([]model.Booking, error) {
	return s.bookingRepo.ListByRecurringID(ctx, recurringID, time.Now().UTC(), limit)
}

func validateCreateRecurringRequest(req *model.CreateRecurringBookingRequest) error {
	if req.IsTherapistRequested && req.TherapistID == nil {
		return NewValidationError("requested_therapist_required", "a requested therapist must be selected", map[string]string{"therapist_id": "is required when is_therapist_requested is true"})
	}
	if req.ClientID <= 0 {
		return NewValidationError("missing_client", "client_id is required", nil)
	}
	if req.ServiceID == nil {
		return NewValidationError("missing_service", "service_id is required", nil)
	}
	allowed := map[string]bool{"daily": true, "weekly": true, "monthly": true}
	if !allowed[req.Frequency] {
		return NewValidationError("invalid_frequency", "frequency must be daily, weekly, or monthly", nil)
	}
	if req.Frequency == "weekly" && len(req.DaysOfWeek) == 0 {
		return NewValidationError("missing_days", "days_of_week is required for weekly frequency", nil)
	}
	if req.Frequency == "monthly" && req.DayOfMonth == nil {
		return NewValidationError("missing_dom", "day_of_month is required for monthly frequency", nil)
	}
	if req.TimeOfDay == "" {
		return NewValidationError("missing_time", "time_of_day is required (HH:MM)", nil)
	}
	if req.StartDate == "" {
		return NewValidationError("missing_start", "start_date is required (YYYY-MM-DD)", nil)
	}
	if req.DurationMinutes <= 0 {
		return NewValidationError("invalid_duration", "duration_minutes must be positive", nil)
	}
	return nil
}
