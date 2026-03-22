-- Unified redirects table (replaces docs_slug_aliases)
CREATE TABLE IF NOT EXISTS docs_redirects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  source_path TEXT NOT NULL,
  target_collection_slug TEXT NOT NULL,
  target_article_slug TEXT,
  type TEXT NOT NULL DEFAULT 'manual',
  source_system TEXT,
  source_object_type TEXT,
  source_object_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_redirects_ws_source
  ON docs_redirects (workspace_id, source_path);
CREATE INDEX IF NOT EXISTS idx_docs_redirects_ws
  ON docs_redirects (workspace_id);

-- Migrate existing slug aliases to redirects
INSERT INTO docs_redirects (workspace_id, source_path, target_collection_slug, target_article_slug, type)
  SELECT sa.workspace_id, sa.old_slug, COALESCE(c.slug, ''), ha.slug, 'slug_change'
  FROM docs_slug_aliases sa
  JOIN docs_helpcenter_articles ha ON ha.document_id = sa.document_id
  JOIN docs_documents d ON d.id = sa.document_id
  LEFT JOIN docs_collections c ON c.id = d.collection_id
  ON CONFLICT DO NOTHING;

-- Update collection slug uniqueness to workspace-level
DROP INDEX IF EXISTS idx_docs_collection_space_slug;
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collection_ws_slug
  ON docs_collections (workspace_id, slug) WHERE slug != '' AND deleted_at IS NULL;
