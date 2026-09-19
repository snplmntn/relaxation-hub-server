-- ============================================================================
-- 010. Replay consolidated migration changes (002-009) for migrated environments.
-- ============================================================================
-- These operations were previously in removed migration files. This replay migration
-- ensures environments that already tracked old migration filenames continue to
-- receive these schema updates safely.

-- ============================================================================
-- Consolidated from 002_daily_sales_salary_reports.sql
-- ============================================================================

CREATE TABLE IF NOT EXISTS daily_sales_remittances (
    remittance_id SERIAL PRIMARY KEY,
    business_date DATE NOT NULL,
    branch_id INT NOT NULL REFERENCES branches(branch_id) ON DELETE RESTRICT,
    bill_1000 INT NOT NULL DEFAULT 0 CHECK (bill_1000 >= 0),
    bill_500 INT NOT NULL DEFAULT 0 CHECK (bill_500 >= 0),
    bill_200 INT NOT NULL DEFAULT 0 CHECK (bill_200 >= 0),
    bill_100 INT NOT NULL DEFAULT 0 CHECK (bill_100 >= 0),
    bill_50 INT NOT NULL DEFAULT 0 CHECK (bill_50 >= 0),
    bill_20 INT NOT NULL DEFAULT 0 CHECK (bill_20 >= 0),
    bill_10 INT NOT NULL DEFAULT 0 CHECK (bill_10 >= 0),
    bill_5 INT NOT NULL DEFAULT 0 CHECK (bill_5 >= 0),
    bill_1 INT NOT NULL DEFAULT 0 CHECK (bill_1 >= 0),
    actual_remitted NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (actual_remitted >= 0),
    tips_total NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (tips_total >= 0),
    client_funds_used NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (client_funds_used >= 0),
    client_funds_added NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (client_funds_added >= 0),
    remitted_to_mark NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (remitted_to_mark >= 0),
    other_remitted_amount NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (other_remitted_amount >= 0),
    remitted_to TEXT,
    others_deducted NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (others_deducted >= 0),
    others_added NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (others_added >= 0),
    notes TEXT,
    created_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (business_date, branch_id)
);

CREATE INDEX IF NOT EXISTS idx_daily_sales_remittances_business_date ON daily_sales_remittances(business_date);
CREATE INDEX IF NOT EXISTS idx_daily_sales_remittances_branch_date ON daily_sales_remittances(branch_id, business_date);

CREATE TABLE IF NOT EXISTS therapist_payroll_adjustments (
    adjustment_id SERIAL PRIMARY KEY,
    therapist_id INT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    adjustment_date DATE NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('add', 'minus')),
    category VARCHAR(30) NOT NULL CHECK (category IN ('benefits', 'cash_advance', 'salary', 'correction', 'parcel', 'absence', 'other')),
    amount NUMERIC(10,2) NOT NULL CHECK (amount >= 0),
    reason TEXT NOT NULL,
    cash_movement NUMERIC(10,2) NOT NULL DEFAULT 0,
    created_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_by INT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CHECK (period_end >= period_start)
);

CREATE INDEX IF NOT EXISTS idx_therapist_payroll_adjustments_period ON therapist_payroll_adjustments(period_start, period_end) WHERE voided_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_therapist_payroll_adjustments_therapist ON therapist_payroll_adjustments(therapist_id) WHERE voided_at IS NULL;

-- ============================================================================
-- Consolidated from 003_add_load_reduction_indexes.sql
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_booking_assignment_queue_due_order
    ON booking_assignment_queue (next_attempt_at, enqueued_at, booking_id);

CREATE INDEX IF NOT EXISTS idx_bookings_in_progress_actual_start_due
    ON bookings (actual_start, booking_id)
    WHERE status = 'in_progress' AND actual_start IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_bookings_assigned_scheduled_due
    ON bookings (scheduled_start, booking_id)
    WHERE status = 'assigned';

CREATE INDEX IF NOT EXISTS idx_booking_events_booking_type
    ON booking_events (booking_id, event_type);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created_keyset
    ON notifications (user_id, created_at DESC, notification_id DESC);

CREATE INDEX IF NOT EXISTS idx_wallet_transactions_wallet_created_keyset
    ON wallet_transactions (wallet_id, created_at DESC, transaction_id DESC);

-- ============================================================================
-- Consolidated from 004_booking_reminder_jobs.sql
-- ============================================================================

CREATE TABLE IF NOT EXISTS booking_reminder_jobs (
    job_id BIGSERIAL PRIMARY KEY,
    booking_id INT NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    scheduled_start TIMESTAMP NOT NULL,
    due_at TIMESTAMP NOT NULL,
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (booking_id, event_type)
);

CREATE INDEX IF NOT EXISTS idx_booking_reminder_jobs_due_unprocessed
    ON booking_reminder_jobs (due_at, job_id)
    WHERE processed_at IS NULL;

CREATE OR REPLACE FUNCTION enqueue_booking_reminder_jobs()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'assigned' AND NEW.scheduled_start IS NOT NULL THEN
        INSERT INTO booking_reminder_jobs (booking_id, event_type, scheduled_start, due_at)
        SELECT NEW.booking_id, reminder.event_type, NEW.scheduled_start, NEW.scheduled_start - reminder.before_start
        FROM (VALUES
            ('reminder_24h'::text, INTERVAL '24 hours'),
            ('reminder_2h'::text, INTERVAL '2 hours')
        ) AS reminder(event_type, before_start)
        ON CONFLICT (booking_id, event_type) DO UPDATE
        SET scheduled_start = EXCLUDED.scheduled_start,
            due_at = EXCLUDED.due_at,
            processed_at = CASE
                WHEN booking_reminder_jobs.scheduled_start IS DISTINCT FROM EXCLUDED.scheduled_start THEN NULL
                ELSE booking_reminder_jobs.processed_at
            END,
            updated_at = NOW();
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_enqueue_booking_reminder_jobs ON bookings;
CREATE TRIGGER trg_enqueue_booking_reminder_jobs
AFTER INSERT OR UPDATE OF status, scheduled_start ON bookings
FOR EACH ROW
EXECUTE FUNCTION enqueue_booking_reminder_jobs();

-- ============================================================================
-- Consolidated from 005_backfill_booking_reminder_jobs.sql
-- ============================================================================

INSERT INTO booking_reminder_jobs (booking_id, event_type, scheduled_start, due_at)
SELECT
    b.booking_id,
    reminder.event_type,
    b.scheduled_start,
    b.scheduled_start - reminder.before_start
FROM bookings b
CROSS JOIN (VALUES
    ('reminder_24h'::text, INTERVAL '24 hours'),
    ('reminder_2h'::text, INTERVAL '2 hours')
) AS reminder(event_type, before_start)
WHERE b.status = 'assigned'
  AND b.scheduled_start IS NOT NULL
  AND b.scheduled_start > NOW()
ON CONFLICT (booking_id, event_type) DO UPDATE
SET scheduled_start = EXCLUDED.scheduled_start,
    due_at = EXCLUDED.due_at,
    processed_at = CASE
        WHEN booking_reminder_jobs.scheduled_start IS DISTINCT FROM EXCLUDED.scheduled_start THEN NULL
        ELSE booking_reminder_jobs.processed_at
    END,
    updated_at = NOW();

-- ============================================================================
-- Consolidated from 006_blog_posts.sql
-- ============================================================================

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

-- ============================================================================
-- Consolidated from 006_staff_out_times.sql
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_out_times (
    out_time_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    work_date DATE NOT NULL,
    out_at TIMESTAMPTZ NOT NULL,
    notes TEXT,
    created_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_out_times_active_user_date
    ON staff_out_times(user_id, work_date)
    WHERE voided_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_staff_out_times_work_date
    ON staff_out_times(work_date)
    WHERE voided_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_staff_out_times_user
    ON staff_out_times(user_id)
    WHERE voided_at IS NULL;

-- ============================================================================
-- Consolidated from 007_payroll_foundations.sql
-- ============================================================================

CREATE TABLE IF NOT EXISTS staff_attendance_entries (
    attendance_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    work_date DATE NOT NULL,
    time_in_at TIMESTAMPTZ,
    time_out_at TIMESTAMPTZ,
    notes TEXT,
    created_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (time_out_at IS NULL OR time_in_at IS NULL OR time_out_at > time_in_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_attendance_active_user_date
ON staff_attendance_entries(user_id, work_date)
WHERE voided_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_staff_attendance_work_date
ON staff_attendance_entries(work_date)
WHERE voided_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_staff_attendance_user
ON staff_attendance_entries(user_id)
WHERE voided_at IS NULL;

INSERT INTO staff_attendance_entries (
    user_id, work_date, time_in_at, time_out_at, notes,
    created_by, updated_by, voided_by, voided_at, created_at, updated_at
)
SELECT
    user_id, work_date, NULL, out_at, notes,
    created_by, updated_by, voided_by, voided_at, created_at, updated_at
FROM staff_out_times
ON CONFLICT DO NOTHING;

ALTER TABLE rider_profiles
ADD COLUMN IF NOT EXISTS usual_branch_id BIGINT REFERENCES branches(branch_id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS usual_location_label TEXT;

CREATE INDEX IF NOT EXISTS idx_rider_profiles_usual_branch
ON rider_profiles(usual_branch_id)
WHERE usual_branch_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS staff_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(user_id) ON DELETE RESTRICT,
    usual_branch_id BIGINT REFERENCES branches(branch_id) ON DELETE SET NULL,
    usual_location_label TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_staff_profiles_usual_branch
ON staff_profiles(usual_branch_id)
WHERE usual_branch_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS staff_compensation_rates (
    rate_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    role TEXT NOT NULL CHECK (role IN ('rider', 'admin')),
    daily_rate_cents BIGINT NOT NULL CHECK (daily_rate_cents >= 0),
    overtime_multiplier NUMERIC(8,4) NOT NULL DEFAULT 1.2500 CHECK (overtime_multiplier >= 0),
    effective_from DATE NOT NULL,
    effective_to DATE,
    notes TEXT,
    created_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

CREATE INDEX IF NOT EXISTS idx_staff_comp_rates_user_dates
ON staff_compensation_rates(user_id, effective_from, effective_to);

CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_comp_rates_user_open
ON staff_compensation_rates(user_id)
WHERE effective_to IS NULL;

CREATE TABLE IF NOT EXISTS staff_payroll_adjustments (
    adjustment_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    role TEXT NOT NULL CHECK (role IN ('therapist', 'rider', 'admin')),
    adjustment_date DATE NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('add', 'minus')),
    category TEXT NOT NULL CHECK (category IN (
        'benefits',
        'cash_advance',
        'salary_correction',
        'attendance_correction',
        'bonus',
        'deduction',
        'parcel',
        'absence',
        'other'
    )),
    amount_cents BIGINT NOT NULL CHECK (amount_cents >= 0),
    reason TEXT NOT NULL,
    cash_movement_cents BIGINT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    updated_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (period_end >= period_start)
);

CREATE INDEX IF NOT EXISTS idx_staff_payroll_adjustments_period
ON staff_payroll_adjustments(period_start, period_end)
WHERE voided_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_staff_payroll_adjustments_user
ON staff_payroll_adjustments(user_id)
WHERE voided_at IS NULL;

CREATE TABLE IF NOT EXISTS payroll_runs (
    payroll_run_id BIGSERIAL PRIMARY KEY,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'paid', 'voided')),
    generated_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    approved_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    voided_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    voided_at TIMESTAMPTZ,
    voided_reason TEXT,
    replaced_by_run_id BIGINT REFERENCES payroll_runs(payroll_run_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (period_end >= period_start)
);

CREATE INDEX IF NOT EXISTS idx_payroll_runs_period
ON payroll_runs(period_start, period_end);

CREATE INDEX IF NOT EXISTS idx_payroll_runs_status
ON payroll_runs(status);

CREATE TABLE IF NOT EXISTS payroll_rows (
    payroll_row_id BIGSERIAL PRIMARY KEY,
    payroll_run_id BIGINT NOT NULL REFERENCES payroll_runs(payroll_run_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    role TEXT NOT NULL CHECK (role IN ('therapist', 'rider', 'admin')),
    full_name_snapshot TEXT NOT NULL,
    usual_branch_id_snapshot BIGINT REFERENCES branches(branch_id) ON DELETE SET NULL,
    usual_location_label_snapshot TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'paid', 'blocked', 'voided')),
    regular_minutes INTEGER NOT NULL DEFAULT 0 CHECK (regular_minutes >= 0),
    overtime_minutes INTEGER NOT NULL DEFAULT 0 CHECK (overtime_minutes >= 0),
    daily_rate_cents BIGINT,
    overtime_multiplier NUMERIC(8,4),
    gross_cents BIGINT NOT NULL DEFAULT 0,
    add_adjustments_cents BIGINT NOT NULL DEFAULT 0,
    minus_adjustments_cents BIGINT NOT NULL DEFAULT 0,
    final_pay_cents BIGINT NOT NULL DEFAULT 0,
    blocker_codes TEXT[] NOT NULL DEFAULT '{}',
    paid_at TIMESTAMPTZ,
    paid_by BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    payment_method TEXT CHECK (payment_method IS NULL OR payment_method IN ('cash', 'gcash', 'bank_transfer', 'other')),
    payment_reference TEXT,
    payment_notes TEXT,
    ledger_entry_id BIGINT REFERENCES ledger_entries(entry_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payroll_rows_run_user
ON payroll_rows(payroll_run_id, user_id);

CREATE INDEX IF NOT EXISTS idx_payroll_rows_user_status
ON payroll_rows(user_id, status);

CREATE TABLE IF NOT EXISTS payroll_attendance_details (
    detail_id BIGSERIAL PRIMARY KEY,
    payroll_row_id BIGINT NOT NULL REFERENCES payroll_rows(payroll_row_id) ON DELETE CASCADE,
    attendance_id BIGINT NOT NULL REFERENCES staff_attendance_entries(attendance_id) ON DELETE RESTRICT,
    work_date DATE NOT NULL,
    time_in_at TIMESTAMPTZ,
    time_out_at TIMESTAMPTZ,
    worked_minutes INTEGER NOT NULL DEFAULT 0,
    regular_minutes INTEGER NOT NULL DEFAULT 0,
    overtime_minutes INTEGER NOT NULL DEFAULT 0,
    daily_rate_cents BIGINT,
    overtime_multiplier NUMERIC(8,4),
    gross_cents BIGINT NOT NULL DEFAULT 0,
    source_updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payroll_attendance_detail_source
ON payroll_attendance_details(attendance_id)
WHERE payroll_row_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS payroll_booking_details (
    detail_id BIGSERIAL PRIMARY KEY,
    payroll_row_id BIGINT NOT NULL REFERENCES payroll_rows(payroll_row_id) ON DELETE CASCADE,
    booking_id BIGINT NOT NULL REFERENCES bookings(booking_id) ON DELETE RESTRICT,
    business_date DATE NOT NULL,
    service_name TEXT NOT NULL,
    duration_minutes INTEGER NOT NULL DEFAULT 0,
    final_total_cents BIGINT NOT NULL DEFAULT 0,
    therapist_earnings_cents BIGINT NOT NULL DEFAULT 0,
    source_updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payroll_booking_detail_source
ON payroll_booking_details(booking_id)
WHERE payroll_row_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS payroll_adjustment_details (
    detail_id BIGSERIAL PRIMARY KEY,
    payroll_row_id BIGINT NOT NULL REFERENCES payroll_rows(payroll_row_id) ON DELETE CASCADE,
    adjustment_id BIGINT NOT NULL REFERENCES staff_payroll_adjustments(adjustment_id) ON DELETE RESTRICT,
    adjustment_date DATE NOT NULL,
    type TEXT NOT NULL,
    category TEXT NOT NULL,
    amount_cents BIGINT NOT NULL,
    reason TEXT NOT NULL,
    source_updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_payroll_adjustment_detail_source
ON payroll_adjustment_details(adjustment_id)
WHERE payroll_row_id IS NOT NULL;

ALTER TABLE ledger_entries
ADD COLUMN IF NOT EXISTS payroll_run_id BIGINT REFERENCES payroll_runs(payroll_run_id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS payroll_row_id BIGINT REFERENCES payroll_rows(payroll_row_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_ledger_entries_payroll_row
ON ledger_entries(payroll_row_id)
WHERE payroll_row_id IS NOT NULL;

-- ============================================================================
-- Consolidated from 008_client_blocked_account_status.sql
-- ============================================================================

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_account_status_check;
ALTER TABLE users DROP CONSTRAINT IF EXISTS check_account_status;

ALTER TABLE users
ADD CONSTRAINT check_account_status
CHECK (account_status IN ('active', 'banned', 'suspended', 'inactive', 'blocked'));

-- ============================================================================
-- Consolidated from 009_add_client_vip.sql
-- ============================================================================

ALTER TABLE users
ADD COLUMN IF NOT EXISTS is_vip BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_vip ON users(is_vip) WHERE is_vip = TRUE;
