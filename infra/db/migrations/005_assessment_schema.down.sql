-- Rollback: drop learning style assessment tables.

BEGIN;

DROP TABLE IF EXISTS assessment_answers  CASCADE;
DROP TABLE IF EXISTS assessment_sessions CASCADE;

COMMIT;
