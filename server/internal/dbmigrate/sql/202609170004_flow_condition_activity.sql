-- Conditions can stop a Flow before an agent is selected or an action runs.
ALTER TABLE agent_trigger_executions ALTER COLUMN agent_id DROP NOT NULL;
ALTER TABLE agent_trigger_executions ADD COLUMN IF NOT EXISTS condition_outcome text;
ALTER TABLE agent_trigger_executions ADD COLUMN IF NOT EXISTS condition_assessment_id uuid;
