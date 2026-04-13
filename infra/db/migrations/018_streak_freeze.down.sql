-- tl-2n5 rollback.
DROP INDEX IF EXISTS gaming_profiles_streak_frozen_until_idx;
ALTER TABLE gaming_profiles DROP COLUMN IF EXISTS streak_frozen_until;
