package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type ApplicationRepository interface {
	Create(ctx context.Context, app *model.ApplicantApplication) error
	GetByID(ctx context.Context, applicationID int64) (*model.ApplicantApplication, error)
	List(ctx context.Context, filters model.ListApplicationsFilters) ([]model.ApplicantApplication, int, error)
	UpdateStatus(ctx context.Context, applicationID int64, status model.ApplicationStatus, reviewedBy int64, notes string) (*model.ApplicantApplication, error)
}

type applicationRepoImpl struct {
	db db.DBTX
}

func NewApplicationRepository(db db.DBTX) ApplicationRepository {
	return &applicationRepoImpl{db: db}
}

func (r *applicationRepoImpl) Create(ctx context.Context, app *model.ApplicantApplication) error {
	answersJSON, err := json.Marshal(app.Answers)
	if err != nil {
		return fmt.Errorf("marshal answers: %w", err)
	}
	attachmentsJSON, err := json.Marshal(app.Attachments)
	if err != nil {
		return fmt.Errorf("marshal attachments: %w", err)
	}

	return r.db.QueryRow(ctx, `
		INSERT INTO applicant_applications (
			user_id, target_role, position_applied, preferred_branch_id, preferred_branch_label,
			status, answers_json, attachments_json, review_notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING application_id, submitted_at
	`,
		app.UserID,
		app.TargetRole,
		app.PositionApplied,
		app.PreferredBranchID,
		app.PreferredBranchLabel,
		app.Status,
		answersJSON,
		attachmentsJSON,
		app.ReviewNotes,
	).Scan(&app.ApplicationID, &app.SubmittedAt)
}

func (r *applicationRepoImpl) GetByID(ctx context.Context, applicationID int64) (*model.ApplicantApplication, error) {
	apps, _, err := r.listInternal(ctx, model.ListApplicationsFilters{
		Page:  1,
		Limit: 1,
	}, "a.application_id = $1", []interface{}{applicationID})
	if err != nil {
		return nil, err
	}
	if len(apps) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &apps[0], nil
}

func (r *applicationRepoImpl) List(ctx context.Context, filters model.ListApplicationsFilters) ([]model.ApplicantApplication, int, error) {
	var where []string
	var args []interface{}
	argIdx := 1

	if role := strings.TrimSpace(strings.ToLower(filters.Role)); role != "" {
		where = append(where, fmt.Sprintf("a.target_role = $%d", argIdx))
		args = append(args, role)
		argIdx++
	}
	if status := strings.TrimSpace(strings.ToLower(filters.Status)); status != "" {
		where = append(where, fmt.Sprintf("a.status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if q := strings.TrimSpace(strings.ToLower(filters.Search)); q != "" {
		where = append(where, fmt.Sprintf("(LOWER(COALESCE(u.full_name, '')) LIKE $%d OR LOWER(COALESCE(u.primary_email, '')) LIKE $%d OR COALESCE(u.primary_phone, '') LIKE $%d)", argIdx, argIdx, argIdx))
		args = append(args, "%"+q+"%")
		argIdx++
	}

	condition := "1=1"
	if len(where) > 0 {
		condition = strings.Join(where, " AND ")
	}
	return r.listInternal(ctx, filters, condition, args)
}

func (r *applicationRepoImpl) listInternal(ctx context.Context, filters model.ListApplicationsFilters, whereClause string, args []interface{}) ([]model.ApplicantApplication, int, error) {
	page := filters.Page
	if page <= 0 {
		page = 1
	}
	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	countQuery := "SELECT COUNT(1) FROM applicant_applications a LEFT JOIN users u ON u.user_id = a.user_id WHERE " + whereClause
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery := `
		SELECT
			a.application_id, a.user_id, a.target_role, a.position_applied,
			a.preferred_branch_id, COALESCE(a.preferred_branch_label, ''), a.status,
			COALESCE(a.answers_json, '{}'::jsonb), COALESCE(a.attachments_json, '{}'::jsonb),
			a.submitted_at, a.reviewed_at, a.reviewed_by_admin_id, COALESCE(a.review_notes, ''),
			COALESCE(u.full_name, ''), COALESCE(u.primary_email, ''), COALESCE(u.primary_phone, '')
		FROM applicant_applications a
		LEFT JOIN users u ON u.user_id = a.user_id
		WHERE ` + whereClause + `
		ORDER BY a.submitted_at DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Query(ctx, listQuery, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]model.ApplicantApplication, 0, limit)
	for rows.Next() {
		var app model.ApplicantApplication
		var answersRaw []byte
		var attachmentsRaw []byte
		if err := rows.Scan(
			&app.ApplicationID,
			&app.UserID,
			&app.TargetRole,
			&app.PositionApplied,
			&app.PreferredBranchID,
			&app.PreferredBranchLabel,
			&app.Status,
			&answersRaw,
			&attachmentsRaw,
			&app.SubmittedAt,
			&app.ReviewedAt,
			&app.ReviewedByAdminID,
			&app.ReviewNotes,
			&app.ApplicantNameSnapshot,
			&app.ApplicantEmail,
			&app.ApplicantPhone,
		); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(answersRaw, &app.Answers); err != nil {
			app.Answers = map[string]interface{}{}
		}
		if err := json.Unmarshal(attachmentsRaw, &app.Attachments); err != nil {
			app.Attachments = map[string]interface{}{}
		}
		out = append(out, app)
	}
	return out, total, rows.Err()
}

func (r *applicationRepoImpl) UpdateStatus(ctx context.Context, applicationID int64, status model.ApplicationStatus, reviewedBy int64, notes string) (*model.ApplicantApplication, error) {
	_, err := r.db.Exec(ctx, `
		UPDATE applicant_applications
		SET status = $1, reviewed_by_admin_id = $2, review_notes = $3, reviewed_at = CURRENT_TIMESTAMP
		WHERE application_id = $4
	`, status, reviewedBy, notes, applicationID)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, applicationID)
}
