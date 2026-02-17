-- Add reference_code to bookings table
ALTER TABLE bookings ADD COLUMN reference_code VARCHAR(20);

-- Create unique index (implicitly indexes for lookup)
CREATE UNIQUE INDEX idx_bookings_reference_code ON bookings(reference_code);

-- Optional: If we want to backfill existing bookings with a placeholder or generate them
-- For now, we leave them null or let the app handle it.
-- Code format is 'RH-YYYYMMDD-HEX', so we can't easily generate valid ones in SQL without a complex function.
