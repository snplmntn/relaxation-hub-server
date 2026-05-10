package repository

import (
	"context"

	"github.com/snplmntn/relaxation-hub-server/internal/db"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

// LegalDocumentRepository defines legal document read operations.
type LegalDocumentRepository interface {
	GetByKey(ctx context.Context, docKey string) (*model.LegalDocument, error)
	UpsertByKey(ctx context.Context, docKey, title, content string) (*model.LegalDocument, error)
}

type legalDocumentRepo struct {
	db db.DBTX
}

func NewLegalDocumentRepository(db db.DBTX) LegalDocumentRepository {
	return &legalDocumentRepo{db: db}
}

func (r *legalDocumentRepo) GetByKey(ctx context.Context, docKey string) (*model.LegalDocument, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var doc model.LegalDocument
	err := r.db.QueryRow(ctx, `
		SELECT doc_key, title, content_markdown, version, effective_at, updated_at
		FROM legal_documents
		WHERE doc_key = $1
		LIMIT 1
	`, docKey).Scan(
		&doc.DocKey,
		&doc.Title,
		&doc.ContentMarkdown,
		&doc.Version,
		&doc.EffectiveAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *legalDocumentRepo) UpsertByKey(ctx context.Context, docKey, title, content string) (*model.LegalDocument, error) {
	ctx, cancel := db.WithQueryTimeout(ctx)
	defer cancel()

	var doc model.LegalDocument
	err := r.db.QueryRow(ctx, `
		INSERT INTO legal_documents (doc_key, title, content_markdown, version, effective_at, updated_at)
		VALUES ($1, $2, $3, '1.0.0', NOW(), NOW())
		ON CONFLICT (doc_key) DO UPDATE
		SET
			title = EXCLUDED.title,
			content_markdown = EXCLUDED.content_markdown,
			updated_at = NOW()
		RETURNING doc_key, title, content_markdown, version, effective_at, updated_at
	`, docKey, title, content).Scan(
		&doc.DocKey,
		&doc.Title,
		&doc.ContentMarkdown,
		&doc.Version,
		&doc.EffectiveAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &doc, nil
}
