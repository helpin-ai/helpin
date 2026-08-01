-- Track delegated Agent Runtime runs without changing existing Helpin execution paths.

ALTER TABLE agent_runs
  ADD COLUMN IF NOT EXISTS external_runtime TEXT,
  ADD COLUMN IF NOT EXISTS external_runtime_id TEXT;

DROP INDEX IF EXISTS idx_agent_runs_external_runtime;
DROP INDEX IF EXISTS idx_agent_runs_external_runtime_id;
DROP INDEX IF EXISTS idx_agent_runs_external_runtime_pair;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_runs_external_runtime_pair
  ON agent_runs (external_runtime, external_runtime_id)
  WHERE external_runtime_id IS NOT NULL;
