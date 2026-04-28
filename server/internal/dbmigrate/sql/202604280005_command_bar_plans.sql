CREATE TABLE IF NOT EXISTS command_bar_plans (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  status text NOT NULL DEFAULT 'running',
  prompt text NOT NULL,
  page_context jsonb NOT NULL DEFAULT '{}'::jsonb,
  steps jsonb NOT NULL DEFAULT '[]'::jsonb,
  run_ids_by_step jsonb NOT NULL DEFAULT '{}'::jsonb,
  current_step_index integer NOT NULL DEFAULT 0,
  run_count integer NOT NULL DEFAULT 0,
  error_message text,
  cancelled_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_command_bar_plans_workspace_created
  ON command_bar_plans (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_command_bar_plans_workspace_status_created
  ON command_bar_plans (workspace_id, status, created_at DESC);
