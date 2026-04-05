-- Rollback: tl-vw6 learning profile tables

BEGIN;

DROP TABLE IF EXISTS misconceptions;
DROP TABLE IF EXISTS explanation_preferences;
DROP TABLE IF EXISTS learning_profiles;

COMMIT;
