-- Migration 017: Onboarding flow state
--
-- Adds onboarding tracking columns to users so the frontend wizard can
-- check whether a user has completed first-run setup and redirect
-- accordingly.  Default FALSE ensures existing users appear as though
-- they have already onboarded (opt-out) rather than forcing them through
-- the wizard retroactively.

BEGIN;

ALTER TABLE users
  ADD COLUMN has_completed_onboarding BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN onboarded_at             TIMESTAMP WITH TIME ZONE;

COMMIT;
