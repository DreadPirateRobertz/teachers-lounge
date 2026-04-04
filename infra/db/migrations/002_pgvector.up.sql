-- TeachersLounge — pgvector columns
-- Requires pgvector extension on Cloud SQL Postgres 15+
-- Run after 001_initial_schema.sql

BEGIN;

CREATE EXTENSION IF NOT EXISTS vector;

-- Embeddings on interactions for semantic search over student history
ALTER TABLE interactions
    ADD COLUMN embedding vector(1024);

CREATE INDEX idx_interactions_embedding ON interactions
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- Embeddings on learning_profiles (aggregated student "voice" embedding)
ALTER TABLE learning_profiles
    ADD COLUMN history_embedding vector(1024);

COMMIT;
