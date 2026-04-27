-- Post-backfill safety net: fail fast if any new row ships with
-- the sentinel '~' or invalid characters. Only apply AFTER backfill
-- has run and been verified (all rows match ^[a-z]+$).
--
-- DO NOT apply this migration until:
--   1. The backfill command has completed successfully
--   2. SELECT count(*) FROM docs_documents WHERE sort_key = '~' returns 0
--   3. SELECT count(*) FROM docs_collections WHERE sort_key = '~' returns 0
--   4. DOCS_ORDERING_USE_SORT_KEY has been true in production for >= 1 week

ALTER TABLE docs_documents
  ADD CONSTRAINT IF NOT EXISTS docs_documents_sort_key_valid
  CHECK (sort_key ~ '^[a-z]+$');

ALTER TABLE docs_collections
  ADD CONSTRAINT IF NOT EXISTS docs_collections_sort_key_valid
  CHECK (sort_key ~ '^[a-z]+$');
