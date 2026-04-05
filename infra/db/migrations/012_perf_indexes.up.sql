-- Phase 8 performance indexes.
-- Addresses query patterns identified during load testing.

BEGIN;

-- user-service: subscription status lookups are a hot path on every
-- authenticated request. The existing UNIQUE(user_id) constraint provides
-- equality on user_id but not a composite covering status.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscriptions_user_status
    ON subscriptions(user_id, status);

-- tutoring-service: interaction history for a session is loaded on every
-- chat request. The existing separate indexes on session_id and created_at
-- are not used together. A composite covering index eliminates the sort.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_interactions_session_created
    ON interactions(session_id, created_at DESC);

-- analytics-service: student overview queries filter by user_id and sort
-- by recency. A composite on (user_id, created_at DESC) covers both clauses.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_quiz_results_user_created
    ON quiz_results(user_id, created_at DESC);

COMMIT;
