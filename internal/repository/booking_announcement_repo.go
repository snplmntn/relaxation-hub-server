package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

var (
	ErrVariationNotInCampaign   = errors.New("variation does not belong to campaign")
	ErrWinningVariationRequired = errors.New("winning variation cannot be removed")
)

type BookingAnnouncementRepository interface {
	List(context.Context) ([]model.BookingAnnouncement, error)
	Create(context.Context, *model.BookingAnnouncement) error
	Update(context.Context, *model.BookingAnnouncement) error
	Delete(context.Context, int64) error
	ChooseWinner(context.Context, int64, int64) (*model.BookingAnnouncement, error)
	ResolveForBooking(context.Context, int64) ([]model.ResolvedBookingAnnouncement, error)
}

type bookingAnnouncementQuerier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type bookingAnnouncementRepo struct{ db db.DBTX }

func NewBookingAnnouncementRepository(database db.DBTX) BookingAnnouncementRepository {
	return &bookingAnnouncementRepo{db: database}
}

const bookingAnnouncementSelect = `
	SELECT a.announcement_id, a.message, a.start_date, a.end_date,
		a.winner_variation_id, a.created_at, a.updated_at,
		v.variation_id, v.message, v.position, v.created_at, v.updated_at
	FROM booking_announcements a
	JOIN booking_announcement_variations v ON v.announcement_id = a.announcement_id`

func queryBookingAnnouncements(ctx context.Context, q bookingAnnouncementQuerier, where string, lockRows bool, args ...any) ([]model.BookingAnnouncement, error) {
	query := bookingAnnouncementSelect + where + ` ORDER BY a.start_date DESC, a.announcement_id DESC, v.position`
	if lockRows {
		query += ` FOR SHARE OF a, v`
	}
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	announcements := make([]model.BookingAnnouncement, 0)
	byID := make(map[int64]int)
	for rows.Next() {
		var announcement model.BookingAnnouncement
		var variation model.BookingAnnouncementVariation
		if err := rows.Scan(
			&announcement.AnnouncementID, &announcement.Message, &announcement.StartDate, &announcement.EndDate,
			&announcement.WinnerVariationID, &announcement.CreatedAt, &announcement.UpdatedAt,
			&variation.VariationID, &variation.Message, &variation.Position, &variation.CreatedAt, &variation.UpdatedAt,
		); err != nil {
			return nil, err
		}
		variation.AnnouncementID = announcement.AnnouncementID
		index, ok := byID[announcement.AnnouncementID]
		if !ok {
			index = len(announcements)
			byID[announcement.AnnouncementID] = index
			announcement.Variations = make([]model.BookingAnnouncementVariation, 0, 1)
			announcements = append(announcements, announcement)
		}
		announcements[index].Variations = append(announcements[index].Variations, variation)
	}
	return announcements, rows.Err()
}

func (r *bookingAnnouncementRepo) List(ctx context.Context) ([]model.BookingAnnouncement, error) {
	return queryBookingAnnouncements(ctx, r.db, "", false)
}

func (r *bookingAnnouncementRepo) Create(ctx context.Context, announcement *model.BookingAnnouncement) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	err = tx.QueryRow(ctx, `
		INSERT INTO booking_announcements (message, start_date, end_date)
		VALUES ($1, $2, $3)
		RETURNING announcement_id, created_at, updated_at`,
		announcement.Message, announcement.StartDate, announcement.EndDate,
	).Scan(&announcement.AnnouncementID, &announcement.CreatedAt, &announcement.UpdatedAt)
	if err != nil {
		return err
	}
	for i := range announcement.Variations {
		variation := &announcement.Variations[i]
		variation.AnnouncementID = announcement.AnnouncementID
		if err := tx.QueryRow(ctx, `
			INSERT INTO booking_announcement_variations (announcement_id, message, position)
			VALUES ($1, $2, $3)
			RETURNING variation_id, created_at, updated_at`,
			variation.AnnouncementID, variation.Message, variation.Position,
		).Scan(&variation.VariationID, &variation.CreatedAt, &variation.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *bookingAnnouncementRepo) Update(ctx context.Context, announcement *model.BookingAnnouncement) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var winnerID *int64
	if err := tx.QueryRow(ctx, `SELECT winner_variation_id FROM booking_announcements WHERE announcement_id = $1 FOR UPDATE`, announcement.AnnouncementID).Scan(&winnerID); err != nil {
		return err
	}
	if winnerID != nil {
		winnerRetained := false
		for _, variation := range announcement.Variations {
			winnerRetained = winnerRetained || variation.VariationID == *winnerID
		}
		if !winnerRetained {
			return ErrWinningVariationRequired
		}
	}

	if err := tx.QueryRow(ctx, `
		UPDATE booking_announcements
		SET message = $2, start_date = $3, end_date = $4, updated_at = NOW()
		WHERE announcement_id = $1
		RETURNING created_at, updated_at`,
		announcement.AnnouncementID, announcement.Message, announcement.StartDate, announcement.EndDate,
	).Scan(&announcement.CreatedAt, &announcement.UpdatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE booking_announcement_variations SET position = position + 1000000 WHERE announcement_id = $1`, announcement.AnnouncementID); err != nil {
		return err
	}

	retainedIDs := make([]int64, 0, len(announcement.Variations))
	for i := range announcement.Variations {
		variation := &announcement.Variations[i]
		variation.AnnouncementID = announcement.AnnouncementID
		if variation.VariationID == 0 {
			if err := tx.QueryRow(ctx, `
				INSERT INTO booking_announcement_variations (announcement_id, message, position)
				VALUES ($1, $2, $3)
				RETURNING variation_id, created_at, updated_at`,
				variation.AnnouncementID, variation.Message, variation.Position,
			).Scan(&variation.VariationID, &variation.CreatedAt, &variation.UpdatedAt); err != nil {
				return err
			}
		} else if err := tx.QueryRow(ctx, `
			UPDATE booking_announcement_variations
			SET message = $3, position = $4, updated_at = NOW()
			WHERE variation_id = $1 AND announcement_id = $2
			RETURNING created_at, updated_at`,
			variation.VariationID, announcement.AnnouncementID, variation.Message, variation.Position,
		).Scan(&variation.CreatedAt, &variation.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrVariationNotInCampaign
			}
			return err
		}
		retainedIDs = append(retainedIDs, variation.VariationID)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM booking_announcement_variations WHERE announcement_id = $1 AND NOT (variation_id = ANY($2::bigint[]))`, announcement.AnnouncementID, retainedIDs); err != nil {
		return err
	}
	announcement.WinnerVariationID = winnerID
	return tx.Commit(ctx)
}

func (r *bookingAnnouncementRepo) Delete(ctx context.Context, announcementID int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM booking_announcements WHERE announcement_id = $1`, announcementID)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (r *bookingAnnouncementRepo) ChooseWinner(ctx context.Context, announcementID, variationID int64) (*model.BookingAnnouncement, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE booking_announcements
		SET winner_variation_id = $2, updated_at = NOW()
		WHERE announcement_id = $1
		  AND EXISTS (
			SELECT 1 FROM booking_announcement_variations
			WHERE announcement_id = $1 AND variation_id = $2
		  )`, announcementID, variationID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM booking_announcements WHERE announcement_id = $1)`, announcementID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, pgx.ErrNoRows
		}
		return nil, ErrVariationNotInCampaign
	}
	announcements, err := queryBookingAnnouncements(ctx, r.db, " WHERE a.announcement_id = $1", false, announcementID)
	if err != nil {
		return nil, err
	}
	if len(announcements) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &announcements[0], nil
}

func (r *bookingAnnouncementRepo) ResolveForBooking(ctx context.Context, bookingID int64) ([]model.ResolvedBookingAnnouncement, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var bookingDate *time.Time
	var groupID *int64
	if err := tx.QueryRow(ctx, `
		SELECT business_day(scheduled_start), group_id
		FROM bookings WHERE booking_id = $1`, bookingID,
	).Scan(&bookingDate, &groupID); err != nil {
		return nil, err
	}
	if bookingDate == nil {
		return []model.ResolvedBookingAnnouncement{}, tx.Commit(ctx)
	}

	announcements, err := queryBookingAnnouncements(ctx, tx, " WHERE $1 BETWEEN a.start_date AND a.end_date", true, *bookingDate)
	if err != nil {
		return nil, err
	}
	resolved := make([]model.ResolvedBookingAnnouncement, 0, len(announcements))
	for _, announcement := range announcements {
		subjectID := bookingID
		if groupID != nil {
			subjectID = *groupID
		}
		variation := model.ChooseBookingAnnouncementVariation(announcement, subjectID)
		if variation.VariationID == 0 {
			continue
		}

		item := model.ResolvedBookingAnnouncement{AnnouncementID: announcement.AnnouncementID}
		if groupID != nil {
			err = tx.QueryRow(ctx, `
				INSERT INTO booking_announcement_assignments (announcement_id, variation_id, group_id, message_snapshot)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (announcement_id, group_id) WHERE group_id IS NOT NULL
				DO UPDATE SET announcement_id = EXCLUDED.announcement_id
				RETURNING COALESCE(variation_id, 0), message_snapshot`,
				announcement.AnnouncementID, variation.VariationID, *groupID, variation.Message,
			).Scan(&item.VariationID, &item.Message)
		} else {
			err = tx.QueryRow(ctx, `
				INSERT INTO booking_announcement_assignments (announcement_id, variation_id, booking_id, message_snapshot)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (announcement_id, booking_id) WHERE booking_id IS NOT NULL
				DO UPDATE SET announcement_id = EXCLUDED.announcement_id
				RETURNING COALESCE(variation_id, 0), message_snapshot`,
				announcement.AnnouncementID, variation.VariationID, bookingID, variation.Message,
			).Scan(&item.VariationID, &item.Message)
		}
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, item)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return resolved, nil
}
