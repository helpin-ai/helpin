-- PM triage is separate from support conversation lifecycle decisions. It stores
-- only identifiers, hashes and decision metadata, never raw customer text.
CREATE TABLE IF NOT EXISTS pm_triage_assessments (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_kind text NOT NULL CHECK (source_kind IN ('task', 'support_conversation')),
    source_id uuid NOT NULL,
    source_hash text NOT NULL,
    context_hash text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('primary', 'shadow')),
    status text NOT NULL CHECK (status IN ('pending', 'ready', 'failed')),
    outcome jsonb NOT NULL DEFAULT '{}'::jsonb,
    reviewed jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pm_triage_cache ON pm_triage_assessments
    (workspace_id, actor_id, source_kind, source_id, context_hash, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pm_triage_daily ON pm_triage_assessments
    (workspace_id, created_at);
CREATE TABLE IF NOT EXISTS pm_triage_label_suppressions (
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    task_id uuid NOT NULL REFERENCES pm_tasks(id) ON DELETE CASCADE,
    label_id uuid NOT NULL REFERENCES pm_labels(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, task_id, label_id)
);
