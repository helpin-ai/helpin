-- Add persistent target_path column to docs_redirects so redirect
-- resolution can use PublicID-backed canonical paths instead of
-- depending on slug uniqueness.
ALTER TABLE docs_redirects
  ADD COLUMN IF NOT EXISTS target_path TEXT;

CREATE INDEX IF NOT EXISTS idx_docs_redirects_ws_target_path
  ON docs_redirects (workspace_id, target_path)
  WHERE target_path IS NOT NULL;

-- Backfill target_path for article redirects: join on slug to get PublicID.
-- Joins through collection to disambiguate when the same article slug
-- exists in multiple collections within the same workspace.
UPDATE docs_redirects r
SET target_path = '/articles/' || ha.slug || '-' || ha.public_id
FROM docs_helpcenter_articles ha
JOIN docs_documents d ON d.id = ha.document_id
LEFT JOIN docs_collections c ON c.id = d.collection_id AND c.deleted_at IS NULL
WHERE r.target_article_slug IS NOT NULL
  AND r.target_article_slug != ''
  AND r.target_path IS NULL
  AND ha.slug = r.target_article_slug
  AND d.workspace_id = r.workspace_id
  AND d.deleted_at IS NULL
  AND ha.public_id IS NOT NULL
  AND ha.public_id != ''
  AND (r.target_collection_slug = '' OR c.slug = r.target_collection_slug);

-- Backfill target_path for collection-only redirects (no article slug).
UPDATE docs_redirects r
SET target_path = '/c/' || c.slug || '-' || c.public_id
FROM docs_collections c
WHERE (r.target_article_slug IS NULL OR r.target_article_slug = '')
  AND r.target_collection_slug != ''
  AND r.target_path IS NULL
  AND c.slug = r.target_collection_slug
  AND c.workspace_id = r.workspace_id
  AND c.deleted_at IS NULL
  AND c.public_id IS NOT NULL
  AND c.public_id != '';
