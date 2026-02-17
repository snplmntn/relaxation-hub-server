-- Migration 010: drop therapist_service_pressures after migration to boolean columns

-- WARNING: Ensure migration 009 has been applied and data verified before running this.
-- This migration removes the legacy pressures table and related indexes.

BEGIN;

DROP INDEX IF EXISTS idx_tsp_service;
DROP INDEX IF EXISTS idx_tsp_therapist;

DROP TABLE IF EXISTS therapist_service_pressures;

COMMIT;
