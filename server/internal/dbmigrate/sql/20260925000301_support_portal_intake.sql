-- Short-lived, one-use capabilities for unverified request intake and uploads.
CREATE TABLE IF NOT EXISTS support_portal_intake_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_support_portal_intake_sessions_workspace_id ON support_portal_intake_sessions (workspace_id);
