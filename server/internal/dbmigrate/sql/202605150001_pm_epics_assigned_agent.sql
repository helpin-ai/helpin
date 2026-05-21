ALTER TABLE pm_epics
  ADD COLUMN IF NOT EXISTS assigned_agent_id uuid;

CREATE INDEX IF NOT EXISTS idx_pm_epics_assigned_agent_id
  ON pm_epics (assigned_agent_id);
