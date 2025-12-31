package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// EmergencyAlertRepository manages emergency alerts.
type EmergencyAlertRepository interface {
	Create(ctx context.Context, alert *model.EmergencyAlert) error
	GetByID(ctx context.Context, alertID int64) (*model.EmergencyAlert, error)
	Resolve(ctx context.Context, alertID, resolverID int64, status, note string) error
}

type emergencyAlertRepoImpl struct {
	db db.DBTX
}

func NewEmergencyAlertRepository(db db.DBTX) EmergencyAlertRepository {
	return &emergencyAlertRepoImpl{db: db}
}

func (r *emergencyAlertRepoImpl) Create(ctx context.Context, alert *model.EmergencyAlert) error {
	query := `
        INSERT INTO emergency_alerts (booking_id, triggered_by, location_lat, location_lng, status)
        VALUES ($1,$2,$3,$4,$5)
        RETURNING alert_id, triggered_at, resolved, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query,
		alert.BookingID,
		alert.TriggeredBy,
		alert.LocationLat,
		alert.LocationLng,
		alert.Status,
	).Scan(&alert.AlertID, &alert.TriggeredAt, &alert.Resolved, &alert.CreatedAt, &alert.UpdatedAt)
}

func (r *emergencyAlertRepoImpl) GetByID(ctx context.Context, alertID int64) (*model.EmergencyAlert, error) {
	query := `
        SELECT alert_id, booking_id, triggered_by, triggered_at, location_lat, location_lng,
               status, resolved, resolved_at, resolved_by, resolution_notes, created_at, updated_at
        FROM emergency_alerts
        WHERE alert_id = $1
    `
	var a model.EmergencyAlert
	if err := r.db.QueryRow(ctx, query, alertID).Scan(
		&a.AlertID,
		&a.BookingID,
		&a.TriggeredBy,
		&a.TriggeredAt,
		&a.LocationLat,
		&a.LocationLng,
		&a.Status,
		&a.Resolved,
		&a.ResolvedAt,
		&a.ResolvedBy,
		&a.ResolutionNote,
		&a.CreatedAt,
		&a.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}
	return &a, nil
}

func (r *emergencyAlertRepoImpl) Resolve(ctx context.Context, alertID, resolverID int64, status, note string) error {
	cmd, err := r.db.Exec(ctx, `
        UPDATE emergency_alerts
        SET status = $1,
            resolved = TRUE,
            resolved_at = CURRENT_TIMESTAMP,
            resolved_by = $2,
            resolution_notes = $3,
            updated_at = CURRENT_TIMESTAMP
        WHERE alert_id = $4
    `, status, resolverID, note, alertID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
