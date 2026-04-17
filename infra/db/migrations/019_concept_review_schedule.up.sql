-- TeachersLounge — concept review schedule (tl-5wz)
-- Per-student SM-2 schedule keyed on global concept slugs (text concept_id).
-- Distinct from student_concept_mastery, which is keyed on per-course Concept UUIDs.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE concept_review_schedule (
  user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  concept_id       TEXT        NOT NULL,
  ease_factor      FLOAT       NOT NULL DEFAULT 2.5,
  interval_days    INT         NOT NULL DEFAULT 1,
  repetitions      INT         NOT NULL DEFAULT 0,
  last_reviewed_at TIMESTAMPTZ,
  due_at           TIMESTAMPTZ,
  PRIMARY KEY (user_id, concept_id)
);

-- Powers GET /spaced-repetition/due: filter by user, sort by due_at ascending.
CREATE INDEX ix_crs_user_due ON concept_review_schedule (user_id, due_at);

ALTER TABLE concept_review_schedule ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_isolation ON concept_review_schedule
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

COMMIT;
