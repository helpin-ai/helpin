-- Personal dock shortcuts to chats and runs (model.DockPin). AutoMigrate
-- created this table on hosted deployments; installations that apply only
-- versioned SQL need it as well.

CREATE TABLE IF NOT EXISTS dock_pins (
    workspace_id uuid NOT NULL,
    user_id uuid NOT NULL,
    kind text NOT NULL,
    resource_id uuid NOT NULL,
    created_at timestamptz,
    PRIMARY KEY (workspace_id, user_id, kind, resource_id)
);
