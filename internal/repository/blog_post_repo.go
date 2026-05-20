package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type BlogPostRepository interface {
	Create(ctx context.Context, post *model.BlogPost) error
	GetAdminByID(ctx context.Context, id int64) (*model.BlogPost, error)
	GetBySlug(ctx context.Context, slug string) (*model.BlogPost, error)
	SlugExists(ctx context.Context, slug string, excludeID *int64) (bool, error)
	ListAdmin(ctx context.Context, status, q string, limit, offset int) ([]model.BlogPost, error)
	ListPublished(ctx context.Context, limit, offset int) ([]model.BlogPost, error)
	Update(ctx context.Context, post *model.BlogPost) error
	SoftDelete(ctx context.Context, id int64) error
}

type blogPostRepo struct {
	db db.DBTX
}

func NewBlogPostRepository(db db.DBTX) BlogPostRepository {
	return &blogPostRepo{db: db}
}

func (r *blogPostRepo) Create(ctx context.Context, post *model.BlogPost) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.db.QueryRow(ctx, `
		INSERT INTO blog_posts (
			title, slug, excerpt, cover_image_url, content_html, status,
			seo_title, seo_description, published_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING blog_post_id, created_at, updated_at
	`,
		post.Title,
		post.Slug,
		post.Excerpt,
		post.CoverImageURL,
		post.ContentHTML,
		post.Status,
		post.SEOTitle,
		post.SEODescription,
		post.PublishedAt,
	).Scan(&post.BlogPostID, &post.CreatedAt, &post.UpdatedAt)
}

func (r *blogPostRepo) GetAdminByID(ctx context.Context, id int64) (*model.BlogPost, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return scanBlogPost(r.db.QueryRow(ctx, blogPostSelectSQL()+`
		WHERE blog_post_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`, id))
}

func (r *blogPostRepo) GetBySlug(ctx context.Context, slug string) (*model.BlogPost, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return scanBlogPost(r.db.QueryRow(ctx, blogPostSelectSQL()+`
		WHERE slug = $1 AND status = 'published' AND deleted_at IS NULL
		LIMIT 1
	`, slug))
}

func (r *blogPostRepo) SlugExists(ctx context.Context, slug string, excludeID *int64) (bool, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var exists bool
	if excludeID != nil {
		err := r.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM blog_posts
				WHERE slug = $1 AND blog_post_id <> $2 AND deleted_at IS NULL
			)
		`, slug, *excludeID).Scan(&exists)
		return exists, err
	}

	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM blog_posts
			WHERE slug = $1 AND deleted_at IS NULL
		)
	`, slug).Scan(&exists)
	return exists, err
}

func (r *blogPostRepo) ListAdmin(ctx context.Context, status, q string, limit, offset int) ([]model.BlogPost, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, blogPostSelectSQL()+`
		WHERE deleted_at IS NULL
			AND ($1 = '' OR status = $1)
			AND ($2 = '' OR title ILIKE '%' || $2 || '%' OR excerpt ILIKE '%' || $2 || '%')
		ORDER BY updated_at DESC, blog_post_id DESC
		LIMIT $3 OFFSET $4
	`, status, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBlogPosts(rows)
}

func (r *blogPostRepo) ListPublished(ctx context.Context, limit, offset int) ([]model.BlogPost, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	rows, err := r.db.Query(ctx, blogPostSelectSQL()+`
		WHERE status = 'published' AND deleted_at IS NULL
		ORDER BY published_at DESC NULLS LAST, blog_post_id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanBlogPosts(rows)
}

func (r *blogPostRepo) Update(ctx context.Context, post *model.BlogPost) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	return r.db.QueryRow(ctx, `
		UPDATE blog_posts
		SET
			title = $1,
			slug = $2,
			excerpt = $3,
			cover_image_url = $4,
			content_html = $5,
			status = $6,
			seo_title = $7,
			seo_description = $8,
			published_at = $9,
			updated_at = NOW()
		WHERE blog_post_id = $10 AND deleted_at IS NULL
		RETURNING updated_at
	`,
		post.Title,
		post.Slug,
		post.Excerpt,
		post.CoverImageURL,
		post.ContentHTML,
		post.Status,
		post.SEOTitle,
		post.SEODescription,
		post.PublishedAt,
		post.BlogPostID,
	).Scan(&post.UpdatedAt)
}

func (r *blogPostRepo) SoftDelete(ctx context.Context, id int64) error {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	_, err := r.db.Exec(ctx, `
		UPDATE blog_posts
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE blog_post_id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

func blogPostSelectSQL() string {
	return `
		SELECT blog_post_id, title, slug, excerpt, cover_image_url, content_html,
			status, seo_title, seo_description, published_at, created_at, updated_at
		FROM blog_posts
	`
}

func scanBlogPost(row pgx.Row) (*model.BlogPost, error) {
	var post model.BlogPost
	if err := row.Scan(
		&post.BlogPostID,
		&post.Title,
		&post.Slug,
		&post.Excerpt,
		&post.CoverImageURL,
		&post.ContentHTML,
		&post.Status,
		&post.SEOTitle,
		&post.SEODescription,
		&post.PublishedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &post, nil
}

func scanBlogPosts(rows pgx.Rows) ([]model.BlogPost, error) {
	posts := []model.BlogPost{}
	for rows.Next() {
		var post model.BlogPost
		if err := rows.Scan(
			&post.BlogPostID,
			&post.Title,
			&post.Slug,
			&post.Excerpt,
			&post.CoverImageURL,
			&post.ContentHTML,
			&post.Status,
			&post.SEOTitle,
			&post.SEODescription,
			&post.PublishedAt,
			&post.CreatedAt,
			&post.UpdatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, rows.Err()
}
