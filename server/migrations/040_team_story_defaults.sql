ALTER TABLE workspace_teams
  ADD COLUMN IF NOT EXISTS team_type TEXT NOT NULL DEFAULT 'engineering',
  ADD COLUMN IF NOT EXISTS default_story_type TEXT NOT NULL DEFAULT 'feature';
