package service

import (
	"context"
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type mockBlogPostRepo struct {
	existingSlugs map[string]bool
	created       *model.BlogPost
	updateTarget  *model.BlogPost
	updated       *model.BlogPost
}

func (m *mockBlogPostRepo) Create(_ context.Context, post *model.BlogPost) error {
	post.BlogPostID = 1
	m.created = post
	return nil
}

func (m *mockBlogPostRepo) GetAdminByID(_ context.Context, _ int64) (*model.BlogPost, error) {
	return m.updateTarget, nil
}

func (m *mockBlogPostRepo) GetBySlug(_ context.Context, _ string) (*model.BlogPost, error) {
	return nil, nil
}

func (m *mockBlogPostRepo) SlugExists(_ context.Context, slug string, _ *int64) (bool, error) {
	return m.existingSlugs[slug], nil
}

func (m *mockBlogPostRepo) ListAdmin(_ context.Context, _, _ string, _, _ int) ([]model.BlogPost, error) {
	return nil, nil
}

func (m *mockBlogPostRepo) ListPublished(_ context.Context, _, _ int) ([]model.BlogPost, error) {
	return nil, nil
}

func (m *mockBlogPostRepo) Update(_ context.Context, post *model.BlogPost) error {
	m.updated = post
	return nil
}

func (m *mockBlogPostRepo) SoftDelete(_ context.Context, _ int64) error {
	return nil
}

func TestBlogPostServiceCreateGeneratesUniqueSlug(t *testing.T) {
	repo := &mockBlogPostRepo{
		existingSlugs: map[string]bool{
			"rest-at-home": true,
		},
	}
	svc := NewBlogPostService(repo)
	svc.now = func() time.Time { return time.Date(2026, 5, 19, 1, 0, 0, 0, time.UTC) }

	post, err := svc.Create(context.Background(), &model.CreateBlogPostRequest{
		Title:       "Rest at Home",
		Excerpt:     "Simple ways to recover after a long week.",
		ContentHTML: "<p>Body</p>",
		Status:      model.BlogPostStatusPublished,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if post.Slug != "rest-at-home-2" {
		t.Fatalf("expected unique slug, got %q", post.Slug)
	}
	if post.PublishedAt == nil {
		t.Fatal("expected published_at to be set for published post")
	}
}

func TestBlogPostServiceUpdateDraftClearsPublishedAt(t *testing.T) {
	publishedAt := time.Date(2026, 5, 19, 1, 0, 0, 0, time.UTC)
	repo := &mockBlogPostRepo{
		existingSlugs: map[string]bool{},
		updateTarget: &model.BlogPost{
			BlogPostID:  7,
			Title:       "Existing Post",
			Slug:        "existing-post",
			Excerpt:     "Excerpt",
			ContentHTML: "<p>Body</p>",
			Status:      model.BlogPostStatusPublished,
			PublishedAt: &publishedAt,
		},
	}
	svc := NewBlogPostService(repo)
	status := model.BlogPostStatusDraft

	post, err := svc.Update(context.Background(), 7, &model.UpdateBlogPostRequest{
		Status: &status,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if post.Status != model.BlogPostStatusDraft {
		t.Fatalf("expected draft status, got %q", post.Status)
	}
	if post.PublishedAt != nil {
		t.Fatalf("expected published_at to be cleared, got %v", post.PublishedAt)
	}
}
