package model

import "time"

const (
	BlogPostStatusDraft     = "draft"
	BlogPostStatusPublished = "published"
)

type BlogPost struct {
	BlogPostID     int64      `json:"blog_post_id"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Excerpt        string     `json:"excerpt"`
	CoverImageURL  *string    `json:"cover_image_url,omitempty"`
	ContentHTML    string     `json:"content_html,omitempty"`
	Status         string     `json:"status,omitempty"`
	SEOTitle       *string    `json:"seo_title,omitempty"`
	SEODescription *string    `json:"seo_description,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
}

type CreateBlogPostRequest struct {
	Title          string  `json:"title"`
	Slug           string  `json:"slug,omitempty"`
	Excerpt        string  `json:"excerpt"`
	CoverImageURL  *string `json:"cover_image_url,omitempty"`
	ContentHTML    string  `json:"content_html"`
	Status         string  `json:"status"`
	SEOTitle       *string `json:"seo_title,omitempty"`
	SEODescription *string `json:"seo_description,omitempty"`
}

type UpdateBlogPostRequest struct {
	Title          *string `json:"title,omitempty"`
	Slug           *string `json:"slug,omitempty"`
	Excerpt        *string `json:"excerpt,omitempty"`
	CoverImageURL  *string `json:"cover_image_url,omitempty"`
	ContentHTML    *string `json:"content_html,omitempty"`
	Status         *string `json:"status,omitempty"`
	SEOTitle       *string `json:"seo_title,omitempty"`
	SEODescription *string `json:"seo_description,omitempty"`
}
