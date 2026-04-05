-- Migration 013: interaction_quality table for Phase 7 LLM judge (tl-dkg)
-- Stores nightly Claude Haiku evaluations of sampled tutor interactions.

CREATE TABLE interaction_quality (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    interaction_id  UUID        NOT NULL REFERENCES interactions(id) ON DELETE CASCADE,
    judge_score     INTEGER     NOT NULL CHECK (judge_score BETWEEN 1 AND 5),
    judge_reasoning TEXT        NOT NULL,
    score_directness INTEGER    CHECK (score_directness BETWEEN 1 AND 5),
    score_pace       INTEGER    CHECK (score_pace BETWEEN 1 AND 5),
    score_grounding  INTEGER    CHECK (score_grounding BETWEEN 1 AND 5),
    judged_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    judge_model     VARCHAR(64) NOT NULL DEFAULT 'claude-haiku-4-5-20251001',

    CONSTRAINT uq_interaction_quality_interaction UNIQUE (interaction_id)
);

CREATE INDEX idx_interaction_quality_judged_at ON interaction_quality (judged_at DESC);
