CREATE TABLE IF NOT EXISTS flow_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    template_id text NOT NULL,
    template_version integer NOT NULL DEFAULT 1,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    status text NOT NULL,
    current_node_id text,
    trigger_type text NOT NULL DEFAULT 'manual',
    trigger_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    output_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    spec_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text,
    started_by uuid,
    completed_at timestamptz,
    cancellation_reason text,
    retry_count integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS flow_node_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    flow_run_id uuid NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
    node_id text NOT NULL,
    node_type text NOT NULL,
    status text NOT NULL,
    attempt_count integer NOT NULL DEFAULT 1,
    agent_id uuid,
    child_run_id uuid,
    child_session_id uuid,
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    output jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_message text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS flow_triggers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    template_id text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    trigger_type text NOT NULL,
    trigger_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    scope_filters jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE agent_runs
    ADD COLUMN IF NOT EXISTS flow_run_id uuid,
    ADD COLUMN IF NOT EXISTS flow_node_run_id uuid;

ALTER TABLE planning_sessions
    ADD COLUMN IF NOT EXISTS flow_run_id uuid,
    ADD COLUMN IF NOT EXISTS flow_node_run_id uuid;

ALTER TABLE pm_epics
    ADD COLUMN IF NOT EXISTS active_flow_run_id uuid;

CREATE INDEX IF NOT EXISTS idx_flow_runs_workspace_id ON flow_runs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_flow_runs_template_id ON flow_runs(template_id);
CREATE INDEX IF NOT EXISTS idx_flow_runs_target_type ON flow_runs(target_type);
CREATE INDEX IF NOT EXISTS idx_flow_runs_target_id ON flow_runs(target_id);
CREATE INDEX IF NOT EXISTS idx_flow_runs_status ON flow_runs(status);
CREATE INDEX IF NOT EXISTS idx_flow_runs_dedupe_key ON flow_runs(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_flow_runs_active_target ON flow_runs(workspace_id, template_id, target_type, target_id, status);

CREATE INDEX IF NOT EXISTS idx_flow_node_runs_flow_run_id ON flow_node_runs(flow_run_id);
CREATE INDEX IF NOT EXISTS idx_flow_node_runs_node_id ON flow_node_runs(node_id);
CREATE INDEX IF NOT EXISTS idx_flow_node_runs_status ON flow_node_runs(status);
CREATE INDEX IF NOT EXISTS idx_flow_node_runs_flow_node_created ON flow_node_runs(flow_run_id, node_id, created_at);
CREATE INDEX IF NOT EXISTS idx_flow_node_runs_child_run_id ON flow_node_runs(child_run_id);
CREATE INDEX IF NOT EXISTS idx_flow_node_runs_child_session_id ON flow_node_runs(child_session_id);

CREATE INDEX IF NOT EXISTS idx_flow_triggers_workspace_id ON flow_triggers(workspace_id);
CREATE INDEX IF NOT EXISTS idx_flow_triggers_template_id ON flow_triggers(template_id);
CREATE INDEX IF NOT EXISTS idx_flow_triggers_trigger_type ON flow_triggers(trigger_type);

CREATE INDEX IF NOT EXISTS idx_agent_runs_flow_run_id ON agent_runs(flow_run_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_flow_node_run_id ON agent_runs(flow_node_run_id);
CREATE INDEX IF NOT EXISTS idx_planning_sessions_flow_run_id ON planning_sessions(flow_run_id);
CREATE INDEX IF NOT EXISTS idx_planning_sessions_flow_node_run_id ON planning_sessions(flow_node_run_id);
CREATE INDEX IF NOT EXISTS idx_pm_epics_active_flow_run_id ON pm_epics(active_flow_run_id);
