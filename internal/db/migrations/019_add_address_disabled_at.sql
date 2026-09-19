-- Forward migration: support disabling (vs deleting) a client address.
-- A disabled address is preserved and can be re-enabled; it is excluded from
-- new-booking selection. Distinct from deleted_at (soft delete).

ALTER TABLE public.addresses
    ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMP;
