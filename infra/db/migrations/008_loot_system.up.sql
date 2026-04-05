-- Migration 006: Loot system — cosmetics inventory + badge names
-- Adds a cosmetics JSONB column to gaming_profiles for storing unlocked
-- cosmetic items (avatar frames, colour palettes, titles), and a badge_name
-- column to achievements so the UI can display human-readable badge labels
-- without needing to look up the type string.

BEGIN;

ALTER TABLE gaming_profiles
    ADD COLUMN IF NOT EXISTS cosmetics JSONB NOT NULL DEFAULT '{}';

ALTER TABLE achievements
    ADD COLUMN IF NOT EXISTS badge_name TEXT NOT NULL DEFAULT '';

COMMIT;
