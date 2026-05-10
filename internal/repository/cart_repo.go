package repository

import (
	"context"
	"encoding/json"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// CartRepository defines persistence methods for shopping carts.
type CartRepository interface {
	// GetOrCreateCart returns the user's cart, creating one if it doesn't exist.
	GetOrCreateCart(ctx context.Context, userID int64) (*model.Cart, error)
	// GetCartByUserID returns the user's cart with all items populated.
	GetCartByUserID(ctx context.Context, userID int64) (*model.Cart, error)
	// AddItem adds an item to the user's cart.
	AddItem(ctx context.Context, userID int64, item *model.CartItem) error
	// UpdateItem updates an existing cart item.
	UpdateItem(ctx context.Context, itemID int64, req *model.UpdateCartItemRequest) error
	// RemoveItem removes an item from the cart.
	RemoveItem(ctx context.Context, itemID int64) error
	// ClearCart removes all items from a user's cart.
	ClearCart(ctx context.Context, userID int64) error
}

type cartRepo struct {
	db db.DBTX
}

// NewCartRepository creates a new CartRepository.
func NewCartRepository(db db.DBTX) CartRepository {
	return &cartRepo{db: db}
}

func (r *cartRepo) GetOrCreateCart(ctx context.Context, userID int64) (*model.Cart, error) {
	var cart model.Cart

	// Try to get existing cart
	err := r.db.QueryRow(ctx, `
		SELECT cart_id, user_id, created_at, updated_at
		FROM carts
		WHERE user_id = $1
	`, userID).Scan(&cart.CartID, &cart.UserID, &cart.CreatedAt, &cart.UpdatedAt)

	if err != nil {
		// If not found, create one
		err = r.db.QueryRow(ctx, `
			INSERT INTO carts (user_id)
			VALUES ($1)
			RETURNING cart_id, user_id, created_at, updated_at
		`, userID).Scan(&cart.CartID, &cart.UserID, &cart.CreatedAt, &cart.UpdatedAt)
		if err != nil {
			return nil, err
		}
	}

	return &cart, nil
}

func (r *cartRepo) GetCartByUserID(ctx context.Context, userID int64) (*model.Cart, error) {
	cart, err := r.GetOrCreateCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get all items with joined service data
	rows, err := r.db.Query(ctx, `
		SELECT 
			ci.cart_item_id, ci.cart_id, ci.service_id,
			ci.guest_name, ci.duration_minutes, ci.gender_preference,
			ci.pressure_preference, COALESCE(ci.notes, ''), ci.sequence_number,
			ci.start_condition, ci.addons, ci.date_added,
			s.service_id, s.name, COALESCE(s.description, ''), s.base_price,
			s.duration_minutes, COALESCE(s.category, ''), s.is_active,
			COALESCE(s.preview_image_url, '')
		FROM cart_items ci
		JOIN services s ON ci.service_id = s.service_id
		WHERE ci.cart_id = $1
		ORDER BY ci.sequence_number ASC, ci.date_added ASC
	`, cart.CartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.CartItem
	for rows.Next() {
		var item model.CartItem
		var svc model.Service
		var addonsJSON []byte

		if err := rows.Scan(
			&item.CartItemID, &item.CartID, &item.ServiceID,
			&item.GuestName, &item.DurationMinutes, &item.GenderPreference,
			&item.PressurePreference, &item.Notes, &item.SequenceNumber,
			&item.StartCondition, &addonsJSON, &item.DateAdded,
			&svc.ServiceID, &svc.Name, &svc.Description, &svc.BasePrice,
			&svc.DurationMinutes, &svc.Category, &svc.IsActive,
			&svc.PreviewImageURL,
		); err != nil {
			return nil, err
		}

		// Parse addons JSON
		if err := item.ParseAddonsJSON(addonsJSON); err != nil {
			return nil, err
		}

		item.Service = &svc
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	cart.Items = items
	return cart, nil
}

func (r *cartRepo) AddItem(ctx context.Context, userID int64, item *model.CartItem) error {
	// Get or create cart
	cart, err := r.GetOrCreateCart(ctx, userID)
	if err != nil {
		return err
	}

	// Determine sequence number
	var maxSeq int
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_number), -1)
		FROM cart_items
		WHERE cart_id = $1
	`, cart.CartID).Scan(&maxSeq)
	if err != nil {
		return err
	}
	item.SequenceNumber = maxSeq + 1

	// Serialize addons
	addonsJSON, err := item.AddonsJSON()
	if err != nil {
		return err
	}

	// Insert item
	return r.db.QueryRow(ctx, `
		INSERT INTO cart_items (
			cart_id, service_id, guest_name, duration_minutes,
			gender_preference, pressure_preference, notes,
			sequence_number, start_condition, addons
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING cart_item_id, date_added
	`,
		cart.CartID, item.ServiceID, item.GuestName, item.DurationMinutes,
		item.GenderPreference, item.PressurePreference, item.Notes,
		item.SequenceNumber, item.StartCondition, addonsJSON,
	).Scan(&item.CartItemID, &item.DateAdded)
}

func (r *cartRepo) UpdateItem(ctx context.Context, itemID int64, req *model.UpdateCartItemRequest) error {
	// Build dynamic update query
	query := "UPDATE cart_items SET "
	args := []interface{}{}
	argNum := 1

	if req.GuestName != nil {
		query += "guest_name = $" + string(rune('0'+argNum)) + ", "
		args = append(args, *req.GuestName)
		argNum++
	}
	if req.DurationMinutes != nil {
		query += "duration_minutes = $" + string(rune('0'+argNum)) + ", "
		args = append(args, *req.DurationMinutes)
		argNum++
	}
	if req.GenderPreference != nil {
		query += "gender_preference = $" + string(rune('0'+argNum)) + ", "
		args = append(args, *req.GenderPreference)
		argNum++
	}
	if req.PressurePreference != nil {
		query += "pressure_preference = $" + string(rune('0'+argNum)) + ", "
		args = append(args, *req.PressurePreference)
		argNum++
	}
	if req.Notes != nil {
		query += "notes = $" + string(rune('0'+argNum)) + ", "
		args = append(args, *req.Notes)
		argNum++
	}
	if req.StartCondition != nil {
		query += "start_condition = $" + string(rune('0'+argNum)) + ", "
		args = append(args, *req.StartCondition)
		argNum++
	}
	if req.Addons != nil {
		addonsJSON, err := json.Marshal(*req.Addons)
		if err != nil {
			return err
		}
		query += "addons = $" + string(rune('0'+argNum)) + ", "
		args = append(args, addonsJSON)
		argNum++
	}

	// Remove trailing comma and space
	if len(args) == 0 {
		return nil // Nothing to update
	}
	query = query[:len(query)-2]

	// Add WHERE clause
	query += " WHERE cart_item_id = $" + string(rune('0'+argNum))
	args = append(args, itemID)

	_, err := r.db.Exec(ctx, query, args...)
	return err
}

func (r *cartRepo) RemoveItem(ctx context.Context, itemID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM cart_items WHERE cart_item_id = $1`, itemID)
	return err
}

func (r *cartRepo) ClearCart(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM cart_items
		WHERE cart_id = (SELECT cart_id FROM carts WHERE user_id = $1)
	`, userID)
	return err
}
