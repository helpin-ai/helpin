-- Backfill docs_redirects.target_path after the schema column exists.
-- This migration must stay separate from 202604140001 because that file
-- was already applied in production before the backfill logic was added.

-- Backfill target_path for article redirects: join on slug to get PublicID.
-- Only fills rows that have a target_article_slug and no target_path yet.
UPDATE docs_redirects r
SET target_path = '/articles/' || ha.slug || '-' || ha.public_id
FROM docs_helpcenter_articles ha
JOIN docs_documents d ON d.id = ha.document_id
WHERE r.target_article_slug IS NOT NULL
  AND r.target_article_slug != ''
  AND r.target_path IS NULL
  AND ha.slug = r.target_article_slug
  AND d.workspace_id = r.workspace_id
  AND d.deleted_at IS NULL
  AND ha.public_id IS NOT NULL
  AND ha.public_id != '';

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
