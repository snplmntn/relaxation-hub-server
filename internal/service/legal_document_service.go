package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/snplmntn/relaxation-hub-server/internal/model"
	"github.com/snplmntn/relaxation-hub-server/internal/repository"
)

var (
	ErrInvalidLegalDocumentKey     = errors.New("invalid legal document key")
	ErrLegalDocumentTitleRequired  = errors.New("title is required")
	ErrLegalDocumentContentMissing = errors.New("content is required")
)

type LegalDocumentService struct {
	repo repository.LegalDocumentRepository
}

func NewLegalDocumentService(repo repository.LegalDocumentRepository) *LegalDocumentService {
	return &LegalDocumentService{repo: repo}
}

func (s *LegalDocumentService) GetByLegacyKey(ctx context.Context, docKey string) (*model.LegalDocument, error) {
	key := strings.TrimSpace(docKey)
	switch key {
	case "privacy-policy":
		return s.GetByContentKey(ctx, model.LegalDocKeyPrivacyPolicy)
	case "terms-of-service":
		return s.GetByContentKey(ctx, model.LegalDocKeyTermsAndConditions)
	case "refund-policy":
		return s.GetByContentKey(ctx, model.LegalDocKeyRefundPolicy)
	case "about":
		doc, err := s.repo.GetByKey(ctx, "about")
		if err == nil {
			return doc, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.LegalDocument{
				DocKey:          "about",
				Title:           "About Relaxation Hub",
				ContentMarkdown: "<h1>About Relaxation Hub</h1><p>Relaxation Hub connects clients, therapists, and riders through dependable wellness services.</p>",
				Version:         "1.0.0",
				EffectiveAt:     time.Date(2026, time.February, 23, 0, 0, 0, 0, time.UTC),
				UpdatedAt:       time.Date(2026, time.February, 23, 0, 0, 0, 0, time.UTC),
			}, nil
		}
		return nil, err
	default:
		return nil, ErrInvalidLegalDocumentKey
	}
}

func (s *LegalDocumentService) GetByContentKey(ctx context.Context, docKey string) (*model.LegalDocument, error) {
	canonicalKey, err := normalizeContentKey(docKey)
	if err != nil {
		return nil, err
	}

	doc, err := s.repo.GetByKey(ctx, canonicalKey)
	if err == nil {
		return doc, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	return defaultContentDoc(canonicalKey), nil
}

func (s *LegalDocumentService) UpdateContentByKey(ctx context.Context, docKey, title, content string) (*model.LegalDocument, error) {
	canonicalKey, err := normalizeContentKey(docKey)
	if err != nil {
		return nil, err
	}

	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return nil, ErrLegalDocumentTitleRequired
	}
	if strings.TrimSpace(content) == "" {
		return nil, ErrLegalDocumentContentMissing
	}

	return s.repo.UpsertByKey(ctx, canonicalKey, trimmedTitle, content)
}

func normalizeContentKey(rawKey string) (string, error) {
	switch strings.TrimSpace(rawKey) {
	case model.LegalDocKeyTermsAndConditions:
		return model.LegalDocKeyTermsAndConditions, nil
	case model.LegalDocKeyPrivacyPolicy:
		return model.LegalDocKeyPrivacyPolicy, nil
	case model.LegalDocKeyRefundPolicy:
		return model.LegalDocKeyRefundPolicy, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidLegalDocumentKey, rawKey)
	}
}

func defaultContentDoc(key string) *model.LegalDocument {
	title := ""
	content := ""

	switch key {
	case model.LegalDocKeyTermsAndConditions:
		title = "Terms and Conditions"
		content = `<h1>Terms and Conditions</h1>
<p>By booking services with Relaxation Hub, you agree to these terms and conditions.</p>
<h2>1. Service Scope</h2>
<p>Services are fulfilled based on the booking details you provide. Please review your selections before confirming.</p>
<h2>2. Client Responsibilities</h2>
<p>Provide accurate location, contact, and special instructions to help ensure successful service delivery.</p>
<h2>3. Cancellations and No-Shows</h2>
<p>Late cancellations and no-shows may be subject to policy enforcement under platform rules.</p>
<h2>4. Liability</h2>
<p>Relaxation Hub is not liable for delays caused by force majeure events, traffic disruptions, or other events beyond reasonable control.</p>
<p>For support, contact us through the in-app support channels.</p>`
	case model.LegalDocKeyPrivacyPolicy:
		title = "Privacy Policy"
		content = `<h1>Privacy Policy</h1>
<p>Relaxation Hub values your privacy. We only collect information needed to operate and improve our services.</p>
<h2>1. Data We Collect</h2>
<p>We may collect account, booking, location, and device-related information required to provide service functionality.</p>
<h2>2. Data Usage</h2>
<p>Data is used for booking fulfillment, safety operations, communications, support, and service improvements.</p>
<h2>3. Data Protection</h2>
<p>We use reasonable security controls to protect your personal data and restrict unauthorized access.</p>
<p>For support, contact us through the in-app support channels.</p>`
	case model.LegalDocKeyRefundPolicy:
		title = "Refund Policy"
		content = `<h1>Refund Policy</h1>
<p>Relaxation Hub reviews refund requests fairly based on booking records and reported incidents.</p>
<h2>1. Request Window</h2>
<p>Please submit refund concerns as soon as possible after service completion or issue occurrence.</p>
<h2>2. Eligibility</h2>
<p>Approved refunds depend on verification, booking details, and policy compliance.</p>
<h2>3. Resolution</h2>
<p>Depending on the case, resolution may include partial refund, full refund, credit, or other remediation.</p>
<p>For support, contact us through the in-app support channels.</p>`
	}

	defaultTime := time.Date(2026, time.February, 23, 0, 0, 0, 0, time.UTC)
	return &model.LegalDocument{
		DocKey:          key,
		Title:           title,
		ContentMarkdown: content,
		Version:         "1.0.0",
		EffectiveAt:     defaultTime,
		UpdatedAt:       defaultTime,
	}
}
