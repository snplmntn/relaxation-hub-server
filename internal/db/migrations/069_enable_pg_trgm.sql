-- Migration 069: Enable trigram support for ILIKE text search acceleration.

CREATE EXTENSION IF NOT EXISTS pg_trgm;
