-- tl-2n5: streak freeze
--
-- Adds streak_frozen_until to gaming_profiles. When NOW() < streak_frozen_until,
-- StreakCheckin must not reset current_streak even if the user missed a day.
-- A freeze is purchased via POST /gaming/streak/freeze for 50 gems and lasts
-- 24 hours from the purchase moment.

ALTER TABLE gaming_profiles
    ADD COLUMN streak_frozen_until TIMESTAMPTZ;

-- Partial index accelerates the "any active freezes?" admin query path.
CREATE INDEX gaming_profiles_streak_frozen_until_idx
    ON gaming_profiles (streak_frozen_until)
    WHERE streak_frozen_until IS NOT NULL;
