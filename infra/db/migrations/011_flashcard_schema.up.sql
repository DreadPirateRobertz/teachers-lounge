BEGIN;

CREATE TABLE flashcards (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  question_id     UUID REFERENCES question_bank(id) ON DELETE SET NULL,
  session_id      UUID REFERENCES quiz_sessions(id) ON DELETE SET NULL,
  front           TEXT NOT NULL,
  back            TEXT NOT NULL,
  source          TEXT NOT NULL DEFAULT 'quiz' CHECK (source IN ('quiz','manual')),
  topic           TEXT,
  course_id       UUID,
  -- SM-2 scheduling fields
  ease_factor     FLOAT NOT NULL DEFAULT 2.5,
  interval_days   INT NOT NULL DEFAULT 1,
  repetitions     INT NOT NULL DEFAULT 0,
  next_review_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_reviewed_at TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_flashcards_user_id        ON flashcards(user_id);
CREATE INDEX idx_flashcards_next_review    ON flashcards(user_id, next_review_at);
CREATE INDEX idx_flashcards_session_id     ON flashcards(session_id);

CREATE TABLE flashcard_reviews (
  id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  flashcard_id       UUID NOT NULL REFERENCES flashcards(id) ON DELETE CASCADE,
  user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  quality            INT NOT NULL CHECK (quality BETWEEN 0 AND 5),
  ease_factor_before FLOAT NOT NULL,
  interval_before    INT NOT NULL,
  ease_factor_after  FLOAT NOT NULL,
  interval_after     INT NOT NULL,
  reviewed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_flashcard_reviews_flashcard_id ON flashcard_reviews(flashcard_id);
CREATE INDEX idx_flashcard_reviews_user_id      ON flashcard_reviews(user_id);

ALTER TABLE flashcards       ENABLE ROW LEVEL SECURITY;
ALTER TABLE flashcard_reviews ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_isolation ON flashcards
    USING (user_id = current_setting('app.current_user_id', true)::uuid);
CREATE POLICY user_isolation ON flashcard_reviews
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

COMMIT;
