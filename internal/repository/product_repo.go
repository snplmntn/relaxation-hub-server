package repository

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// ProductRepository defines the interface for product data access.
type ProductRepository interface {
	Create(ctx context.Context, p *model.Product) error
	GetByID(ctx context.Context, productID int64) (*model.Product, error)
	ListActive(ctx context.Context) ([]model.Product, error)
	Update(ctx context.Context, p *model.Product) error
	Delete(ctx context.Context, productID int64) error
}

type productRepo struct {
	db db.DBTX
}

// NewProductRepository creates a new ProductRepository.
func NewProductRepository(db db.DBTX) ProductRepository {
	return &productRepo{db: db}
}

func (r *productRepo) Create(ctx context.Context, p *model.Product) error {
	query := `
		INSERT INTO products (name, description, price, image_url, category, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING product_id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		p.Name, p.Description, p.Price, p.ImageURL, p.Category, p.IsActive,
	).Scan(&p.ProductID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *productRepo) GetByID(ctx context.Context, productID int64) (*model.Product, error) {
	query := `
		SELECT product_id, name, description, price, image_url, category, is_active, created_at, updated_at
		FROM products WHERE product_id = $1
	`
	p := &model.Product{}
	err := r.db.QueryRow(ctx, query, productID).Scan(
		&p.ProductID, &p.Name, &p.Description, &p.Price, &p.ImageURL, &p.Category, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *productRepo) ListActive(ctx context.Context) ([]model.Product, error) {
	query := `
		SELECT product_id, name, description, price, image_url, category, is_active, created_at, updated_at
		FROM products WHERE is_active = TRUE ORDER BY name
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(
			&p.ProductID, &p.Name, &p.Description, &p.Price, &p.ImageURL, &p.Category, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (r *productRepo) Update(ctx context.Context, p *model.Product) error {
	query := `
		UPDATE products SET name = $1, description = $2, price = $3, image_url = $4, category = $5, is_active = $6, updated_at = NOW()
		WHERE product_id = $7
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		p.Name, p.Description, p.Price, p.ImageURL, p.Category, p.IsActive, p.ProductID,
	).Scan(&p.UpdatedAt)
}

func (r *productRepo) Delete(ctx context.Context, productID int64) error {
	query := `DELETE FROM products WHERE product_id = $1`
	_, err := r.db.Exec(ctx, query, productID)
	return err
}
