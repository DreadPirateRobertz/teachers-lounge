-- TeachersLounge — flashcard system (tl-y3v)
-- Auto-generated flashcards from tutoring session transcripts, SM-2 scheduled
-- for review, and exportable to Anki.  Distinct from concept_review_schedule
-- (global concept slugs) and student_concept_mastery (per-course concept UUIDs)
-- because flashcards are free-text Q/A pairs keyed by card ID.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE flashcards (
  id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  front               TEXT        NOT NULL,
  back                TEXT        NOT NULL,
  concept_id          TEXT        NULL,            -- optional tag for Anki export
  source_session_id   UUID        NULL REFERENCES chat_sessions(id) ON DELETE SET NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_reviewed_at    TIMESTAMPTZ NULL,
  due_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),   -- newly generated cards are immediately due
  sm2_interval_days   INT         NOT NULL DEFAULT 1,
  sm2_ease_factor     FLOAT       NOT NULL DEFAULT 2.5,
  sm2_repetitions     INT         NOT NULL DEFAULT 0
);

-- Powers GET /flashcards?due=true: filter by user, sort by due_at ascending.
CREATE INDEX ix_flashcards_user_due ON flashcards (user_id, due_at);

-- Powers dedup inside /flashcards/generate (avoid re-inserting Q/A pairs that
-- already exist for this session) and the session-history UI.
CREATE INDEX ix_flashcards_user_session ON flashcards (user_id, source_session_id);

ALTER TABLE flashcards ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_isolation ON flashcards
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

COMMIT;
