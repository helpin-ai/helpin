ALTER TABLE IF EXISTS docs_import_jobs
  ADD COLUMN IF NOT EXISTS payload_encrypted TEXT;

ALTER TABLE IF EXISTS docs_import_jobs
  ADD COLUMN IF NOT EXISTS workflow_id TEXT;

CREATE INDEX IF NOT EXISTS idx_docs_import_jobs_workflow_id
  ON docs_import_jobs (workflow_id);
