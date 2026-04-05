-- TeachersLounge — Phase 7 Compliance Schema
-- FERPA audit trail wiring + GDPR right to erasure + consent management

BEGIN;

-- ============================================================
-- CONSENT MANAGEMENT
-- ============================================================
CREATE TYPE consent_type AS ENUM ('tutoring', 'analytics', 'marketing');

-- One row per (user, consent_type). Upserted on update.
CREATE TABLE consent_records (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consent_type consent_type NOT NULL,
    granted      BOOLEAN NOT NULL DEFAULT false,
    granted_at   TIMESTAMPTZ,
    ip_address   INET,
    user_agent   TEXT,
    UNIQUE(user_id, consent_type)
);

CREATE INDEX idx_consent_records_user_id ON consent_records(user_id);

ALTER TABLE consent_records ENABLE ROW LEVEL SECURITY;
CREATE POLICY student_isolation ON consent_records
    USING (user_id = current_setting('app.current_user_id', true)::uuid);

-- ============================================================
-- ERASURE JOBS
-- Intentionally no FK to users so rows survive user deletion.
-- A background worker processes these to clean Qdrant + GCS.
-- ============================================================
CREATE TABLE erasure_jobs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID NOT NULL,  -- no FK — user is deleted before job runs
    status       export_status NOT NULL DEFAULT 'pending',
    -- JSON metadata for the worker: {"qdrant_collections": [...], "gcs_prefix": "..."}
    metadata     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_erasure_jobs_pending ON erasure_jobs(status, created_at)
    WHERE status = 'pending';

COMMIT;
