package model

import "time"

const (
	LegalDocKeyTermsAndConditions = "terms_and_conditions"
	LegalDocKeyPrivacyPolicy      = "privacy_policy"
	LegalDocKeyRefundPolicy       = "refund_policy"
)

// LegalDocument represents a public legal content document.
type LegalDocument struct {
	DocKey          string    `json:"doc_key"`
	Title           string    `json:"title"`
	ContentMarkdown string    `json:"content_markdown"`
	Version         string    `json:"version"`
	EffectiveAt     time.Time `json:"effective_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UpdateLegalDocumentRequest is the payload used for legal content updates.
type UpdateLegalDocumentRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}
