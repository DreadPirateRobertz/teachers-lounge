-- Rollback: drop concept_review_schedule table (tl-5wz)

BEGIN;

DROP TABLE IF EXISTS concept_review_schedule CASCADE;

COMMIT;
