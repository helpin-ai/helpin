ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS dismissal_reason text,
    ADD COLUMN IF NOT EXISTS reviewed_at timestamptz,
    ADD COLUMN IF NOT EXISTS acted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_reviewed_at ON crm_buyer_signals (reviewed_at);
CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_acted_at ON crm_buyer_signals (acted_at);

CREATE TABLE IF NOT EXISTS crm_signal_feedback (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    signal_id uuid NOT NULL REFERENCES crm_buyer_signals(id) ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members(id) ON DELETE RESTRICT,
    action text NOT NULL CHECK (action IN ('reviewed', 'dismissed', 'acted')),
    dismissal_reason text CHECK (dismissal_reason IS NULL OR dismissal_reason IN (
        'incorrect_evidence', 'wrong_entity', 'duplicate', 'irrelevant', 'handled', 'bad_timing'
    )),
    rule_key text,
    rule_version integer,
    signal_domain text NOT NULL,
    identity_method text NOT NULL,
    detected_at timestamptz NOT NULL,
    occurred_at timestamptz NOT NULL,
    detection_to_event_millis bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_crm_signal_feedback_dimensions
    ON crm_signal_feedback (workspace_id, rule_key, rule_version, signal_domain, identity_method);

CREATE TABLE IF NOT EXISTS crm_signal_routing_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    version integer NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    minimum_priority double precision NOT NULL DEFAULT 12,
    required_trust text NOT NULL DEFAULT 'verified',
    route_to_owner boolean NOT NULL DEFAULT true,
    destination_team_id uuid,
    channels jsonb NOT NULL DEFAULT '["feed"]'::jsonb,
    created_by_member_id uuid NOT NULL REFERENCES workspace_members(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, version)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_crm_signal_routing_policy_enabled
    ON crm_signal_routing_policies (workspace_id) WHERE enabled;

CREATE TABLE IF NOT EXISTS crm_signal_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    signal_id uuid NOT NULL REFERENCES crm_buyer_signals(id) ON DELETE CASCADE,
    policy_id uuid NOT NULL REFERENCES crm_signal_routing_policies(id) ON DELETE RESTRICT,
    policy_version integer NOT NULL,
    channel text NOT NULL CHECK (channel IN ('feed', 'notification', 'digest')),
    recipient_member_id uuid,
    destination_team_id uuid,
    status text NOT NULL DEFAULT 'routed',
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_crm_signal_delivery_route
    ON crm_signal_deliveries (
        signal_id,
        policy_id,
        channel,
        COALESCE(recipient_member_id::text, ''),
        COALESCE(destination_team_id::text, '')
    );
