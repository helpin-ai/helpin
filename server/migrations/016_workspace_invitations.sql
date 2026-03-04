-- 016_workspace_invitations.sql
-- Reference migration for workspace invitations and workspace_people.user_id link.
-- GORM AutoMigrate handles the actual schema changes.

CREATE TABLE IF NOT EXISTS workspace_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    token TEXT NOT NULL UNIQUE,
    invited_by UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workspace_invitations_workspace_id ON workspace_invitations(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_invitations_token ON workspace_invitations(token);

-- Link workspace_people to users
ALTER TABLE workspace_people ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id);
