package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

// ProductCatalog exposes business logic for product catalog entries.
type ProductCatalog struct {
	repo           repository.ProductRepository
	storageService StorageService
}

// NewProductCatalog creates a new product catalog with the given repository.
func NewProductCatalog(repo repository.ProductRepository, storageService StorageService) *ProductCatalog {
	return &ProductCatalog{repo: repo, storageService: storageService}
}

// Create validates and inserts a new product.
func (c *ProductCatalog) Create(ctx context.Context, req *model.CreateProductRequest) (*model.Product, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	if req.Price < 0 {
		return nil, fmt.Errorf("price must be non-negative")
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	var imageURL *string
	if req.ImageURL != nil {
		trimmed := strings.TrimSpace(*req.ImageURL)
		imageURL = &trimmed
	}

	p := &model.Product{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Price:       req.Price,
		ImageURL:    imageURL,
		Category:    strings.TrimSpace(req.Category),
		IsActive:    isActive,
	}

	if err := c.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return p, nil
}

// Update modifies an existing product.
func (c *ProductCatalog) Update(ctx context.Context, productID int64, req *model.UpdateProductRequest) (*model.Product, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	p, err := c.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("product not found: %w", err)
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		p.Name = name
	}
	if req.Description != nil {
		desc := strings.TrimSpace(*req.Description)
		p.Description = desc
	}
	if req.Price != nil {
		if *req.Price < 0 {
			return nil, fmt.Errorf("price must be non-negative")
		}
		p.Price = *req.Price
	}
	if req.ImageURL != nil {
		trimmed := strings.TrimSpace(*req.ImageURL)
		p.ImageURL = &trimmed
	}
	if req.Category != nil {
		cat := strings.TrimSpace(*req.Category)
		p.Category = cat
	}
	if req.IsActive != nil {
		p.IsActive = *req.IsActive
	}

	if err := c.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return p, nil
}

// Delete removes a product by ID.
func (c *ProductCatalog) Delete(ctx context.Context, productID int64) error {
	if err := c.repo.Delete(ctx, productID); err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	return nil
}

// ListAll returns all products (active and inactive) for admin use.
func (c *ProductCatalog) ListAll(ctx context.Context) ([]model.Product, error) {
	return c.repo.ListAll(ctx)
}

// ListActive returns only active products for public use.
func (c *ProductCatalog) ListActive(ctx context.Context) ([]model.Product, error) {
	return c.repo.ListActive(ctx)
}

// GetByID returns a single product by ID.
func (c *ProductCatalog) GetByID(ctx context.Context, productID int64) (*model.Product, error) {
	return c.repo.GetByID(ctx, productID)
}
