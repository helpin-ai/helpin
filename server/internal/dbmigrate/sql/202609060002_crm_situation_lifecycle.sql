-- Additive lifecycle controls. No historical outcomes or actors are inferred.
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS outcome_basis text CHECK (outcome_basis IN ('human_assessment'));
ALTER TABLE crm_situations ADD COLUMN IF NOT EXISTS duplicate_of_situation_id uuid;

CREATE TABLE IF NOT EXISTS crm_situation_changes (
    id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    situation_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    command_key text NOT NULL CHECK (length(command_key) BETWEEN 1 AND 200),
    command_fingerprint text NOT NULL,
    operation text NOT NULL CHECK (operation IN ('created', 'update', 'pause', 'resume', 'close')),
    actor_kind text NOT NULL CHECK (actor_kind IN ('member', 'signal', 'suggestion')),
    actor_member_id uuid,
    reason text NOT NULL DEFAULT '' CHECK (length(reason) <= 2000),
    before jsonb,
    after jsonb NOT NULL,
    in_flight_action_count bigint NOT NULL DEFAULT 0 CHECK (in_flight_action_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((actor_kind = 'member' AND actor_member_id IS NOT NULL)
        OR (actor_kind <> 'member' AND actor_member_id IS NULL AND operation = 'created')),
    UNIQUE (workspace_id, situation_id, revision),
    UNIQUE (workspace_id, situation_id, command_key),
    FOREIGN KEY (workspace_id, situation_id)
        REFERENCES crm_situations(workspace_id, id) ON DELETE CASCADE
);
