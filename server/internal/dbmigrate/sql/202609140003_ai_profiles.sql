CREATE TABLE IF NOT EXISTS ai_profiles (
 id UUID PRIMARY KEY,
 workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 user_id UUID REFERENCES users(id) ON DELETE CASCADE,
 scope TEXT NOT NULL,
 name TEXT NOT NULL,
 revision BIGINT NOT NULL CHECK (revision > 0),
 "primary" JSONB NOT NULL,
 fallback JSONB,
 deleted_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CONSTRAINT ai_profiles_scope_owner CHECK ((scope='personal' AND user_id IS NOT NULL) OR (scope='workspace' AND user_id IS NULL))
);
CREATE INDEX IF NOT EXISTS idx_ai_profiles_workspace_owner ON ai_profiles(workspace_id,user_id) WHERE deleted_at IS NULL;
CREATE TABLE IF NOT EXISTS ai_workspace_settings (
 workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
 default_profile_id UUID REFERENCES ai_profiles(id),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
