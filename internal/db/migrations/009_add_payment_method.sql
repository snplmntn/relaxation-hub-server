-- Migration 009: add payment_method to bookings
-- Adds a column to store the client's chosen payment method (cash/gcash).
BEGIN;

ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20) 
        CHECK (payment_method IN ('cash', 'gcash'))
        NOT NULL DEFAULT 'cash';

COMMIT;
