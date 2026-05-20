CREATE TABLE IF NOT EXISTS blog_posts (
    blog_post_id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    slug TEXT NOT NULL,
    excerpt TEXT NOT NULL DEFAULT '',
    cover_image_url TEXT,
    content_html TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    seo_title TEXT,
    seo_description TEXT,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_blog_posts_slug_active
    ON blog_posts (slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_blog_posts_public
    ON blog_posts (published_at DESC, blog_post_id DESC)
    WHERE status = 'published' AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_blog_posts_admin
    ON blog_posts (updated_at DESC, blog_post_id DESC)
    WHERE deleted_at IS NULL;
