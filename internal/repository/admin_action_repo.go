package repository

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// AdminActionRepository manages admin audit logs.
type AdminActionRepository interface {
	Log(ctx context.Context, action *model.AdminAction) error
	GetByAdmin(ctx context.Context, adminID int64, limit int) ([]model.AdminAction, error)
	GetAll(ctx context.Context, limit int) ([]model.AdminAction, error)
}

type adminActionRepoImpl struct {
	db db.DBTX
}

func NewAdminActionRepository(db db.DBTX) AdminActionRepository {
	return &adminActionRepoImpl{db: db}
}

func (r *adminActionRepoImpl) Log(ctx context.Context, action *model.AdminAction) error {
	query := `
		INSERT INTO admin_actions (
			admin_id, action_type, target_type, target_id, description,
			old_value, new_value, ip_address, performed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8, NOW())
		RETURNING action_id, performed_at
	`
	return r.db.QueryRow(ctx, query,
		action.AdminID,
		action.ActionType,
		action.TargetType,
		action.TargetID,
		action.Description,
		action.OldValue,
		action.NewValue,
		action.IPAddress,
	).Scan(&action.ActionID, &action.PerformedAt)
}

func (r *adminActionRepoImpl) GetByAdmin(ctx context.Context, adminID int64, limit int) ([]model.AdminAction, error) {
	query := `
		SELECT action_id, admin_id, action_type, target_type, target_id, description,
		       old_value, new_value, ip_address, performed_at
		FROM admin_actions
		WHERE admin_id = $1
		ORDER BY performed_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, adminID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []model.AdminAction
	for rows.Next() {
		var a model.AdminAction
		if err := rows.Scan(&a.ActionID, &a.AdminID, &a.ActionType, &a.TargetType, &a.TargetID, &a.Description, &a.OldValue, &a.NewValue, &a.IPAddress, &a.PerformedAt); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

func (r *adminActionRepoImpl) GetAll(ctx context.Context, limit int) ([]model.AdminAction, error) {
	query := `
		SELECT action_id, admin_id, action_type, target_type, target_id, description,
		       old_value, new_value, ip_address, performed_at
		FROM admin_actions
		ORDER BY performed_at DESC
		LIMIT $1
	`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []model.AdminAction
	for rows.Next() {
		var a model.AdminAction
		if err := rows.Scan(&a.ActionID, &a.AdminID, &a.ActionType, &a.TargetType, &a.TargetID, &a.Description, &a.OldValue, &a.NewValue, &a.IPAddress, &a.PerformedAt); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}
