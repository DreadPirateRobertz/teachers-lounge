-- tl-mhd: rollback of 020_concept_graph.

BEGIN;

DROP INDEX IF EXISTS idx_concept_graph_subject;
DROP INDEX IF EXISTS idx_concept_graph_path_gist;
DROP TABLE IF EXISTS concept_graph;

COMMIT;
