-- Migration 009: FERPA/GDPR compliance additions
--
-- Adds:
--   users.is_admin          — grants access to GET /admin/audit
--   users.guardian_consented — explicit boolean for minor consent status
--   export_jobs.result_data — JSONB payload written when job completes

BEGIN;

ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE export_jobs ADD COLUMN result_data JSONB;

COMMIT;
