-- tl-mhd: global concept knowledge graph with Postgres ltree paths.
-- Ancestors (path @>) are prerequisites; descendants (path <@) are dependents.

BEGIN;

CREATE EXTENSION IF NOT EXISTS ltree;

CREATE TABLE IF NOT EXISTS concept_graph (
    id         SERIAL PRIMARY KEY,
    concept_id TEXT  NOT NULL UNIQUE,
    label      TEXT  NOT NULL,
    subject    TEXT  NOT NULL,
    path       LTREE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_concept_graph_path_gist
    ON concept_graph USING gist(path);

CREATE INDEX IF NOT EXISTS idx_concept_graph_subject
    ON concept_graph(subject);

COMMIT;
