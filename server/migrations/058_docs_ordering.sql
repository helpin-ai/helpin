-- 058: Add canonical ordering for docs documents and normalize existing positions.

-- Add position column to docs_documents.
ALTER TABLE docs_documents
  ADD COLUMN IF NOT EXISTS position integer NOT NULL DEFAULT 0;

-- Add composite index for efficient ordering queries.
CREATE INDEX IF NOT EXISTS idx_docs_doc_space_collection_position
  ON docs_documents (space_id, collection_id, position);

-- Backfill docs_spaces with contiguous positions per (workspace_id, type).
WITH ranked_spaces AS (
  SELECT id,
         row_number() OVER (
           PARTITION BY workspace_id, type
           ORDER BY position ASC, created_at ASC, id ASC
         ) - 1 AS normalized_position
  FROM docs_spaces
  WHERE deleted_at IS NULL
)
UPDATE docs_spaces s
SET position = r.normalized_position
FROM ranked_spaces r
WHERE r.id = s.id;

-- Backfill docs_collections with contiguous positions per space_id.
WITH ranked_collections AS (
  SELECT id,
         row_number() OVER (
           PARTITION BY space_id
           ORDER BY position ASC, created_at ASC, id ASC
         ) - 1 AS normalized_position
  FROM docs_collections
  WHERE deleted_at IS NULL
)
UPDATE docs_collections c
SET position = r.normalized_position
FROM ranked_collections r
WHERE r.id = c.id;

-- Backfill docs_documents with contiguous positions per (space_id, collection_id).
WITH ranked_docs AS (
  SELECT id,
         row_number() OVER (
           PARTITION BY space_id, collection_id
           ORDER BY created_at ASC, id ASC
         ) - 1 AS normalized_position
  FROM docs_documents
  WHERE deleted_at IS NULL
)
UPDATE docs_documents d
SET position = r.normalized_position
FROM ranked_docs r
WHERE r.id = d.id;
