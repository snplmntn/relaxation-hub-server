CREATE TABLE IF NOT EXISTS booking_announcements (
    announcement_id BIGSERIAL PRIMARY KEY,
    message TEXT NOT NULL CHECK (btrim(message) <> ''),
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT booking_announcements_valid_range CHECK (end_date >= start_date)
);

CREATE INDEX IF NOT EXISTS idx_booking_announcements_dates
    ON booking_announcements (start_date, end_date);
