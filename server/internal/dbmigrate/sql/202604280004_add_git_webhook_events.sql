CREATE TABLE IF NOT EXISTS git_webhook_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider varchar(30) NOT NULL,
    event_type varchar(60) NOT NULL,
    delivery_id varchar(120),
    integration_id uuid,
    workspace_id uuid,
    repository_full_name varchar(255),
    action varchar(80),
    status varchar(30) NOT NULL DEFAULT 'received',
    status_code integer,
    error_message text,
    raw_payload jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_git_webhook_events_provider ON git_webhook_events(provider);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_event_type ON git_webhook_events(event_type);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_delivery_id ON git_webhook_events(delivery_id);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_integration_id ON git_webhook_events(integration_id);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_workspace_id ON git_webhook_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_repository_full_name ON git_webhook_events(repository_full_name);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_action ON git_webhook_events(action);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_status ON git_webhook_events(status);
CREATE INDEX IF NOT EXISTS idx_git_webhook_events_received_at ON git_webhook_events(received_at DESC);
