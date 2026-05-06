ALTER TABLE pm_task_templates
  ADD COLUMN IF NOT EXISTS workflow_state_id uuid;

CREATE INDEX IF NOT EXISTS idx_pm_task_templates_workflow_state_id
  ON pm_task_templates (workflow_state_id);
