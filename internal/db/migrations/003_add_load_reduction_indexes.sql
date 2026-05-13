-- Add duplicate-safe indexes for measured and planned load hotspots.
-- Keep this migration additive only; historical migrations may already be applied.

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
