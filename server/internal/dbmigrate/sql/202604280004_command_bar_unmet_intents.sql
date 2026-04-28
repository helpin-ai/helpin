CREATE TABLE IF NOT EXISTS command_bar_unmet_intents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  actor_id UUID,
  prompt TEXT NOT NULL,
  page_context JSONB NOT NULL DEFAULT '{}'::jsonb,
  candidate_agents JSONB NOT NULL DEFAULT '[]'::jsonb,
  reason TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_command_bar_unmet_intents_workspace_created
  ON command_bar_unmet_intents (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_bar_unmet_intents_workspace_actor_created
  ON command_bar_unmet_intents (workspace_id, actor_id, created_at DESC);
