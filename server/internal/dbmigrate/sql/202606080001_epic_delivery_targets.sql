CREATE TABLE IF NOT EXISTS epic_delivery_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    epic_id UUID NOT NULL,
    repository_id UUID,
    repo_full_name TEXT,
    integration_id UUID,
    base_branch TEXT,
    epic_branch TEXT,
    delivery_state TEXT NOT NULL DEFAULT 'unconfigured',
    final_pr_number INTEGER,
    final_pr_title TEXT,
    final_pr_url TEXT,
    final_pr_status TEXT,
    last_commit_sha TEXT,
    last_run_id UUID,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_epic_delivery_targets_epic_id
    ON epic_delivery_targets (epic_id);

CREATE INDEX IF NOT EXISTS idx_epic_delivery_targets_workspace_id
    ON epic_delivery_targets (workspace_id);

CREATE INDEX IF NOT EXISTS idx_epic_delivery_targets_repository_id
    ON epic_delivery_targets (repository_id);

CREATE INDEX IF NOT EXISTS idx_epic_delivery_targets_integration_id
    ON epic_delivery_targets (integration_id);
