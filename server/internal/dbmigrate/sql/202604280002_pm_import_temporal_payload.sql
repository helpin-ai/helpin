ALTER TABLE IF EXISTS pm_import_jobs
  ADD COLUMN IF NOT EXISTS payload_encrypted text;

ALTER TABLE IF EXISTS pm_import_jobs
  ADD COLUMN IF NOT EXISTS workflow_id text;

CREATE INDEX IF NOT EXISTS idx_pm_import_jobs_workflow_id
  ON pm_import_jobs (workflow_id);
