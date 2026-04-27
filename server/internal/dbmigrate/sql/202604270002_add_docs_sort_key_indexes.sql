-- Bucket-scoped read indexes for fractional sort_key ordering.
-- No WHERE clause on deleted_at — indexing all rows avoids bloat
-- from soft-delete/restore churn at current volumes.
--
-- These indexes support the canonical ORDER BY: sort_key ASC, id ASC
-- scoped by (workspace_id, space_id, collection_id/parent_collection_id).

CREATE INDEX IF NOT EXISTS idx_docs_documents_bucket_sort
  ON docs_documents (workspace_id, space_id, collection_id, sort_key, id);

CREATE INDEX IF NOT EXISTS idx_docs_collections_bucket_sort
  ON docs_collections (workspace_id, space_id, parent_collection_id, sort_key, id);
