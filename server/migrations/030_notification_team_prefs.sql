-- Add team_id to notification_preferences for per-team overrides
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS team_id UUID REFERENCES workspace_teams(id) ON DELETE CASCADE;

-- Drop old unique index and create new one that includes team_id
DROP INDEX IF EXISTS uq_notification_pref_user_workspace;
CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_pref_user_workspace_team
    ON notification_preferences (user_id, workspace_id, COALESCE(team_id, '00000000-0000-0000-0000-000000000000'));
