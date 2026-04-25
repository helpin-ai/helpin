ALTER TABLE pm_team_repo_defaults
  ADD COLUMN IF NOT EXISTS closed_state_id UUID;
