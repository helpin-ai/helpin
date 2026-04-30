-- Post-backfill safety net: fail fast if any new row ships with
-- the sentinel '~' or invalid characters. Only apply AFTER backfill
-- has run and been verified (all rows match ^[a-z]+$).
--
-- DO NOT apply this migration until:
--   1. The backfill command has completed successfully
--   2. SELECT count(*) FROM docs_documents WHERE sort_key = '~' returns 0
--   3. SELECT count(*) FROM docs_collections WHERE sort_key = '~' returns 0
--   4. DOCS_ORDERING_USE_SORT_KEY has been true in production for >= 1 week
--
-- Intentionally a marker for now. Add the actual constraints in a future
-- migration after the backfill and rollout checks above are complete.

SELECT 1;
