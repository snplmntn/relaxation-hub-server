ALTER TABLE booking_services
ADD COLUMN IF NOT EXISTS allocated_duration_minutes INTEGER;

ALTER TABLE booking_services
DROP CONSTRAINT IF EXISTS booking_services_allocated_duration_positive;

ALTER TABLE booking_services
ADD CONSTRAINT booking_services_allocated_duration_positive
CHECK (allocated_duration_minutes IS NULL OR allocated_duration_minutes > 0);
