-- Migration 046: Add dispatch_buffer_minutes to ride_pricing_config
-- Allows Admin to tune the "Start Trip" buffer based on real-world feedback.

ALTER TABLE ride_pricing_config
ADD COLUMN IF NOT EXISTS dispatch_buffer_minutes INTEGER DEFAULT 30;

COMMENT ON COLUMN ride_pricing_config.dispatch_buffer_minutes IS 'Buffer time in minutes to subtract from ScheduledStart to determine DispatchTime';
