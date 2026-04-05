-- TeachersLounge — Student Knowledge Model (tl-vw6)
-- Adds learning_profiles, explanation_preferences, and misconceptions tables.
-- These back the SKM adaptive layer: style dials, explanation history, error tracking.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Per-student Felder-Silverman learning-style dials (local, authoritative store)
CREATE TABLE learning_profiles (
  user_id           UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  active_reflective FLOAT NOT NULL DEFAULT 0.0
      CHECK (active_reflective BETWEEN -1 AND 1),
  sensing_intuitive FLOAT NOT NULL DEFAULT 0.0
      CHECK (sensing_intuitive BETWEEN -1 AND 1),
  visual_verbal     FLOAT NOT NULL DEFAULT 0.0
      CHECK (visual_verbal     BETWEEN -1 AND 1),
  sequential_global FLOAT NOT NULL DEFAULT 0.0
      CHECK (sequential_global BETWEEN -1 AND 1),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Log of which explanation types helped a student understand a specific concept.
-- Accumulated over interactions; used to personalise future explanations.
CREATE TABLE explanation_preferences (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id          UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
  concept_id       UUID        NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
  explanation_type VARCHAR(50) NOT NULL,   -- 'visual', 'verbal', 'example', 'derivation', 'analogy'
  helpful          BOOLEAN     NOT NULL,
  recorded_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_explanation_prefs_user_concept
    ON explanation_preferences(user_id, concept_id);

-- Misconception log: tracked student errors with recency-weighted confidence.
-- confidence decays over time in application logic; resolved=TRUE dismisses the entry.
CREATE TABLE misconceptions (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
  concept_id   UUID        NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
  description  TEXT        NOT NULL,
  confidence   FLOAT       NOT NULL DEFAULT 1.0
                  CHECK (confidence BETWEEN 0 AND 1),
  recorded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved     BOOLEAN     NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_misconceptions_user_concept
    ON misconceptions(user_id, concept_id);
CREATE INDEX idx_misconceptions_user_recent
    ON misconceptions(user_id, last_seen_at DESC)
    WHERE resolved = FALSE;

COMMIT;
