package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// AddressRepository defines data access methods for addresses
type AddressRepository interface {
	Create(ctx context.Context, address *model.Address) error
	GetByID(ctx context.Context, addressID, userID int64) (*model.Address, error)
	ListForUser(ctx context.Context, userID int64, includeDeleted bool) ([]model.Address, error)
	Update(ctx context.Context, address *model.Address) error
	SetDefault(ctx context.Context, addressID, userID int64) error
	SoftDelete(ctx context.Context, addressID, userID int64) error
}

type addressRepoImpl struct {
	db *pgxpool.Pool
}

func NewAddressRepository(db *pgxpool.Pool) AddressRepository {
	return &addressRepoImpl{db: db}
}

func (r *addressRepoImpl) Create(ctx context.Context, address *model.Address) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM addresses WHERE user_id = $1 AND deleted_at IS NULL
	`, address.UserID).Scan(&count); err != nil {
		return err
	}

	isDefault := address.IsDefault || count == 0
	if isDefault {
		if _, err := tx.Exec(ctx, `
			UPDATE addresses SET is_default = FALSE WHERE user_id = $1 AND deleted_at IS NULL
		`, address.UserID); err != nil {
			return err
		}
	}

	query := `
		INSERT INTO addresses (
			user_id, label, street, city, province, postal_code, country,
			latitude, longitude, is_default
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10
		)
		RETURNING address_id, is_default, created_at, updated_at
	`

	if err := tx.QueryRow(ctx, query,
		address.UserID,
		address.Label,
		address.Street,
		address.City,
		address.Province,
		address.PostalCode,
		address.Country,
		address.Latitude,
		address.Longitude,
		isDefault,
	).Scan(&address.AddressID, &address.IsDefault, &address.CreatedAt, &address.UpdatedAt); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *addressRepoImpl) GetByID(ctx context.Context, addressID, userID int64) (*model.Address, error) {
	query := `
		SELECT address_id, user_id, label, street, city, province, postal_code,
		       country, latitude, longitude, is_default, deleted_at, created_at, updated_at
		FROM addresses
		WHERE address_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`
	row := r.db.QueryRow(ctx, query, addressID, userID)

	var addr model.Address
	if err := row.Scan(
		&addr.AddressID,
		&addr.UserID,
		&addr.Label,
		&addr.Street,
		&addr.City,
		&addr.Province,
		&addr.PostalCode,
		&addr.Country,
		&addr.Latitude,
		&addr.Longitude,
		&addr.IsDefault,
		&addr.DeletedAt,
		&addr.CreatedAt,
		&addr.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	return &addr, nil
}

func (r *addressRepoImpl) ListForUser(ctx context.Context, userID int64, includeDeleted bool) ([]model.Address, error) {
	query := `
		SELECT address_id, user_id, label, street, city, province, postal_code,
		       country, latitude, longitude, is_default, deleted_at, created_at, updated_at
		FROM addresses
		WHERE user_id = $1
	`
	if !includeDeleted {
		query += " AND deleted_at IS NULL"
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []model.Address
	for rows.Next() {
		var addr model.Address
		if err := rows.Scan(
			&addr.AddressID,
			&addr.UserID,
			&addr.Label,
			&addr.Street,
			&addr.City,
			&addr.Province,
			&addr.PostalCode,
			&addr.Country,
			&addr.Latitude,
			&addr.Longitude,
			&addr.IsDefault,
			&addr.DeletedAt,
			&addr.CreatedAt,
			&addr.UpdatedAt,
		); err != nil {
			return nil, err
		}
		addresses = append(addresses, addr)
	}

	return addresses, nil
}

func (r *addressRepoImpl) Update(ctx context.Context, address *model.Address) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE addresses
		SET label = $1,
		    street = $2,
		    city = $3,
		    province = $4,
		    postal_code = $5,
		    country = $6,
		    latitude = $7,
		    longitude = $8
		WHERE address_id = $9 AND user_id = $10 AND deleted_at IS NULL
	`, address.Label, address.Street, address.City, address.Province, address.PostalCode,
		address.Country, address.Latitude, address.Longitude, address.AddressID, address.UserID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *addressRepoImpl) SetDefault(ctx context.Context, addressID, userID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Ensure address belongs to user and not deleted
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM addresses WHERE address_id = $1 AND user_id = $2 AND deleted_at IS NULL
		)
	`, addressID, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}

	if _, err := tx.Exec(ctx, `
		UPDATE addresses SET is_default = FALSE WHERE user_id = $1
	`, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE addresses SET is_default = TRUE
		WHERE address_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, addressID, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *addressRepoImpl) SoftDelete(ctx context.Context, addressID, userID int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// mark deleted
	cmd, err := tx.Exec(ctx, `
		UPDATE addresses
		SET deleted_at = CURRENT_TIMESTAMP, is_default = FALSE
		WHERE address_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, addressID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	// promote another default if none remaining
	var hasDefault bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM addresses
			WHERE user_id = $1 AND is_default = TRUE AND deleted_at IS NULL
		)
	`, userID).Scan(&hasDefault); err != nil {
		return err
	}

	if !hasDefault {
		if _, err := tx.Exec(ctx, `
			UPDATE addresses
			SET is_default = TRUE
			WHERE address_id = (
				SELECT address_id FROM addresses
				WHERE user_id = $1 AND deleted_at IS NULL
				ORDER BY created_at DESC
				LIMIT 1
			)
		`, userID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
