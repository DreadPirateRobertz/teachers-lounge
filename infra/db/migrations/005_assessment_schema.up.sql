-- TeachersLounge — Learning Style Assessment Schema
-- Stores assessment sessions and per-question answers.
-- Questions are embedded in the gaming-service (no DB table needed).

BEGIN;

-- ============================================================
-- ASSESSMENT SESSIONS
-- ============================================================
-- results: {"active_reflective": 0.4, "sensing_intuitive": -0.2,
--            "visual_verbal": -0.6, "sequential_global": 0.1}
-- All dial values are in [-1.0, 1.0]:
--   active_reflective : -1 = strongly active,   +1 = strongly reflective
--   sensing_intuitive : -1 = strongly sensing,  +1 = strongly intuitive
--   visual_verbal     : -1 = strongly visual,   +1 = strongly verbal
--   sequential_global : -1 = strongly sequential,+1 = strongly global
CREATE TABLE assessment_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'completed', 'abandoned')),
    current_index   INT NOT NULL DEFAULT 0,
    total_questions INT NOT NULL DEFAULT 12,
    xp_earned       INT NOT NULL DEFAULT 0,
    results         JSONB,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_assessment_sessions_user_id ON assessment_sessions(user_id);
CREATE INDEX idx_assessment_sessions_status  ON assessment_sessions(user_id, status);

-- ============================================================
-- ASSESSMENT ANSWERS
-- ============================================================
CREATE TABLE assessment_answers (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id  UUID NOT NULL REFERENCES assessment_sessions(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id TEXT NOT NULL,   -- matches assessment.Question.ID (static list)
    chosen_key  TEXT NOT NULL,   -- "A" or "B"
    answered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assessment_answers_session_id ON assessment_answers(session_id);
CREATE INDEX idx_assessment_answers_user_id    ON assessment_answers(user_id);

-- ============================================================
-- ROW LEVEL SECURITY
-- ============================================================
ALTER TABLE assessment_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment_answers  ENABLE ROW LEVEL SECURITY;

CREATE POLICY student_isolation ON assessment_sessions
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON assessment_answers
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

COMMIT;
