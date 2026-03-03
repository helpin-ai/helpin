-- 015_pm_activity_log.sql
-- PM activity log

CREATE TABLE pm_activity_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    field_name TEXT,
    old_value TEXT,
    new_value TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pm_activity_entity ON pm_activity_log(entity_type, entity_id);
CREATE INDEX idx_pm_activity_workspace_created ON pm_activity_log(workspace_id, created_at DESC);
CREATE INDEX idx_pm_activity_actor ON pm_activity_log(actor_id);
