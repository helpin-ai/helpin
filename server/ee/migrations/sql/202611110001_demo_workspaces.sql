-- Demo workspaces built from a company's public help center. One row tracks
-- one build from request to result, so a build survives restarts and can be
-- polled.
CREATE TABLE IF NOT EXISTS demo_workspaces (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The workspace that asked for the demo and holds the company record.
    home_workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    company_id uuid NOT NULL,
    company_name text NOT NULL,
    domain text NOT NULL DEFAULT '',
    help_center_url text NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'building', 'ready', 'failed')),
    step text NOT NULL DEFAULT 'organization',
    error text,
    organization_id uuid,
    workspace_id uuid,
    space_id uuid,
    import_job_id uuid,
    subdomain text,
    demo_url text,
    report jsonb NOT NULL DEFAULT '{}'::jsonb,
    requested_by uuid NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    lease_until timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- One demo per company: asking again returns the existing build.
CREATE UNIQUE INDEX IF NOT EXISTS idx_demo_workspaces_home_company
    ON demo_workspaces (home_workspace_id, company_id);

-- The worker picks up unfinished builds whose lease has run out.
CREATE INDEX IF NOT EXISTS idx_demo_workspaces_unfinished
    ON demo_workspaces (lease_until, created_at) WHERE status IN ('queued', 'building');
