-- Automation rules: user-configured trigger → action rules
-- First implementation: stage-based agent pipeline for PM stories

CREATE TABLE IF NOT EXISTS automation_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    team_id         UUID REFERENCES teams(id) ON DELETE CASCADE,
    workflow_id     UUID REFERENCES pm_workflows(id) ON DELETE CASCADE,
    trigger_type    TEXT NOT NULL,
    trigger_config  JSONB NOT NULL DEFAULT '{}'::jsonb,
    action_type     TEXT NOT NULL,
    action_config   JSONB NOT NULL DEFAULT '{}'::jsonb,
    position        INT NOT NULL DEFAULT 0,
    stop_on_match   BOOLEAN NOT NULL DEFAULT false,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_automation_rules_trigger
    ON automation_rules(workspace_id, trigger_type, enabled);
CREATE INDEX IF NOT EXISTS idx_automation_rules_workflow
    ON automation_rules(workflow_id) WHERE workflow_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_automation_rules_team
    ON automation_rules(team_id) WHERE team_id IS NOT NULL;
