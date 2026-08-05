CREATE TABLE IF NOT EXISTS dock_action_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    dock_chat_run_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    summary TEXT NOT NULL,
    spec JSONB NOT NULL DEFAULT '{}'::jsonb,
    usage JSONB NOT NULL DEFAULT '{}'::jsonb,
    approval_interaction_id UUID,
    expires_at TIMESTAMPTZ NOT NULL,
    activated_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dock_action_proposals_workspace_id ON dock_action_proposals(workspace_id);
CREATE INDEX IF NOT EXISTS idx_dock_action_proposals_dock_chat_run_id ON dock_action_proposals(dock_chat_run_id);
CREATE INDEX IF NOT EXISTS idx_dock_action_proposals_actor_id ON dock_action_proposals(actor_id);
CREATE INDEX IF NOT EXISTS idx_dock_action_proposals_status ON dock_action_proposals(status);
CREATE INDEX IF NOT EXISTS idx_dock_action_proposals_expires_at ON dock_action_proposals(expires_at);
CREATE INDEX IF NOT EXISTS idx_dock_action_proposals_approval_interaction_id ON dock_action_proposals(approval_interaction_id);
