-- Rollback 002_pgvector.up.sql

BEGIN;

DROP INDEX IF EXISTS idx_interactions_embedding;
ALTER TABLE interactions DROP COLUMN IF EXISTS embedding;

ALTER TABLE learning_profiles DROP COLUMN IF EXISTS history_embedding;

DROP EXTENSION IF EXISTS vector;

COMMIT;
