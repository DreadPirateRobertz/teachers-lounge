-- Rollback: drop review_records table (tl-vw6)

BEGIN;

DROP TABLE IF EXISTS review_records CASCADE;

COMMIT;
