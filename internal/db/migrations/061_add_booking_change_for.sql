-- +goose Up
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS change_for DECIMAL(10, 2);
