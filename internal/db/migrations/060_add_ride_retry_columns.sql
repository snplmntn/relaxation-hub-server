-- +goose Up
ALTER TABLE rides ADD COLUMN retry_count INT NOT NULL DEFAULT 0;
ALTER TABLE rides ADD COLUMN last_retried_at TIMESTAMPTZ;

-- Composite index for GetUnmatchedRidesForRetry: pending rides with no rider, ordered by retry eligibility
CREATE INDEX idx_rides_retry_lookup ON rides (status, rider_id, retry_count, last_retried_at)
  WHERE status = 'pending' AND rider_id IS NULL;

-- Partial index for schedule-aware rider filtering (FindNearbyRiders NOT EXISTS subquery)
CREATE INDEX idx_rides_active_schedule ON rides (rider_id, scheduled_for)
  WHERE status IN ('accepted', 'in_progress', 'arrived') AND scheduled_for IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_rides_active_schedule;
DROP INDEX IF EXISTS idx_rides_retry_lookup;
ALTER TABLE rides DROP COLUMN IF EXISTS last_retried_at;
ALTER TABLE rides DROP COLUMN IF EXISTS retry_count;
