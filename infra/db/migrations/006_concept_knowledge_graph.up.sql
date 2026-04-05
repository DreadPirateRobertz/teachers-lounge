-- TeachersLounge — Concept knowledge graph additions
-- Adds SM-2 scheduling fields and difficulty to support spaced-repetition
-- review queue and prerequisite gap detection.

BEGIN;

-- Add difficulty column to concepts (0.0 = easy, 1.0 = very hard)
ALTER TABLE concepts
    ADD COLUMN IF NOT EXISTS difficulty FLOAT NOT NULL DEFAULT 0.5
        CHECK (difficulty BETWEEN 0 AND 1);

-- Add SM-2 scheduling fields to student_concept_mastery
ALTER TABLE student_concept_mastery
    ADD COLUMN IF NOT EXISTS ease_factor   FLOAT NOT NULL DEFAULT 2.5,
    ADD COLUMN IF NOT EXISTS interval_days INT   NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS repetitions   INT   NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS review_count  INT   NOT NULL DEFAULT 0;

-- Index for efficient prerequisite chain walks via ltree
CREATE INDEX IF NOT EXISTS idx_concepts_path_gist ON concepts USING gist(path::ltree);

-- Index for prerequisite edge lookups
CREATE INDEX IF NOT EXISTS idx_concept_prereqs_prereq_id
    ON concept_prerequisites(prerequisite_id);

COMMIT;
