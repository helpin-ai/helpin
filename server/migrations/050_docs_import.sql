-- Import job tracking
CREATE TABLE IF NOT EXISTS docs_import_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  space_id UUID,
  source TEXT NOT NULL DEFAULT 'helpscout',
  status TEXT NOT NULL DEFAULT 'pending',
  total INT NOT NULL DEFAULT 0,
  completed INT NOT NULL DEFAULT 0,
  failed INT NOT NULL DEFAULT 0,
  failures JSONB NOT NULL DEFAULT '[]',
  config JSONB NOT NULL DEFAULT '{}',
  redirect_map JSONB,
  error TEXT,
  started_by UUID NOT NULL,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_docs_import_jobs_ws ON docs_import_jobs (workspace_id);

-- Add slug to docs_collections
ALTER TABLE docs_collections ADD COLUMN IF NOT EXISTS slug TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_docs_collection_space_slug
  ON docs_collections (space_id, slug) WHERE slug != '' AND deleted_at IS NULL;
