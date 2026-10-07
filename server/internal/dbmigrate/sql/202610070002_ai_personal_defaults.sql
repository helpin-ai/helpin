-- Personal preferences do not alter shared defaults or accepted run snapshots.
CREATE TABLE IF NOT EXISTS ai_personal_settings (
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    default_profile_id uuid REFERENCES ai_profiles(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_ai_personal_settings_default_profile ON ai_personal_settings (default_profile_id);
