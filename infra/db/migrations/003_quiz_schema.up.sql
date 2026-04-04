-- TeachersLounge — Quiz System Schema
-- Adds question bank, quiz sessions, and quiz answer tables.

BEGIN;

CREATE TYPE quiz_status AS ENUM ('active', 'completed', 'abandoned');

-- ============================================================
-- QUESTION BANK
-- ============================================================
-- options: [{"key": "A", "text": "..."}, {"key": "B", "text": "..."}, ...]
-- hints:   ["broad hint", "more specific hint", "near-answer hint"]
CREATE TABLE question_bank (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    course_id   UUID REFERENCES courses(id) ON DELETE CASCADE,
    topic       TEXT NOT NULL,
    difficulty  INT NOT NULL DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 5),
    question    TEXT NOT NULL,
    options     JSONB NOT NULL DEFAULT '[]',
    correct_key TEXT NOT NULL,
    hints       JSONB NOT NULL DEFAULT '[]',
    explanation TEXT NOT NULL DEFAULT '',
    xp_reward   INT NOT NULL DEFAULT 10,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_question_bank_topic ON question_bank(topic);
CREATE INDEX idx_question_bank_difficulty ON question_bank(topic, difficulty);

-- ============================================================
-- QUIZ SESSIONS
-- ============================================================
CREATE TABLE quiz_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id       UUID REFERENCES courses(id) ON DELETE SET NULL,
    topic           TEXT,
    status          quiz_status NOT NULL DEFAULT 'active',
    question_ids    UUID[] NOT NULL DEFAULT '{}',
    current_index   INT NOT NULL DEFAULT 0,
    total_questions INT NOT NULL DEFAULT 0,
    correct_count   INT NOT NULL DEFAULT 0,
    total_xp_earned INT NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_quiz_sessions_user_id ON quiz_sessions(user_id);
CREATE INDEX idx_quiz_sessions_status ON quiz_sessions(user_id, status);

-- ============================================================
-- QUIZ ANSWERS
-- ============================================================
CREATE TABLE quiz_answers (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id    UUID NOT NULL REFERENCES quiz_sessions(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id   UUID NOT NULL REFERENCES question_bank(id) ON DELETE CASCADE,
    chosen_key    TEXT NOT NULL,
    is_correct    BOOLEAN NOT NULL,
    hints_used    INT NOT NULL DEFAULT 0,
    xp_earned     INT NOT NULL DEFAULT 0,
    time_taken_ms INT,
    answered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_quiz_answers_session_id ON quiz_answers(session_id);
CREATE INDEX idx_quiz_answers_user_id ON quiz_answers(user_id);

-- ============================================================
-- ROW LEVEL SECURITY
-- ============================================================
ALTER TABLE quiz_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE quiz_answers  ENABLE ROW LEVEL SECURITY;
ALTER TABLE question_bank ENABLE ROW LEVEL SECURITY;

CREATE POLICY student_isolation ON quiz_sessions
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON quiz_answers
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

-- question_bank is shared across users — allow all authenticated reads
CREATE POLICY read_all ON question_bank FOR SELECT USING (true);

COMMIT;
