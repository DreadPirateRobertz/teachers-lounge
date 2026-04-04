-- TeachersLounge — Phase 1 Postgres Schema
-- Cloud SQL (Postgres 15+)
-- Run order: this file first, then 002_pgvector.sql

BEGIN;

-- ============================================================
-- EXTENSIONS
-- ============================================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "ltree";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";  -- for ILIKE indexes on email/name

-- ============================================================
-- ENUM TYPES
-- ============================================================
CREATE TYPE account_type      AS ENUM ('standard', 'minor');
CREATE TYPE subscription_plan AS ENUM ('trial', 'monthly', 'quarterly', 'semesterly');
CREATE TYPE subscription_status AS ENUM ('trialing', 'active', 'past_due', 'cancelled', 'expired');
CREATE TYPE interaction_role  AS ENUM ('student', 'tutor');
CREATE TYPE content_type      AS ENUM ('text', 'table', 'equation', 'figure', 'quiz');
CREATE TYPE boss_result       AS ENUM ('victory', 'defeat');
CREATE TYPE export_status     AS ENUM ('pending', 'processing', 'complete', 'failed');
CREATE TYPE processing_status AS ENUM ('pending', 'processing', 'complete', 'failed');

-- ============================================================
-- USERS & AUTH
-- ============================================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    avatar_emoji    TEXT NOT NULL DEFAULT '🎓',
    account_type    account_type NOT NULL DEFAULT 'standard',
    date_of_birth   DATE,                        -- K-12 hook: age gate
    -- parental consent skeleton (K-12, not active at launch)
    guardian_email  TEXT,
    guardian_consent_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users USING gin (email gin_trgm_ops);

-- Refresh tokens (hashed; raw value stored in HTTP-only cookie)
CREATE TABLE auth_tokens (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL UNIQUE,
    device_info     JSONB,                       -- {"user_agent": "...", "ip": "..."}
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX idx_auth_tokens_hash ON auth_tokens(token_hash)
    WHERE revoked_at IS NULL;

-- ============================================================
-- SUBSCRIPTIONS
-- ============================================================
CREATE TABLE subscriptions (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id                 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stripe_customer_id      TEXT NOT NULL,
    stripe_subscription_id  TEXT,               -- NULL during trial before card added
    plan                    subscription_plan NOT NULL,
    status                  subscription_status NOT NULL DEFAULT 'trialing',
    current_period_start    TIMESTAMPTZ,
    current_period_end      TIMESTAMPTZ,
    trial_end               TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)          -- one active subscription per user
);

CREATE INDEX idx_subscriptions_stripe_customer ON subscriptions(stripe_customer_id);
CREATE INDEX idx_subscriptions_stripe_sub ON subscriptions(stripe_subscription_id)
    WHERE stripe_subscription_id IS NOT NULL;

-- ============================================================
-- LEARNING PROFILES (JSONB — flexible schema)
-- ============================================================
CREATE TABLE learning_profiles (
    user_id                     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Felder-Silverman dials: {"active_reflective": 0.6, "sensing_intuitive": 0.3, ...}
    felder_silverman_dials      JSONB NOT NULL DEFAULT '{}',
    -- VARK preferences: {"visual": 0.7, "auditory": 0.4, "reading": 0.5, "kinesthetic": 0.3}
    learning_style_preferences  JSONB NOT NULL DEFAULT '{}',
    -- {"topic": "quadratic formula", "misconception": "confuses discriminant sign", "seen_count": 3}
    misconception_log           JSONB NOT NULL DEFAULT '[]',
    -- {"format": "step_by_step", "analogy_style": "sports", "explanation_depth": "detailed"}
    explanation_preferences     JSONB NOT NULL DEFAULT '{}',
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- COURSES & MATERIALS
-- ============================================================
CREATE TABLE courses (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_courses_user_id ON courses(user_id);

CREATE TABLE materials (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    course_id           UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    filename            TEXT NOT NULL,
    gcs_path            TEXT NOT NULL,
    file_type           TEXT NOT NULL,           -- pdf, docx, pptx, mp4, mp3, etc.
    processing_status   processing_status NOT NULL DEFAULT 'pending',
    chunk_count         INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_materials_course_id ON materials(course_id);

-- ============================================================
-- INTERACTION HISTORY
-- ============================================================
CREATE TABLE interactions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id       UUID REFERENCES courses(id) ON DELETE SET NULL,
    session_id      UUID NOT NULL,               -- client-generated session UUID
    role            interaction_role NOT NULL,
    content         TEXT NOT NULL,
    chunks_used     JSONB,                       -- [{chunk_id, score, rank}]
    response_time_ms INT,
    -- pgvector column added in 002_pgvector.sql
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_interactions_user_id ON interactions(user_id);
CREATE INDEX idx_interactions_session_id ON interactions(session_id);
CREATE INDEX idx_interactions_course_id ON interactions(course_id);
CREATE INDEX idx_interactions_created_at ON interactions(created_at DESC);

-- ============================================================
-- ASSESSMENTS
-- ============================================================
CREATE TABLE quiz_results (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id       UUID REFERENCES courses(id) ON DELETE SET NULL,
    topic           TEXT NOT NULL,
    question        TEXT NOT NULL,
    student_answer  TEXT NOT NULL,
    correct_answer  TEXT NOT NULL,
    is_correct      BOOLEAN NOT NULL,
    time_taken_ms   INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_quiz_results_user_id ON quiz_results(user_id);
CREATE INDEX idx_quiz_results_topic ON quiz_results(user_id, topic);

-- ============================================================
-- GAMING (persistent state)
-- ============================================================
CREATE TABLE gaming_profiles (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    level           INT NOT NULL DEFAULT 1,
    xp              BIGINT NOT NULL DEFAULT 0,
    total_questions INT NOT NULL DEFAULT 0,
    correct_answers INT NOT NULL DEFAULT 0,
    current_streak  INT NOT NULL DEFAULT 0,
    longest_streak  INT NOT NULL DEFAULT 0,
    bosses_defeated INT NOT NULL DEFAULT 0,
    gems            INT NOT NULL DEFAULT 0,
    -- {"fireball": 2, "time_freeze": 1, "hint_token": 5}
    power_ups       JSONB NOT NULL DEFAULT '{}',
    last_study_date DATE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE achievements (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_type TEXT NOT NULL,              -- e.g. "first_boss", "streak_7", "molecule_master"
    earned_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, achievement_type)
);

CREATE INDEX idx_achievements_user_id ON achievements(user_id);

CREATE TABLE boss_history (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    boss_name   TEXT NOT NULL,
    topic       TEXT NOT NULL,
    rounds      INT NOT NULL,
    result      boss_result NOT NULL,
    xp_earned   INT NOT NULL DEFAULT 0,
    fought_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_boss_history_user_id ON boss_history(user_id);

-- ============================================================
-- DOCUMENT CHUNKS (metadata — vectors in Qdrant)
-- ============================================================
CREATE TABLE chunks (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    material_id     UUID NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    course_id       UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    content         TEXT NOT NULL,
    chapter         TEXT,
    section         TEXT,
    page            INT,
    content_type    content_type NOT NULL DEFAULT 'text',
    figure_gcs_path TEXT,                        -- for figure chunks
    metadata        JSONB NOT NULL DEFAULT '{}', -- flexible per content_type
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chunks_material_id ON chunks(material_id);
CREATE INDEX idx_chunks_course_id ON chunks(course_id);

-- ============================================================
-- CONCEPT KNOWLEDGE GRAPH (Postgres ltree)
-- ============================================================
CREATE TABLE concepts (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    course_id   UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    path        LTREE NOT NULL,                  -- e.g. chemistry.organic.reactions.substitution
    UNIQUE(course_id, name)
);

CREATE INDEX idx_concepts_course_id ON concepts(course_id);
CREATE INDEX idx_concepts_path ON concepts USING gist(path);

CREATE TABLE concept_prerequisites (
    concept_id      UUID NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    prerequisite_id UUID NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    weight          FLOAT NOT NULL DEFAULT 1.0,  -- strength of dependency
    PRIMARY KEY(concept_id, prerequisite_id)
);

CREATE TABLE student_concept_mastery (
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    concept_id      UUID NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    mastery_score   FLOAT NOT NULL DEFAULT 0.0 CHECK (mastery_score BETWEEN 0 AND 1),
    last_reviewed_at TIMESTAMPTZ,
    next_review_at  TIMESTAMPTZ,                 -- SM-2 spaced repetition
    decay_rate      FLOAT NOT NULL DEFAULT 0.1,
    PRIMARY KEY(user_id, concept_id)
);

CREATE INDEX idx_student_mastery_user ON student_concept_mastery(user_id);
CREATE INDEX idx_student_mastery_next_review ON student_concept_mastery(user_id, next_review_at);

-- ============================================================
-- FERPA AUDIT TRAIL
-- ============================================================
CREATE TABLE audit_log (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accessor_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    student_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,               -- "read", "export", "delete", etc.
    data_accessed   TEXT NOT NULL,               -- description of data accessed
    purpose         TEXT NOT NULL,
    ip_address      INET
);

CREATE INDEX idx_audit_log_student_id ON audit_log(student_id);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp DESC);

-- ============================================================
-- DATA EXPORT JOBS
-- ============================================================
CREATE TABLE export_jobs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      export_status NOT NULL DEFAULT 'pending',
    gcs_path    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_export_jobs_user_id ON export_jobs(user_id);

-- ============================================================
-- SCI-FI QUOTES (Phase 1: seeded, cached in Redis)
-- ============================================================
CREATE TABLE scifi_quotes (
    id          SERIAL PRIMARY KEY,
    quote       TEXT NOT NULL,
    attribution TEXT NOT NULL,  -- "Character — Work"
    context     TEXT NOT NULL   -- session_start|boss_fight|correct|wrong|victory|defeat|streak|achievement|comeback
);

-- ============================================================
-- ROW LEVEL SECURITY
-- ============================================================
-- Enable RLS on all student-data tables. Services set app.current_user_id
-- before queries. Service accounts bypass via BYPASSRLS role.

ALTER TABLE users                   ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_tokens             ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions           ENABLE ROW LEVEL SECURITY;
ALTER TABLE learning_profiles       ENABLE ROW LEVEL SECURITY;
ALTER TABLE courses                 ENABLE ROW LEVEL SECURITY;
ALTER TABLE materials               ENABLE ROW LEVEL SECURITY;
ALTER TABLE interactions            ENABLE ROW LEVEL SECURITY;
ALTER TABLE quiz_results            ENABLE ROW LEVEL SECURITY;
ALTER TABLE gaming_profiles         ENABLE ROW LEVEL SECURITY;
ALTER TABLE achievements            ENABLE ROW LEVEL SECURITY;
ALTER TABLE boss_history            ENABLE ROW LEVEL SECURITY;
ALTER TABLE chunks                  ENABLE ROW LEVEL SECURITY;
ALTER TABLE student_concept_mastery ENABLE ROW LEVEL SECURITY;
ALTER TABLE export_jobs             ENABLE ROW LEVEL SECURITY;

-- Policies: students see only their own rows
CREATE POLICY student_isolation ON users
    USING (id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON auth_tokens
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON subscriptions
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON learning_profiles
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON courses
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON interactions
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON quiz_results
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON gaming_profiles
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON achievements
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON boss_history
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON student_concept_mastery
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

CREATE POLICY student_isolation ON export_jobs
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

-- ============================================================
-- UPDATED_AT TRIGGER
-- ============================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_courses_updated_at
    BEFORE UPDATE ON courses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_learning_profiles_updated_at
    BEFORE UPDATE ON learning_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_gaming_profiles_updated_at
    BEFORE UPDATE ON gaming_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
