-- ============================================================================
-- Migration: 007 - Normalize start confirmation event types -> confirm_start
-- ============================================================================
-- This migration consolidates previous role-specific start confirmation
-- event types (client_confirm_start, therapist_confirm_start,
-- admin_confirm_start) into a single canonical `confirm_start` value.
--
-- Run this on your Supabase/Postgres instance (via psql, supabase SQL editor,
-- or migrate tool). It's safe to run multiple times (idempotent update).

BEGIN;

-- Convert any legacy role-specific confirm events into the unified type
UPDATE booking_events
SET event_type = 'confirm_start'
WHERE event_type IN ('client_confirm_start', 'therapist_confirm_start', 'admin_confirm_start');

COMMIT;

-- Optional: verify changes (run as a separate query if you want a count)
-- SELECT event_type, count(*) FROM booking_events WHERE event_type LIKE '%confirm%' GROUP BY 1 ORDER BY 2 DESC;
