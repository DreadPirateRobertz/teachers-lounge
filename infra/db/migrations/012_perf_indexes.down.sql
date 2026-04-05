-- Revert Phase 8 performance indexes.

BEGIN;

DROP INDEX CONCURRENTLY IF EXISTS idx_subscriptions_user_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_interactions_session_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_quiz_results_user_created;

COMMIT;
