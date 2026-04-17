-- Rollback: drop flashcards table (tl-y3v)

BEGIN;

DROP TABLE IF EXISTS flashcards CASCADE;

COMMIT;
