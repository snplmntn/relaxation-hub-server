CREATE TABLE IF NOT EXISTS booking_announcement_variations (
    variation_id BIGSERIAL PRIMARY KEY,
    announcement_id BIGINT NOT NULL REFERENCES booking_announcements(announcement_id) ON DELETE CASCADE,
    message TEXT NOT NULL CHECK (btrim(message) <> ''),
    position INTEGER NOT NULL CHECK (position > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (announcement_id, position)
);

INSERT INTO booking_announcement_variations (announcement_id, message, position)
SELECT announcement_id, message, 1
FROM booking_announcements
WHERE NOT EXISTS (
    SELECT 1
    FROM booking_announcement_variations v
    WHERE v.announcement_id = booking_announcements.announcement_id
);

ALTER TABLE booking_announcements
    ADD COLUMN IF NOT EXISTS winner_variation_id BIGINT
    REFERENCES booking_announcement_variations(variation_id) ON DELETE RESTRICT;

CREATE TABLE IF NOT EXISTS booking_announcement_assignments (
    assignment_id BIGSERIAL PRIMARY KEY,
    announcement_id BIGINT NOT NULL REFERENCES booking_announcements(announcement_id) ON DELETE CASCADE,
    variation_id BIGINT REFERENCES booking_announcement_variations(variation_id) ON DELETE SET NULL,
    booking_id INTEGER REFERENCES bookings(booking_id) ON DELETE CASCADE,
    group_id INTEGER REFERENCES booking_groups(group_id) ON DELETE CASCADE,
    message_snapshot TEXT NOT NULL CHECK (btrim(message_snapshot) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT booking_announcement_assignments_subject CHECK (num_nonnulls(booking_id, group_id) = 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_booking_announcement_assignments_booking
    ON booking_announcement_assignments (announcement_id, booking_id)
    WHERE booking_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_booking_announcement_assignments_group
    ON booking_announcement_assignments (announcement_id, group_id)
    WHERE group_id IS NOT NULL;
