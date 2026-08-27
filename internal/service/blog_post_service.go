package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

var (
	ErrBlogPostNotFound        = errors.New("blog post not found")
	ErrBlogPostTitleRequired   = errors.New("title is required")
	ErrBlogPostContentRequired = errors.New("content is required")
	ErrBlogPostInvalidContent  = errors.New("content contains no supported text or images")
	ErrBlogPostInvalidStatus   = errors.New("invalid blog status")
	ErrBlogPostDuplicateSlug   = errors.New("slug is already in use")
)

type BlogPostService struct {
	repo repository.BlogPostRepository
	now  func() time.Time
}

func NewBlogPostService(repo repository.BlogPostRepository) *BlogPostService {
	return &BlogPostService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *BlogPostService) Create(ctx context.Context, req *model.CreateBlogPostRequest) (*model.BlogPost, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrBlogPostTitleRequired
	}
	content, err := sanitizeBlogHTML(strings.TrimSpace(req.ContentHTML))
	if err != nil || !hasMeaningfulBlogContent(content) {
		return nil, ErrBlogPostContentRequired
	}
	status, err := normalizeBlogStatus(req.Status)
	if err != nil {
		return nil, err
	}

	slug, err := s.resolveSlug(ctx, req.Slug, title, nil)
	if err != nil {
		return nil, err
	}

	post := &model.BlogPost{
		Title:          title,
		Slug:           slug,
		Excerpt:        strings.TrimSpace(req.Excerpt),
		CoverImageURL:  optionalTrimmedString(req.CoverImageURL),
		ContentHTML:    content,
		Status:         status,
		SEOTitle:       optionalTrimmedString(req.SEOTitle),
		SEODescription: optionalTrimmedString(req.SEODescription),
	}
	s.applyPublishState(post)

	if err := s.repo.Create(ctx, post); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrBlogPostDuplicateSlug
		}
		return nil, err
	}
	return post, nil
}

func (s *BlogPostService) Update(ctx context.Context, id int64, req *model.UpdateBlogPostRequest) (*model.BlogPost, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}

	post, err := s.repo.GetAdminByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBlogPostNotFound
		}
		return nil, err
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrBlogPostTitleRequired
		}
		post.Title = title
	}
	if req.ContentHTML != nil {
		content, sanitizeErr := sanitizeBlogHTML(strings.TrimSpace(*req.ContentHTML))
		if sanitizeErr != nil || !hasMeaningfulBlogContent(content) {
			return nil, ErrBlogPostInvalidContent
		}
		post.ContentHTML = content
	}
	if req.Excerpt != nil {
		post.Excerpt = strings.TrimSpace(*req.Excerpt)
	}
	if req.CoverImageURL != nil {
		post.CoverImageURL = optionalTrimmedString(req.CoverImageURL)
	}
	if req.SEOTitle != nil {
		post.SEOTitle = optionalTrimmedString(req.SEOTitle)
	}
	if req.SEODescription != nil {
		post.SEODescription = optionalTrimmedString(req.SEODescription)
	}
	if req.Status != nil {
		status, err := normalizeBlogStatus(*req.Status)
		if err != nil {
			return nil, err
		}
		post.Status = status
	}
	if req.Slug != nil {
		slug, err := s.resolveSlug(ctx, *req.Slug, post.Title, &id)
		if err != nil {
			return nil, err
		}
		post.Slug = slug
	}

	s.applyPublishState(post)

	if err := s.repo.Update(ctx, post); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBlogPostNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrBlogPostDuplicateSlug
		}
		return nil, err
	}
	return post, nil
}

func (s *BlogPostService) ListAdmin(ctx context.Context, status, q string, limit, offset int) ([]model.BlogPost, error) {
	normalizedStatus := strings.TrimSpace(status)
	if normalizedStatus != "" {
		var err error
		normalizedStatus, err = normalizeBlogStatus(normalizedStatus)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.ListAdmin(ctx, normalizedStatus, strings.TrimSpace(q), clampLimit(limit), maxInt(offset, 0))
}

func (s *BlogPostService) GetAdminByID(ctx context.Context, id int64) (*model.BlogPost, error) {
	post, err := s.repo.GetAdminByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBlogPostNotFound
		}
		return nil, err
	}
	return post, nil
}

func (s *BlogPostService) ListPublished(ctx context.Context, limit, offset int) ([]model.BlogPost, error) {
	return s.repo.ListPublished(ctx, clampLimit(limit), maxInt(offset, 0))
}

func (s *BlogPostService) GetPublishedBySlug(ctx context.Context, slug string) (*model.BlogPost, error) {
	post, err := s.repo.GetBySlug(ctx, strings.TrimSpace(slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBlogPostNotFound
		}
		return nil, err
	}
	return post, nil
}

func (s *BlogPostService) Delete(ctx context.Context, id int64) error {
	return s.repo.SoftDelete(ctx, id)
}

func (s *BlogPostService) resolveSlug(ctx context.Context, rawSlug, title string, excludeID *int64) (string, error) {
	base := slugify(rawSlug)
	explicit := strings.TrimSpace(rawSlug) != ""
	if base == "" {
		base = slugify(title)
	}
	if base == "" {
		base = "post"
	}

	exists, err := s.repo.SlugExists(ctx, base, excludeID)
	if err != nil {
		return "", err
	}
	if !exists {
		return base, nil
	}
	if explicit {
		return "", ErrBlogPostDuplicateSlug
	}

	for i := 2; i <= 100; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		exists, err := s.repo.SlugExists(ctx, candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not generate unique slug")
}

func (s *BlogPostService) applyPublishState(post *model.BlogPost) {
	if post.Status == model.BlogPostStatusPublished {
		if post.PublishedAt == nil {
			now := s.now()
			post.PublishedAt = &now
		}
		return
	}
	post.PublishedAt = nil
}

func normalizeBlogStatus(status string) (string, error) {
	switch strings.TrimSpace(status) {
	case "", model.BlogPostStatusDraft:
		return model.BlogPostStatusDraft, nil
	case model.BlogPostStatusPublished:
		return model.BlogPostStatusPublished, nil
	default:
		return "", ErrBlogPostInvalidStatus
	}
}

func optionalTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)
var slugDashes = regexp.MustCompile(`-+`)

func slugify(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = nonSlugChars.ReplaceAllString(slug, "-")
	slug = slugDashes.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func maxInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505")
}
