-- Rollback migration 006

BEGIN;

ALTER TABLE achievements DROP COLUMN IF EXISTS badge_name;
ALTER TABLE gaming_profiles DROP COLUMN IF EXISTS cosmetics;

COMMIT;
