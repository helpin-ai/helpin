-- 042_agent_flexibility.sql
-- Add per-agent tool/target overrides, team ownership, scheduling, and approval mode.

ALTER TABLE agents ADD COLUMN IF NOT EXISTS team_id uuid REFERENCES workspace_teams(id) ON DELETE SET NULL;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS allowed_tools jsonb NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS allowed_commands jsonb NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS allowed_targets jsonb NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS schedule text;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS target_selector jsonb;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS trigger_events jsonb NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS approval_mode text NOT NULL DEFAULT 'class_default';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_concurrent_runs int NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_agents_team_id ON agents(team_id);
CREATE INDEX IF NOT EXISTS idx_agents_schedule ON agents(schedule) WHERE schedule IS NOT NULL;

-- Backfill allowed_targets from agent_class so existing agents keep working
-- when runtime resolution switches from class-based to per-agent.
UPDATE agents SET allowed_targets = '["story"]'
WHERE agent_class IN ('engineer', 'reviewer') AND allowed_targets = '[]';

UPDATE agents SET allowed_targets = '["epic"]'
WHERE agent_class = 'product_planner' AND allowed_targets = '[]';

UPDATE agents SET allowed_targets = '["support_conversation"]'
WHERE agent_class = 'support' AND allowed_targets = '[]';
