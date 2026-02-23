package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

type mockLegalDocRepo struct {
	getByKeyFn    func(ctx context.Context, docKey string) (*model.LegalDocument, error)
	upsertByKeyFn func(ctx context.Context, docKey, title, content string) (*model.LegalDocument, error)
}

func (m *mockLegalDocRepo) GetByKey(ctx context.Context, docKey string) (*model.LegalDocument, error) {
	if m.getByKeyFn != nil {
		return m.getByKeyFn(ctx, docKey)
	}
	return nil, pgx.ErrNoRows
}

func (m *mockLegalDocRepo) UpsertByKey(ctx context.Context, docKey, title, content string) (*model.LegalDocument, error) {
	if m.upsertByKeyFn != nil {
		return m.upsertByKeyFn(ctx, docKey, title, content)
	}
	return &model.LegalDocument{
		DocKey:          docKey,
		Title:           title,
		ContentMarkdown: content,
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

func TestLegalDocumentServiceGetByContentKeyInvalidKey(t *testing.T) {
	svc := NewLegalDocumentService(&mockLegalDocRepo{})

	_, err := svc.GetByContentKey(context.Background(), "invalid")
	if !errors.Is(err, ErrInvalidLegalDocumentKey) {
		t.Fatalf("expected invalid key error, got %v", err)
	}
}

func TestLegalDocumentServiceGetByContentKeyFallbackWhenMissing(t *testing.T) {
	svc := NewLegalDocumentService(&mockLegalDocRepo{
		getByKeyFn: func(ctx context.Context, docKey string) (*model.LegalDocument, error) {
			return nil, pgx.ErrNoRows
		},
	})

	doc, err := svc.GetByContentKey(context.Background(), model.LegalDocKeyPrivacyPolicy)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if doc.DocKey != model.LegalDocKeyPrivacyPolicy {
		t.Fatalf("expected key %s, got %s", model.LegalDocKeyPrivacyPolicy, doc.DocKey)
	}
	if doc.Title == "" || doc.ContentMarkdown == "" {
		t.Fatalf("expected fallback content, got %+v", doc)
	}
}

func TestLegalDocumentServiceUpdateContentByKeyValidation(t *testing.T) {
	svc := NewLegalDocumentService(&mockLegalDocRepo{})

	_, err := svc.UpdateContentByKey(context.Background(), model.LegalDocKeyTermsAndConditions, " ", "<p>x</p>")
	if !errors.Is(err, ErrLegalDocumentTitleRequired) {
		t.Fatalf("expected title required error, got %v", err)
	}

	_, err = svc.UpdateContentByKey(context.Background(), model.LegalDocKeyTermsAndConditions, "Terms", " ")
	if !errors.Is(err, ErrLegalDocumentContentMissing) {
		t.Fatalf("expected content required error, got %v", err)
	}
}

func TestLegalDocumentServiceUpdateContentByKeySuccess(t *testing.T) {
	svc := NewLegalDocumentService(&mockLegalDocRepo{
		upsertByKeyFn: func(ctx context.Context, docKey, title, content string) (*model.LegalDocument, error) {
			return &model.LegalDocument{
				DocKey:          docKey,
				Title:           title,
				ContentMarkdown: content,
				UpdatedAt:       time.Now().UTC(),
			}, nil
		},
	})

	doc, err := svc.UpdateContentByKey(context.Background(), model.LegalDocKeyRefundPolicy, "Refund Policy", "<p>Policy text</p>")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if doc.DocKey != model.LegalDocKeyRefundPolicy {
		t.Fatalf("expected key %s, got %s", model.LegalDocKeyRefundPolicy, doc.DocKey)
	}
}
