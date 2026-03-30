-- XP event audit log for the award pipeline.
-- Tracks every XP award with event source, multiplier, and cap status.

BEGIN;

CREATE TABLE xp_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    base_xp     BIGINT NOT NULL,
    multiplier  DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    awarded_xp  BIGINT NOT NULL,
    capped      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_xp_events_user_id ON xp_events(user_id);
CREATE INDEX idx_xp_events_created_at ON xp_events(created_at DESC);
CREATE INDEX idx_xp_events_user_date ON xp_events(user_id, created_at DESC);

ALTER TABLE xp_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY student_isolation ON xp_events
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

COMMIT;
