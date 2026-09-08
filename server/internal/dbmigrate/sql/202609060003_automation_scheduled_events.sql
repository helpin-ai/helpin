-- Shared product-event delivery. No Flow is enabled and no Agent is started.
CREATE TABLE IF NOT EXISTS automation_scheduled_events (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    event_key text NOT NULL CHECK (length(event_key) BETWEEN 1 AND 200),
    kind text NOT NULL CHECK (length(kind) BETWEEN 1 AND 100),
    target_type text NOT NULL CHECK (length(target_type) BETWEEN 1 AND 100),
    target_id uuid NOT NULL,
    expected_revision bigint NOT NULL CHECK (expected_revision > 0),
    due_at timestamptz NOT NULL,
    available_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('scheduled', 'processing', 'delivered', 'cancelled', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts integer NOT NULL DEFAULT 5 CHECK (max_attempts BETWEEN 1 AND 10),
    lease_token uuid,
    lease_until timestamptz,
    result_code text NOT NULL DEFAULT '' CHECK (length(result_code) <= 100),
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, event_key),
    CHECK (attempts <= max_attempts),
    CHECK ((status = 'processing' AND lease_token IS NOT NULL AND lease_until IS NOT NULL)
        OR (status <> 'processing' AND lease_token IS NULL AND lease_until IS NULL)),
    CHECK ((status IN ('delivered', 'cancelled', 'failed') AND completed_at IS NOT NULL)
        OR (status IN ('scheduled', 'processing') AND completed_at IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_automation_events_due
    ON automation_scheduled_events(available_at, due_at, id) WHERE status = 'scheduled';
CREATE INDEX IF NOT EXISTS idx_automation_events_recover
    ON automation_scheduled_events(lease_until, id) WHERE status = 'processing';
CREATE INDEX IF NOT EXISTS idx_automation_events_target
    ON automation_scheduled_events(workspace_id, kind, target_type, target_id, expected_revision);

-- Recover only explicit, currently open checkpoints. These events reevaluate
-- existing commitments; they do not replay evidence or perform external actions.
INSERT INTO automation_scheduled_events
    (id, workspace_id, event_key, kind, target_type, target_id, expected_revision,
     due_at, available_at, status)
SELECT gen_random_uuid(), workspace_id, 'crm.checkpoint:' || id::text || ':' || revision::text,
    'crm.checkpoint_due', 'crm_situation', id, revision, next_checkpoint_at, next_checkpoint_at, 'scheduled'
FROM crm_situations
WHERE lifecycle = 'open' AND next_checkpoint_at IS NOT NULL
ON CONFLICT (workspace_id, event_key) DO NOTHING;
