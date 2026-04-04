-- Rollback 001_initial_schema.up.sql
-- Drop in reverse dependency order. CASCADE handles FK children.

BEGIN;

-- Triggers
DROP TRIGGER IF EXISTS trg_gaming_profiles_updated_at ON gaming_profiles;
DROP TRIGGER IF EXISTS trg_learning_profiles_updated_at ON learning_profiles;
DROP TRIGGER IF EXISTS trg_courses_updated_at ON courses;
DROP TRIGGER IF EXISTS trg_subscriptions_updated_at ON subscriptions;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP FUNCTION IF EXISTS set_updated_at();

-- Tables (leaf → root; CASCADE handles remaining FK refs)
DROP TABLE IF EXISTS scifi_quotes;
DROP TABLE IF EXISTS export_jobs;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS student_concept_mastery;
DROP TABLE IF EXISTS concept_prerequisites;
DROP TABLE IF EXISTS concepts;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS boss_history;
DROP TABLE IF EXISTS achievements;
DROP TABLE IF EXISTS gaming_profiles;
DROP TABLE IF EXISTS quiz_results;
DROP TABLE IF EXISTS interactions;
DROP TABLE IF EXISTS materials;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS learning_profiles;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS auth_tokens;
DROP TABLE IF EXISTS users;

-- Enum types
DROP TYPE IF EXISTS export_status;
DROP TYPE IF EXISTS processing_status;
DROP TYPE IF EXISTS boss_result;
DROP TYPE IF EXISTS content_type;
DROP TYPE IF EXISTS interaction_role;
DROP TYPE IF EXISTS subscription_status;
DROP TYPE IF EXISTS subscription_plan;
DROP TYPE IF EXISTS account_type;

-- Extensions
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS ltree;
DROP EXTENSION IF EXISTS "uuid-ossp";

COMMIT;
