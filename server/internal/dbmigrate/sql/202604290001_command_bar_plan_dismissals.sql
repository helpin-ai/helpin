CREATE TABLE IF NOT EXISTS command_bar_plan_dismissals (
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  plan_id uuid NOT NULL REFERENCES command_bar_plans(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  dismissed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, plan_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_command_bar_plan_dismissals_user
  ON command_bar_plan_dismissals (workspace_id, user_id, dismissed_at DESC);
