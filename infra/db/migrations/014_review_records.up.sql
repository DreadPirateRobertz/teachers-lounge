-- TeachersLounge — SKM review records table (tl-vw6)
-- Persists per-response audit trail for the SM-2 spaced-repetition scheduler.
-- One row per student review interaction on a concept.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE review_records (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  concept_id    UUID NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
  quality       INT  NOT NULL CHECK (quality BETWEEN 0 AND 5),
  mastery_before FLOAT NOT NULL,
  mastery_after  FLOAT NOT NULL,
  interval_after INT  NOT NULL,
  ease_after     FLOAT NOT NULL,
  reviewed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_review_records_user_id     ON review_records(user_id);
CREATE INDEX idx_review_records_concept_id  ON review_records(concept_id);
CREATE INDEX idx_review_records_user_reviewed_at ON review_records(user_id, reviewed_at DESC);

ALTER TABLE review_records ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_isolation ON review_records
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

COMMIT;
