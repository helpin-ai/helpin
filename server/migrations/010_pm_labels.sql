-- 010_pm_labels.sql
-- PM labels

CREATE TABLE pm_labels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    color TEXT,
    archived BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

CREATE INDEX idx_pm_labels_workspace ON pm_labels(workspace_id);
CREATE INDEX idx_pm_labels_archived ON pm_labels(workspace_id, archived);

CREATE TRIGGER trg_pm_labels_updated_at
    BEFORE UPDATE ON pm_labels
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
