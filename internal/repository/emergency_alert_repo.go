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
	List(ctx context.Context, status string, limit int) ([]*model.EmergencyAlert, error)
	CountByStatus(ctx context.Context, status string) (int, error)
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
        RETURNING alert_id, triggered_at, resolved
    `
	err := r.db.QueryRow(ctx, query,
		alert.BookingID,
		alert.TriggeredBy,
		alert.LocationLat,
		alert.LocationLng,
		alert.Status,
	).Scan(&alert.AlertID, &alert.TriggeredAt, &alert.Resolved)
	if err != nil {
		return err
	}
	alert.CreatedAt = alert.TriggeredAt
	alert.UpdatedAt = alert.TriggeredAt
	return nil
}

func (r *emergencyAlertRepoImpl) GetByID(ctx context.Context, alertID int64) (*model.EmergencyAlert, error) {
	query := `
        SELECT alert_id, booking_id, triggered_by, triggered_at, location_lat, location_lng,
               status, resolved, resolved_at, resolved_by, resolution_notes
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
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}
	a.CreatedAt = a.TriggeredAt
	a.UpdatedAt = a.TriggeredAt
	return &a, nil
}

func (r *emergencyAlertRepoImpl) Resolve(ctx context.Context, alertID, resolverID int64, status, note string) error {
	cmd, err := r.db.Exec(ctx, `
        UPDATE emergency_alerts
        SET status = $1,
            resolved = TRUE,
            resolved_at = CURRENT_TIMESTAMP,
            resolved_by = $2,
            resolution_notes = $3
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

func (r *emergencyAlertRepoImpl) List(ctx context.Context, status string, limit int) ([]*model.EmergencyAlert, error) {
	query := `
        SELECT alert_id, booking_id, triggered_by, triggered_at, location_lat, location_lng,
               status, resolved, resolved_at, resolved_by, resolution_notes
        FROM emergency_alerts
        WHERE ($1 = '' OR status = $1)
        ORDER BY triggered_at DESC
        LIMIT $2
    `
	rows, err := r.db.Query(ctx, query, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*model.EmergencyAlert
	for rows.Next() {
		var a model.EmergencyAlert
		if err := rows.Scan(
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
		); err != nil {
			return nil, err
		}
		a.CreatedAt = a.TriggeredAt
		a.UpdatedAt = a.TriggeredAt
		alerts = append(alerts, &a)
	}
	return alerts, nil
}

func (r *emergencyAlertRepoImpl) CountByStatus(ctx context.Context, status string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM emergency_alerts WHERE ($1 = '' OR status = $1)`
	err := r.db.QueryRow(ctx, query, status).Scan(&count)
	return count, err
}
