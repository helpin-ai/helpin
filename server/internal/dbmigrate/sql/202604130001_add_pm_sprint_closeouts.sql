CREATE TABLE IF NOT EXISTS pm_sprint_closeouts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sprint_id UUID NOT NULL UNIQUE REFERENCES pm_sprints(id) ON DELETE CASCADE,
  workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  team_id UUID NULL REFERENCES workspace_teams(id) ON DELETE SET NULL,
  rolled_to_sprint_id UUID NULL REFERENCES pm_sprints(id) ON DELETE SET NULL,
  committed_count INTEGER NOT NULL DEFAULT 0,
  completed_count INTEGER NOT NULL DEFAULT 0,
  unfinished_count INTEGER NOT NULL DEFAULT 0,
  rolled_over_count INTEGER NOT NULL DEFAULT 0,
  committed_points INTEGER NOT NULL DEFAULT 0,
  completed_points INTEGER NOT NULL DEFAULT 0,
  unfinished_points INTEGER NOT NULL DEFAULT 0,
  rolled_over_points INTEGER NOT NULL DEFAULT 0,
  closed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pm_sprint_closeout_tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  closeout_id UUID NOT NULL REFERENCES pm_sprint_closeouts(id) ON DELETE CASCADE,
  task_id UUID NOT NULL REFERENCES pm_tasks(id) ON DELETE CASCADE,
  outcome TEXT NOT NULL CHECK (outcome IN ('completed', 'unfinished_not_rolled', 'rolled_over')),
  estimate INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (closeout_id, task_id)
);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeouts_workspace_closed_at
  ON pm_sprint_closeouts(workspace_id, closed_at DESC);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeouts_team_closed_at
  ON pm_sprint_closeouts(team_id, closed_at DESC);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeouts_rolled_to
  ON pm_sprint_closeouts(rolled_to_sprint_id);

CREATE INDEX IF NOT EXISTS idx_pm_sprint_closeout_tasks_closeout
  ON pm_sprint_closeout_tasks(closeout_id);
