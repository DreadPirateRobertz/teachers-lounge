-- Rollback migration 017: remove onboarding columns.

BEGIN;

ALTER TABLE users
  DROP COLUMN IF EXISTS has_completed_onboarding,
  DROP COLUMN IF EXISTS onboarded_at;

COMMIT;
