-- Migration 064: Create legal_documents table and seed default legal content

CREATE TABLE IF NOT EXISTS legal_documents (
    doc_key VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content_markdown TEXT NOT NULL,
    version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO legal_documents (doc_key, title, content_markdown, version, effective_at, updated_at)
VALUES
(
    'privacy-policy',
    'Privacy Policy',
    $$# Privacy Policy

This Privacy Policy explains how Relaxation Hub collects, uses, and protects personal information.

## Information We Collect
- Account information (name, email, phone, and profile details)
- Location information required for rider operations
- Device and notification token data

## How We Use Data
- Deliver core platform functionality
- Improve reliability, safety, and support workflows
- Send service-related communications

## Contact
For privacy concerns, please contact support through the app.$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'terms-of-service',
    'Terms of Service',
    $$# Terms of Service

By using Relaxation Hub, you agree to comply with platform policies and applicable laws.

## Rider Responsibilities
- Keep account information accurate
- Follow ride and safety procedures
- Avoid fraudulent or abusive behavior

## Enforcement
Accounts may be restricted for policy violations.

## Changes
These terms may be updated from time to time.$$,
    '1.0.0',
    NOW(),
    NOW()
),
(
    'about',
    'About Relaxation Hub',
    $$# About Relaxation Hub

Relaxation Hub connects clients, therapists, and riders through a coordinated service platform.

## Mission
Deliver dependable, safe, and high-quality wellness support.

## Support
Need help? Open a support ticket inside the app.$$,
    '1.0.0',
    NOW(),
    NOW()
)
ON CONFLICT (doc_key) DO UPDATE
SET
    title = EXCLUDED.title,
    content_markdown = EXCLUDED.content_markdown,
    version = EXCLUDED.version,
    effective_at = EXCLUDED.effective_at,
    updated_at = NOW();
