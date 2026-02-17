-- Add change_for column to bookings table
ALTER TABLE bookings ADD COLUMN change_for DECIMAL(10, 2);
