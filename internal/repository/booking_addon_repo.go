package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// BookingAddonRepository defines the interface for booking addon data access.
type BookingAddonRepository interface {
	CreateTx(ctx context.Context, tx pgx.Tx, addon *model.BookingAddon) error
	CreateManyTx(ctx context.Context, tx pgx.Tx, addons []model.BookingAddon) error
	ListByBookingID(ctx context.Context, bookingID int64) ([]model.BookingAddon, error)
	ListByBookingIDWithProducts(ctx context.Context, bookingID int64) ([]model.BookingAddon, error)
	Delete(ctx context.Context, addonID int64) error
}

type bookingAddonRepo struct {
	db db.DBTX
}

// NewBookingAddonRepository creates a new BookingAddonRepository.
func NewBookingAddonRepository(db db.DBTX) BookingAddonRepository {
	return &bookingAddonRepo{db: db}
}

func (r *bookingAddonRepo) CreateTx(ctx context.Context, tx pgx.Tx, addon *model.BookingAddon) error {
	query := `
		INSERT INTO booking_addons (booking_id, product_id, quantity, price_at_booking)
		VALUES ($1, $2, $3, $4)
		RETURNING addon_id, created_at
	`
	return tx.QueryRow(ctx, query,
		addon.BookingID, addon.ProductID, addon.Quantity, addon.PriceAtBooking,
	).Scan(&addon.AddonID, &addon.CreatedAt)
}

func (r *bookingAddonRepo) CreateManyTx(ctx context.Context, tx pgx.Tx, addons []model.BookingAddon) error {
	if len(addons) == 0 {
		return nil
	}

	// Bulk insert using COPY or UNNEST
	// For simplicity and since addons usually isn't massive (e.g. < 50), we can use UNNEST with a single query
	bIDs := make([]int64, len(addons))
	pIDs := make([]int64, len(addons))
	qty := make([]int, len(addons))
	prices := make([]float64, len(addons))

	for i, a := range addons {
		bIDs[i] = a.BookingID
		pIDs[i] = a.ProductID
		qty[i] = a.Quantity
		prices[i] = a.PriceAtBooking
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO booking_addons (booking_id, product_id, quantity, price_at_booking)
		SELECT UNNEST($1::bigint[]), UNNEST($2::bigint[]), UNNEST($3::int[]), UNNEST($4::float8[])
	`, bIDs, pIDs, qty, prices)
	return err
}

func (r *bookingAddonRepo) ListByBookingID(ctx context.Context, bookingID int64) ([]model.BookingAddon, error) {
	query := `
		SELECT addon_id, booking_id, product_id, quantity, price_at_booking, created_at
		FROM booking_addons WHERE booking_id = $1
	`
	rows, err := r.db.Query(ctx, query, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addons []model.BookingAddon
	for rows.Next() {
		var a model.BookingAddon
		if err := rows.Scan(&a.AddonID, &a.BookingID, &a.ProductID, &a.Quantity, &a.PriceAtBooking, &a.CreatedAt); err != nil {
			return nil, err
		}
		addons = append(addons, a)
	}
	return addons, rows.Err()
}

func (r *bookingAddonRepo) ListByBookingIDWithProducts(ctx context.Context, bookingID int64) ([]model.BookingAddon, error) {
	query := `
		SELECT ba.addon_id, ba.booking_id, ba.product_id, ba.quantity, ba.price_at_booking, ba.created_at,
		       p.product_id, p.name, p.description, p.price, p.image_url, p.category, p.is_active, p.created_at, p.updated_at
		FROM booking_addons ba
		JOIN products p ON ba.product_id = p.product_id
		WHERE ba.booking_id = $1
	`
	rows, err := r.db.Query(ctx, query, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addons []model.BookingAddon
	for rows.Next() {
		var a model.BookingAddon
		var p model.Product
		if err := rows.Scan(
			&a.AddonID, &a.BookingID, &a.ProductID, &a.Quantity, &a.PriceAtBooking, &a.CreatedAt,
			&p.ProductID, &p.Name, &p.Description, &p.Price, &p.ImageURL, &p.Category, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		a.Product = &p
		addons = append(addons, a)
	}
	return addons, rows.Err()
}

func (r *bookingAddonRepo) Delete(ctx context.Context, addonID int64) error {
	query := `DELETE FROM booking_addons WHERE addon_id = $1`
	_, err := r.db.Exec(ctx, query, addonID)
	return err
}
