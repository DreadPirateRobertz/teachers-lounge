-- Revert Phase 8 performance indexes.
-- Note: DROP INDEX CONCURRENTLY cannot run inside a transaction block;
-- use plain DROP INDEX instead.

DROP INDEX IF EXISTS idx_subscriptions_user_status;
DROP INDEX IF EXISTS idx_interactions_session_created;
DROP INDEX IF EXISTS idx_quiz_results_user_created;
