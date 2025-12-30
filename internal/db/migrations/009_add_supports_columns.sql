-- Migration 009: add boolean support columns to therapist_services and backfill from therapist_service_pressures

BEGIN;

ALTER TABLE IF EXISTS therapist_services
  ADD COLUMN IF NOT EXISTS supports_soft BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS supports_moderate BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS supports_hard BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill values from therapist_service_pressures
UPDATE therapist_services ts
SET
  supports_soft = COALESCE(src.has_soft, false),
  supports_moderate = COALESCE(src.has_medium, false),
  supports_hard = COALESCE(src.has_hard, false)
FROM (
  SELECT tsp.therapist_id, tsp.service_id,
    bool_or(tsp.pressure = 'soft') AS has_soft,
    bool_or(tsp.pressure = 'medium' OR tsp.pressure = 'med' OR tsp.pressure = 'moderate') AS has_medium,
    bool_or(tsp.pressure = 'hard') AS has_hard
  FROM therapist_service_pressures tsp
  GROUP BY tsp.therapist_id, tsp.service_id
) src
WHERE ts.therapist_id = src.therapist_id AND ts.service_id = src.service_id;

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_therapist_services_supports_soft ON therapist_services (service_id) WHERE supports_soft = TRUE;
CREATE INDEX IF NOT EXISTS idx_therapist_services_supports_moderate ON therapist_services (service_id) WHERE supports_moderate = TRUE;
CREATE INDEX IF NOT EXISTS idx_therapist_services_supports_hard ON therapist_services (service_id) WHERE supports_hard = TRUE;

COMMIT;

-- NOTE: Do NOT DROP therapist_service_pressures in this migration. After verification, you may remove it in a later migration:
-- DROP TABLE IF EXISTS therapist_service_pressures;
