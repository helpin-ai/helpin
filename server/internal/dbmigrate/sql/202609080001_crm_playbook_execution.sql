-- Publication stays immutable and disabled. Explicit live gates are separate.
CREATE TABLE IF NOT EXISTS crm_playbook_automation_settings (
    workspace_id uuid NOT NULL,
    playbook_id uuid NOT NULL,
    connection_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    enabled boolean NOT NULL DEFAULT false,
    entry_mode text NOT NULL CHECK (entry_mode IN ('manual', 'automatic')),
    automatic_since timestamptz,
    max_runs_per_day integer NOT NULL CHECK (max_runs_per_day BETWEEN 1 AND 24),
    max_no_progress_runs integer NOT NULL CHECK (max_no_progress_runs BETWEEN 1 AND 10),
    authorized_by_member_id uuid NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, playbook_id),
    FOREIGN KEY (workspace_id, playbook_id) REFERENCES crm_playbooks(workspace_id, id),
    FOREIGN KEY (workspace_id, playbook_id, connection_id) REFERENCES crm_playbook_connections(workspace_id, playbook_id, id)
);

CREATE TABLE IF NOT EXISTS crm_playbook_automation_bindings (
    workspace_id uuid NOT NULL,
    situation_id uuid NOT NULL,
    playbook_id uuid NOT NULL,
    connection_id uuid NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    generation bigint NOT NULL CHECK (generation > 0),
    authorized_by_member_id uuid NOT NULL,
    last_run_id uuid,
    last_checked_at timestamptz,
    no_progress_runs integer NOT NULL DEFAULT 0 CHECK (no_progress_runs >= 0),
    context_fingerprint text NOT NULL DEFAULT '',
    action_fingerprint text NOT NULL DEFAULT '',
    progress_observed_at timestamptz,
    escalated_at timestamptz,
    last_maintenance_at timestamptz,
    blocker text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, situation_id),
    FOREIGN KEY (workspace_id, situation_id) REFERENCES crm_situations(workspace_id, id),
    FOREIGN KEY (workspace_id, playbook_id) REFERENCES crm_playbooks(workspace_id, id),
    FOREIGN KEY (workspace_id, playbook_id, connection_id) REFERENCES crm_playbook_connections(workspace_id, playbook_id, id)
);

CREATE TABLE IF NOT EXISTS automation_run_bindings (
    run_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    event_id uuid NOT NULL,
    situation_id uuid NOT NULL,
    connection_id uuid NOT NULL,
    generation bigint NOT NULL CHECK (generation > 0),
    situation_revision bigint NOT NULL CHECK (situation_revision > 0),
    agent_id uuid NOT NULL,
    runtime_profile_id uuid NOT NULL,
    input jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    observed_terminal_at timestamptz,
    last_maintenance_at timestamptz,
    UNIQUE (workspace_id, event_id),
    FOREIGN KEY (workspace_id, situation_id) REFERENCES crm_situations(workspace_id, id)
);
CREATE INDEX IF NOT EXISTS automation_run_bindings_situation
    ON automation_run_bindings (workspace_id, situation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS crm_playbook_automation_receipts (
    workspace_id uuid NOT NULL,
    subject_id uuid NOT NULL,
    command_key text NOT NULL,
    fingerprint text NOT NULL,
    settings jsonb,
    binding jsonb,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, subject_id, command_key),
    CHECK ((settings IS NULL) <> (binding IS NULL))
);
