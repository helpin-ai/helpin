-- Approved configuration only: no Flow/Agent edits, enrollment or execution.
CREATE TABLE IF NOT EXISTS crm_playbook_connections (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    playbook_id uuid NOT NULL,
    playbook_version_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    command_key text NOT NULL CHECK (length(command_key) BETWEEN 1 AND 200),
    command_fingerprint text NOT NULL,
    fingerprint text NOT NULL CHECK (length(fingerprint) = 64),
    snapshot jsonb NOT NULL,
    execution_enabled boolean NOT NULL DEFAULT false CHECK (execution_enabled = false),
    published_by_member_id uuid NOT NULL,
    published_at timestamptz NOT NULL,
    UNIQUE (workspace_id, playbook_id, id),
    UNIQUE (workspace_id, playbook_id, version),
    UNIQUE (workspace_id, playbook_id, command_key),
    FOREIGN KEY (workspace_id, playbook_id, playbook_version_id)
        REFERENCES crm_playbook_versions(workspace_id, playbook_id, id) ON DELETE CASCADE
);
