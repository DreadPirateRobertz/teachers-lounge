-- Rollback: remove SM-2 fields and difficulty from concept tables

BEGIN;

DROP INDEX IF EXISTS idx_concept_prereqs_prereq_id;
DROP INDEX IF EXISTS idx_concepts_path_gist;

ALTER TABLE student_concept_mastery
    DROP COLUMN IF EXISTS review_count,
    DROP COLUMN IF EXISTS repetitions,
    DROP COLUMN IF EXISTS interval_days,
    DROP COLUMN IF EXISTS ease_factor;

ALTER TABLE concepts
    DROP COLUMN IF EXISTS difficulty;

COMMIT;
