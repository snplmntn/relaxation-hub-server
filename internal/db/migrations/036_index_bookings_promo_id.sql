-- Voucher booking inventory and usage-limit checks filter on promo_id.
CREATE INDEX IF NOT EXISTS idx_bookings_promo_id
    ON bookings (promo_id)
    WHERE promo_id IS NOT NULL;
