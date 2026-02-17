-- Migration 018: Prevent duplicate reviews for the same booking
CREATE UNIQUE INDEX idx_reviews_unique_booking_non_deleted 
ON reviews(booking_id) 
WHERE deleted_at IS NULL;
