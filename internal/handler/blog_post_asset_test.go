package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/service"
)

type blogAssetStorage struct {
	uploadedKey string
	signedKey   string
	deletedKeys []string
}

func (s *blogAssetStorage) UploadFile(_ context.Context, key string, _ io.Reader, _ string) (string, error) {
	s.uploadedKey = key
	return "https://private-bucket.example/" + key, nil
}

func (s *blogAssetStorage) GetFileURL(key string) string {
	return "https://private-bucket.example/" + key
}

func (s *blogAssetStorage) GetPresignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	s.signedKey = key
	return "https://signed.example/image?signature=test", nil
}

func (s *blogAssetStorage) DeleteFile(_ context.Context, key string) error {
	s.deletedKeys = append(s.deletedKeys, key)
	return nil
}

func (s *blogAssetStorage) GenerateKey(prefix, filename string) string {
	return prefix + "/" + filename
}

func (s *blogAssetStorage) IsConfigured() bool {
	return true
}

func TestBlogPostUploadReturnsStableAssetURL(t *testing.T) {
	storage := &blogAssetStorage{}
	handler := NewBlogPostHandler(nil, storage)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("image", "story.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(testPNGBytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/blog-posts/upload-cover", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "api.example.com")
	response := httptest.NewRecorder()

	handler.UploadCover(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", response.Code, response.Body.String())
	}

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.URL != "https://api.example.com/api/v1/blog-assets/story.png" {
		t.Fatalf("unexpected asset URL: %s", payload.URL)
	}
	if storage.uploadedKey != "blog-posts/story.png" {
		t.Fatalf("unexpected uploaded key: %s", storage.uploadedKey)
	}
}

func TestBlogPostAssetRedirectsToFreshSignedURL(t *testing.T) {
	storage := &blogAssetStorage{}
	handler := NewBlogPostHandler(nil, storage)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/blog-assets/story.jpg", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("filename", "story.jpg")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()

	handler.GetAsset(response, request)

	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d: %s", response.Code, response.Body.String())
	}
	if location := response.Header().Get("Location"); !strings.HasPrefix(location, "https://signed.example/image") {
		t.Fatalf("unexpected redirect location: %s", location)
	}
	if storage.signedKey != "blog-posts/story.jpg" {
		t.Fatalf("unexpected signed key: %s", storage.signedKey)
	}
}

func TestBlogPostUploadRejectsUnsupportedContent(t *testing.T) {
	storage := &blogAssetStorage{}
	handler := NewBlogPostHandler(nil, storage)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("image", "not-an-image.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("<html>not an image</html>")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/blog-posts/upload-cover", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	handler.UploadCover(response, request)

	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected status 415, got %d: %s", response.Code, response.Body.String())
	}
	if storage.uploadedKey != "" {
		t.Fatalf("unsupported content should not be uploaded, got %s", storage.uploadedKey)
	}
}

func TestBlogPostUploadRejectsOversizedImage(t *testing.T) {
	storage := &blogAssetStorage{}
	handler := NewBlogPostHandler(nil, storage)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("image", "large.png")
	if err != nil {
		t.Fatal(err)
	}
	oversized := append(testPNGBytes(), bytes.Repeat([]byte{0}, int(maxBlogImageBytes))...)
	if _, err := file.Write(oversized); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/blog-posts/upload-cover", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	handler.UploadCover(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d: %s", response.Code, response.Body.String())
	}
	if storage.uploadedKey != "" {
		t.Fatalf("oversized image should not be uploaded, got %s", storage.uploadedKey)
	}
}

func TestBlogPostDeleteAssetRemovesOnlyBlogImage(t *testing.T) {
	storage := &blogAssetStorage{}
	handler := NewBlogPostHandler(nil, storage)

	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/blog-posts/assets/story.png", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("filename", "story.png")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()

	handler.DeleteAsset(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", response.Code, response.Body.String())
	}
	if len(storage.deletedKeys) != 1 || storage.deletedKeys[0] != "blog-posts/story.png" {
		t.Fatalf("unexpected deleted keys: %#v", storage.deletedKeys)
	}
}

func TestBlogAssetKeysFindsStableAndLegacyURLs(t *testing.T) {
	cover := "https://api.example.com/api/v1/blog-assets/cover.jpg"
	post := &model.BlogPost{
		CoverImageURL: &cover,
		ContentHTML: `<p>Body</p>` +
			`<img src="https://api.example.com/api/v1/blog-assets/inline.png">` +
			`<img src="https://bucket.s3.example.com/blog-posts/legacy.webp">`,
	}

	keys := blogAssetKeys(post)
	for _, key := range []string{
		"blog-posts/cover.jpg",
		"blog-posts/inline.png",
		"blog-posts/legacy.webp",
	} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("expected key %s in %#v", key, keys)
		}
	}
}

type failingBlogPostRepo struct{}

func (f *failingBlogPostRepo) Create(_ context.Context, _ *model.BlogPost) error {
	return errors.New("database unavailable")
}

func (f *failingBlogPostRepo) GetAdminByID(_ context.Context, _ int64) (*model.BlogPost, error) {
	return nil, errors.New("not implemented")
}

func (f *failingBlogPostRepo) GetBySlug(_ context.Context, _ string) (*model.BlogPost, error) {
	return nil, errors.New("not implemented")
}

func (f *failingBlogPostRepo) SlugExists(_ context.Context, _ string, _ *int64) (bool, error) {
	return false, nil
}

func (f *failingBlogPostRepo) ListAdmin(_ context.Context, _, _ string, _, _ int) ([]model.BlogPost, error) {
	return nil, nil
}

func (f *failingBlogPostRepo) ListPublished(_ context.Context, _, _ int) ([]model.BlogPost, error) {
	return nil, nil
}

func (f *failingBlogPostRepo) Update(_ context.Context, _ *model.BlogPost) error {
	return errors.New("not implemented")
}

func (f *failingBlogPostRepo) SoftDelete(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func TestBlogPostCreateFailureCleansUploadedAssets(t *testing.T) {
	storage := &blogAssetStorage{}
	blogService := service.NewBlogPostService(&failingBlogPostRepo{})
	handler := NewBlogPostHandler(blogService, storage)
	body := strings.NewReader(`{
		"title":"Story",
		"content_html":"<p>Body</p>",
		"status":"published",
		"uploaded_asset_urls":[
			"https://api.example.com/api/v1/blog-assets/cover.png",
			"https://api.example.com/api/v1/blog-assets/inline.jpg"
		]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/blog-posts", body)
	response := httptest.NewRecorder()

	handler.Create(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", response.Code, response.Body.String())
	}
	if len(storage.deletedKeys) != 2 {
		t.Fatalf("expected two cleanup calls, got %#v", storage.deletedKeys)
	}
}

type lifecycleBlogPostRepo struct {
	post *model.BlogPost
}

func (r *lifecycleBlogPostRepo) Create(_ context.Context, post *model.BlogPost) error {
	copy := *post
	r.post = &copy
	return nil
}

func (r *lifecycleBlogPostRepo) GetAdminByID(_ context.Context, _ int64) (*model.BlogPost, error) {
	copy := *r.post
	return &copy, nil
}

func (r *lifecycleBlogPostRepo) GetBySlug(_ context.Context, _ string) (*model.BlogPost, error) {
	copy := *r.post
	return &copy, nil
}

func (r *lifecycleBlogPostRepo) SlugExists(_ context.Context, _ string, _ *int64) (bool, error) {
	return false, nil
}

func (r *lifecycleBlogPostRepo) ListAdmin(_ context.Context, _, _ string, _, _ int) ([]model.BlogPost, error) {
	return []model.BlogPost{*r.post}, nil
}

func (r *lifecycleBlogPostRepo) ListPublished(_ context.Context, _, _ int) ([]model.BlogPost, error) {
	return []model.BlogPost{*r.post}, nil
}

func (r *lifecycleBlogPostRepo) Update(_ context.Context, post *model.BlogPost) error {
	copy := *post
	r.post = &copy
	return nil
}

func (r *lifecycleBlogPostRepo) SoftDelete(_ context.Context, _ int64) error {
	return nil
}

func TestBlogPostUpdateDeletesOnlyRemovedAssets(t *testing.T) {
	oldCover := "https://api.example.com/api/v1/blog-assets/old-cover.jpg"
	repo := &lifecycleBlogPostRepo{post: &model.BlogPost{
		BlogPostID:    7,
		Title:         "Story",
		Slug:          "story",
		CoverImageURL: &oldCover,
		ContentHTML: `<img src="https://api.example.com/api/v1/blog-assets/keep.png">` +
			`<img src="https://api.example.com/api/v1/blog-assets/remove.png">`,
		Status: model.BlogPostStatusDraft,
	}}
	storage := &blogAssetStorage{}
	handler := NewBlogPostHandler(service.NewBlogPostService(repo), storage)
	body := strings.NewReader(`{
		"cover_image_url":"https://api.example.com/api/v1/blog-assets/new-cover.jpg",
		"content_html":"<p>Updated</p><img src=\"https://api.example.com/api/v1/blog-assets/keep.png\">"
	}`)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/blog-posts/7", body)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "7")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()

	handler.Update(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	deleted := make(map[string]bool)
	for _, key := range storage.deletedKeys {
		deleted[key] = true
	}
	if !deleted["blog-posts/old-cover.jpg"] || !deleted["blog-posts/remove.png"] {
		t.Fatalf("expected removed assets to be deleted, got %#v", storage.deletedKeys)
	}
	if deleted["blog-posts/keep.png"] || deleted["blog-posts/new-cover.jpg"] {
		t.Fatalf("active assets must not be deleted, got %#v", storage.deletedKeys)
	}
}

func testPNGBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}
}
