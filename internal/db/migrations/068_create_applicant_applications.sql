-- Migration 068: Applicant applications for rider/therapist onboarding.

CREATE TABLE IF NOT EXISTS applicant_applications (
    application_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    target_role VARCHAR(32) NOT NULL,
    position_applied VARCHAR(128) NOT NULL,
    preferred_branch_id BIGINT NOT NULL REFERENCES branches(branch_id) ON DELETE RESTRICT,
    preferred_branch_label VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    answers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    attachments_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by_admin_id BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    review_notes TEXT,
    CONSTRAINT chk_applicant_applications_target_role CHECK (target_role IN ('therapist', 'rider')),
    CONSTRAINT chk_applicant_applications_status CHECK (status IN ('pending', 'approved', 'rejected', 'needs_followup'))
);

CREATE INDEX IF NOT EXISTS idx_applicant_applications_status_submitted
    ON applicant_applications (status, submitted_at DESC);

CREATE INDEX IF NOT EXISTS idx_applicant_applications_role_status
    ON applicant_applications (target_role, status);

CREATE INDEX IF NOT EXISTS idx_applicant_applications_user_id
    ON applicant_applications (user_id);
