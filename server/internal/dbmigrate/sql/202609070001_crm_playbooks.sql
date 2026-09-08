-- CRM configuration only. No automated enrollment or external execution.
CREATE TABLE IF NOT EXISTS crm_playbooks (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    creation_key text NOT NULL CHECK (length(creation_key) BETWEEN 1 AND 200),
    creation_fingerprint text NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    draft jsonb NOT NULL,
    published_version_id uuid,
    accepting_customers boolean NOT NULL DEFAULT false,
    created_by_member_id uuid NOT NULL,
    updated_by_member_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, creation_key),
    CHECK (NOT accepting_customers OR published_version_id IS NOT NULL)
);
CREATE TABLE IF NOT EXISTS crm_playbook_versions (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    playbook_id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    definition jsonb NOT NULL,
    fingerprint text NOT NULL,
    published_by_member_id uuid NOT NULL,
    published_at timestamptz NOT NULL,
    UNIQUE (workspace_id, playbook_id, id),
    UNIQUE (workspace_id, playbook_id, version),
    FOREIGN KEY (workspace_id, playbook_id) REFERENCES crm_playbooks(workspace_id, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS crm_playbook_changes (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    playbook_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    command_key text NOT NULL CHECK (length(command_key) BETWEEN 1 AND 200),
    command_fingerprint text NOT NULL,
    operation text NOT NULL CHECK (operation IN ('created', 'update_draft', 'publish', 'set_enrollment')),
    actor_member_id uuid NOT NULL,
    reason text NOT NULL DEFAULT '' CHECK (length(reason) <= 2000),
    after jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (workspace_id, playbook_id, revision),
    UNIQUE (workspace_id, playbook_id, command_key),
    FOREIGN KEY (workspace_id, playbook_id) REFERENCES crm_playbooks(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_crm_playbooks_list ON crm_playbooks(workspace_id, created_at DESC, id);

-- Progress belongs to the existing Signal, not a parallel customer process.
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS playbook_id uuid;
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS playbook_version_id uuid;
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS playbook_applied_by_member_id uuid;
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS playbook_applied_at timestamptz;
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS playbook_milestones jsonb;
CREATE INDEX IF NOT EXISTS idx_crm_situation_playbook ON crm_situations(workspace_id, playbook_id, lifecycle);

-- PostgreSQL composite constraints preserve tenant, definition and version identity.
ALTER TABLE crm_playbooks DROP CONSTRAINT IF EXISTS crm_playbook_published_version;
ALTER TABLE crm_playbooks ADD CONSTRAINT crm_playbook_published_version
    FOREIGN KEY (workspace_id, id, published_version_id) REFERENCES crm_playbook_versions(workspace_id, playbook_id, id);
ALTER TABLE crm_situations DROP CONSTRAINT IF EXISTS crm_situation_playbook_version;
ALTER TABLE crm_situations ADD CONSTRAINT crm_situation_playbook_version
    FOREIGN KEY (workspace_id, playbook_id, playbook_version_id) REFERENCES crm_playbook_versions(workspace_id, playbook_id, id);
ALTER TABLE crm_situations DROP CONSTRAINT IF EXISTS crm_situation_playbook_contract;
ALTER TABLE crm_situations ADD CONSTRAINT crm_situation_playbook_contract CHECK (
    (playbook_id IS NULL AND playbook_version_id IS NULL AND playbook_applied_by_member_id IS NULL AND playbook_applied_at IS NULL AND playbook_milestones IS NULL)
    OR (playbook_id IS NOT NULL AND playbook_version_id IS NOT NULL AND playbook_applied_by_member_id IS NOT NULL AND playbook_applied_at IS NOT NULL AND playbook_milestones IS NOT NULL AND jsonb_typeof(playbook_milestones) = 'array')
);
ALTER TABLE crm_situation_changes DROP CONSTRAINT IF EXISTS crm_situation_changes_operation_check;
ALTER TABLE crm_situation_changes ADD CONSTRAINT crm_situation_changes_operation_check
    CHECK (operation IN ('created', 'update', 'pause', 'resume', 'close', 'apply_playbook', 'update_milestone'));
