-- Product-usage profiles for CRM contacts and companies, rebuilt hourly from
-- helpin.events by CRMBehaviorProfileSynchronizer. A workspace's rows are
-- replaced wholesale on every sync, so there is no foreign key: a profile for
-- a deleted record disappears on the next sync and is never listed, because
-- list filters join profiles to existing records.

CREATE TABLE IF NOT EXISTS crm_behavior_profiles (
    entity_type text NOT NULL CHECK (entity_type IN ('contact', 'company')),
    entity_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    first_seen_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    sessions_30d integer NOT NULL DEFAULT 0,
    match_methods text NOT NULL DEFAULT '',
    computed_at timestamptz NOT NULL,
    PRIMARY KEY (entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_crm_behavior_profiles_workspace_id
    ON crm_behavior_profiles (workspace_id);
