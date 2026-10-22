-- Opaque one-time customer login links and revocable portal sessions.
CREATE TABLE IF NOT EXISTS support_portal_magic_links (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 workspace_id UUID NOT NULL,
 email TEXT NOT NULL,
 token_hash TEXT NOT NULL UNIQUE,
 expires_at TIMESTAMPTZ NOT NULL,
 used_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_support_portal_magic_links_workspace_id ON support_portal_magic_links (workspace_id);
CREATE TABLE IF NOT EXISTS support_portal_sessions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 workspace_id UUID NOT NULL,
 identity_id UUID NOT NULL,
 token_hash TEXT NOT NULL UNIQUE,
 expires_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_support_portal_sessions_workspace_id ON support_portal_sessions (workspace_id);
