-- Unified attendance and payroll foundations.

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
