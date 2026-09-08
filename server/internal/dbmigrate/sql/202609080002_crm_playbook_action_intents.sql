-- Canonical suggestions retain the sole decision/execution lifecycle.
CREATE TABLE IF NOT EXISTS crm_playbook_action_intents (
    suggestion_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    situation_id uuid NOT NULL,
    run_id uuid NOT NULL,
    connection_id uuid NOT NULL,
    generation bigint NOT NULL CHECK (generation > 0),
    situation_revision bigint NOT NULL CHECK (situation_revision > 0),
    intent_key text NOT NULL,
    recipient_key text NOT NULL DEFAULT '',
    context_fingerprint text NOT NULL,
    approver_member_id uuid NOT NULL,
    expires_at timestamptz NOT NULL,
    approved_by_member_id uuid,
    approved_at timestamptz,
    approved_fingerprint text NOT NULL DEFAULT '',
    result_type text NOT NULL DEFAULT '',
    result_id uuid,
    created_at timestamptz NOT NULL,
    UNIQUE (workspace_id, intent_key),
    FOREIGN KEY (workspace_id, situation_id) REFERENCES crm_situations(workspace_id, id),
    FOREIGN KEY (suggestion_id) REFERENCES crm_suggestions(id),
    FOREIGN KEY (run_id) REFERENCES automation_run_bindings(run_id),
    FOREIGN KEY (connection_id) REFERENCES crm_playbook_connections(id),
    CHECK ((approved_at IS NULL AND approved_by_member_id IS NULL AND approved_fingerprint = '')
        OR (approved_at IS NOT NULL AND approved_by_member_id IS NOT NULL AND approved_fingerprint <> ''))
);
CREATE INDEX IF NOT EXISTS crm_playbook_action_intents_process ON crm_playbook_action_intents(workspace_id, situation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS crm_playbook_action_intents_recipient ON crm_playbook_action_intents(workspace_id, recipient_key, approved_at DESC);
