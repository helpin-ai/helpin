-- Add persistent target_path column to docs_redirects so redirect
-- resolution can use PublicID-backed canonical paths instead of
-- depending on slug uniqueness.
ALTER TABLE docs_redirects
  ADD COLUMN IF NOT EXISTS target_path TEXT;

CREATE INDEX IF NOT EXISTS idx_docs_redirects_ws_target_path
  ON docs_redirects (workspace_id, target_path)
  WHERE target_path IS NOT NULL;
