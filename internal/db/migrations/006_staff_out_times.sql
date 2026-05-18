-- Add role-neutral daily staff out-time records for salary prerequisites.

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

