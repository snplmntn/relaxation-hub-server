package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// StorageService defines the interface for file storage operations.
// This abstraction allows for easy swapping of storage backends (S3, GCS, local, etc.)
// and simplifies testing through mocking.
type StorageService interface {
	// UploadFile uploads a file to the storage backend.
	// Returns the publicly accessible URL of the uploaded file.
	UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (string, error)

	// GetFileURL returns the publicly accessible URL for a given key.
	GetFileURL(key string) string

	// GetPresignedURL returns a pre-signed URL with temporary access for private files.
	GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)

	// DeleteFile removes a file from the storage backend.
	DeleteFile(ctx context.Context, key string) error

	// GenerateKey creates a unique storage key for a file.
	GenerateKey(prefix, filename string) string

	// IsConfigured returns true if the storage service is properly configured.
	IsConfigured() bool
}

// S3StorageService implements StorageService using AWS S3.
type S3StorageService struct {
	client     *s3.Client
	bucket     string
	region     string
	baseURL    string
	configured bool
}

// S3Config holds configuration for the S3 storage service.
type S3Config struct {
	Bucket  string
	Region  string
	BaseURL string // Optional: custom base URL (e.g., CloudFront distribution)
}

// NewS3StorageService creates a new S3-backed storage service.
// Returns a no-op service if configuration is incomplete.
func NewS3StorageService(ctx context.Context, cfg S3Config) *S3StorageService {
	svc := &S3StorageService{
		bucket:     cfg.Bucket,
		region:     cfg.Region,
		baseURL:    cfg.BaseURL,
		configured: false,
	}

	// Validate required configuration
	if cfg.Bucket == "" || cfg.Region == "" {
		slog.Warn("S3StorageService: incomplete configuration - storage disabled", "bucket", cfg.Bucket, "region", cfg.Region)
		return svc
	}

	// Load AWS configuration using default credential chain
	// This supports: env vars, shared credentials, IAM roles, etc.
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		slog.Warn("S3StorageService: failed to load AWS config - storage disabled", "error", err)
		return svc
	}

	svc.client = s3.NewFromConfig(awsCfg)
	svc.configured = true

	// Set base URL for public access
	if svc.baseURL == "" {
		svc.baseURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", cfg.Bucket, cfg.Region)
	}

	slog.Info("S3StorageService: initialized", "bucket", cfg.Bucket, "region", cfg.Region)
	return svc
}

// UploadFile uploads a file to S3 and returns the public URL.
func (s *S3StorageService) UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	if !s.configured {
		return "", fmt.Errorf("S3 storage not configured")
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}

	// Set content type if provided
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return s.GetFileURL(key), nil
}

// GetFileURL returns the public URL for a stored file.
func (s *S3StorageService) GetFileURL(key string) string {
	if s.baseURL == "" {
		return ""
	}
	// Ensure key doesn't have leading slash for proper URL construction
	key = strings.TrimPrefix(key, "/")
	return fmt.Sprintf("%s/%s", s.baseURL, key)
}

// GetPresignedURL returns a pre-signed URL with temporary access for private files.
func (s *S3StorageService) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if !s.configured {
		return "", fmt.Errorf("S3 storage not configured")
	}

	key = strings.TrimPrefix(key, "/")

	// Create a presign client
	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String("inline"),
	}, s3.WithPresignExpires(expiry))

	if err != nil {
		return "", fmt.Errorf("failed to presign URL: %w", err)
	}

	return request.URL, nil
}

// DeleteFile removes a file from S3.
func (s *S3StorageService) DeleteFile(ctx context.Context, key string) error {
	if !s.configured {
		return fmt.Errorf("S3 storage not configured")
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GenerateKey creates a unique storage key with timestamp and sanitized filename.
func (s *S3StorageService) GenerateKey(prefix, filename string) string {
	// Sanitize filename: keep only basename, replace spaces
	base := filepath.Base(filename)
	base = strings.ReplaceAll(base, " ", "_")

	// Extract extension
	ext := filepath.Ext(base)
	if ext == "" {
		ext = ".bin"
	}
	name := strings.TrimSuffix(base, ext)

	// Create unique key with timestamp
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s/%s_%d%s", strings.Trim(prefix, "/"), name, timestamp, ext)
}

// IsConfigured returns true if the service is ready for use.
func (s *S3StorageService) IsConfigured() bool {
	return s.configured
}

// NoOpStorageService is a storage service that does nothing.
// Used as a fallback when storage is not configured.
type NoOpStorageService struct{}

// NewNoOpStorageService creates a no-op storage service.
func NewNoOpStorageService() *NoOpStorageService {
	return &NoOpStorageService{}
}

func (n *NoOpStorageService) UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	return "", fmt.Errorf("storage not configured")
}

func (n *NoOpStorageService) GetFileURL(key string) string {
	return ""
}

func (n *NoOpStorageService) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "", fmt.Errorf("storage not configured")
}

func (n *NoOpStorageService) DeleteFile(ctx context.Context, key string) error {
	return fmt.Errorf("storage not configured")
}

func (n *NoOpStorageService) GenerateKey(prefix, filename string) string {
	return ""
}

func (n *NoOpStorageService) IsConfigured() bool {
	return false
}
