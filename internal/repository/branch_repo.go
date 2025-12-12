package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BranchRepository manages branches.
type BranchRepository interface {
	Create(ctx context.Context, b *model.Branch) error
	GetByID(ctx context.Context, branchID int64) (*model.Branch, error)
	List(ctx context.Context, activeOnly bool) ([]model.Branch, error)
	Update(ctx context.Context, branchID int64, updates map[string]interface{}) error
}

type branchRepoImpl struct {
	db *pgxpool.Pool
}

func NewBranchRepository(db *pgxpool.Pool) BranchRepository {
	return &branchRepoImpl{db: db}
}

func (r *branchRepoImpl) Create(ctx context.Context, b *model.Branch) error {
	query := `
        INSERT INTO branches (
            branch_name, address_line, barangay, city, province, postal_code,
            latitude, longitude, contact_no, email, is_active
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        RETURNING branch_id, created_at, updated_at
    `
	return r.db.QueryRow(ctx, query,
		b.BranchName,
		b.AddressLine,
		b.Barangay,
		b.City,
		b.Province,
		b.PostalCode,
		b.Latitude,
		b.Longitude,
		b.ContactNo,
		b.Email,
		b.IsActive,
	).Scan(&b.BranchID, &b.CreatedAt, &b.UpdatedAt)
}

func (r *branchRepoImpl) GetByID(ctx context.Context, branchID int64) (*model.Branch, error) {
	query := `
        SELECT branch_id, branch_name, address_line, barangay, city, province, postal_code,
               latitude, longitude, contact_no, email, is_active, created_at, updated_at
        FROM branches
        WHERE branch_id = $1
    `
	var b model.Branch
	if err := r.db.QueryRow(ctx, query, branchID).Scan(
		&b.BranchID,
		&b.BranchName,
		&b.AddressLine,
		&b.Barangay,
		&b.City,
		&b.Province,
		&b.PostalCode,
		&b.Latitude,
		&b.Longitude,
		&b.ContactNo,
		&b.Email,
		&b.IsActive,
		&b.CreatedAt,
		&b.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *branchRepoImpl) List(ctx context.Context, activeOnly bool) ([]model.Branch, error) {
	query := `
        SELECT branch_id, branch_name, address_line, barangay, city, province, postal_code,
               latitude, longitude, contact_no, email, is_active, created_at, updated_at
        FROM branches
    `
	if activeOnly {
		query += " WHERE is_active = TRUE"
	}
	query += " ORDER BY branch_name"

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []model.Branch
	for rows.Next() {
		var b model.Branch
		if err := rows.Scan(&b.BranchID, &b.BranchName, &b.AddressLine, &b.Barangay, &b.City, &b.Province, &b.PostalCode, &b.Latitude, &b.Longitude, &b.ContactNo, &b.Email, &b.IsActive, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		branches = append(branches, b)
	}
	return branches, rows.Err()
}

func (r *branchRepoImpl) Update(ctx context.Context, branchID int64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	var setClauses []string
	var args []interface{}
	argIdx := 1

	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, branchID)

	query := fmt.Sprintf("UPDATE branches SET %s WHERE branch_id = $%d", strings.Join(setClauses, ", "), argIdx)

	cmd, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
