BEGIN;

-- ============================================================
-- BOSS TAUNTS
-- Pool of AI-generated taunts per boss per round.
-- The gaming service picks a random row on each wrong answer;
-- new taunts are generated via LiteLLM (Claude Haiku) and
-- appended to grow the pool over time.
-- ============================================================
CREATE TABLE boss_taunts (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    boss_id    TEXT        NOT NULL,
    round      INT         NOT NULL CHECK (round >= 1),
    taunt      TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_boss_taunts_boss_round ON boss_taunts (boss_id, round);

COMMIT;
