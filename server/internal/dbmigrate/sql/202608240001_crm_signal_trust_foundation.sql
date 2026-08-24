-- Phase 0: stable event tenancy, identity provenance, and signal dimensions.

ALTER TABLE support_widget_installations
    ADD COLUMN IF NOT EXISTS allowed_origins TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS identity_verification_mode TEXT NOT NULL DEFAULT 'report_only';

UPDATE support_widget_installations
SET identity_verification_mode = 'report_only'
WHERE identity_verification_mode IS NULL OR identity_verification_mode = '';

ALTER TABLE support_widget_installations
    ALTER COLUMN identity_verification_mode SET DEFAULT 'enforced';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'support_widget_installations_identity_mode_check'
    ) THEN
        ALTER TABLE support_widget_installations
            ADD CONSTRAINT support_widget_installations_identity_mode_check
            CHECK (identity_verification_mode IN ('off', 'report_only', 'enforced'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS workspace_event_project_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL,
    valid_from TIMESTAMPTZ,
    valid_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, project_id),
    CHECK (project_id <> ''),
    CHECK (source IN ('legacy_widget_key', 'legacy_server_secret', 'retired_widget_key', 'retired_server_secret'))
);

CREATE INDEX IF NOT EXISTS idx_event_project_alias_workspace
    ON workspace_event_project_aliases (workspace_id, valid_to);

INSERT INTO workspace_event_project_aliases (workspace_id, project_id, source, valid_from)
SELECT workspace_id, widget_key, 'legacy_widget_key', created_at
FROM support_widget_installations
WHERE active = TRUE AND widget_key <> ''
ON CONFLICT (project_id) DO NOTHING;

INSERT INTO workspace_event_project_aliases (workspace_id, project_id, source, valid_from)
SELECT workspace_id, secret_key, 'legacy_server_secret', created_at
FROM support_widget_installations
WHERE active = TRUE AND secret_key <> ''
ON CONFLICT (project_id) DO NOTHING;

ALTER TABLE support_widget_sessions
    ADD COLUMN IF NOT EXISTS identity_method TEXT NOT NULL DEFAULT 'anonymous',
    ADD COLUMN IF NOT EXISTS identity_trust TEXT NOT NULL DEFAULT 'untrusted',
    ADD COLUMN IF NOT EXISTS identity_verified_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS identity_verifier_version TEXT;

CREATE INDEX IF NOT EXISTS idx_support_widget_sessions_identity_method
    ON support_widget_sessions (identity_method);
CREATE INDEX IF NOT EXISTS idx_support_widget_sessions_identity_trust
    ON support_widget_sessions (identity_trust);

CREATE TABLE IF NOT EXISTS crm_identity_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    anonymous_id TEXT NOT NULL,
    contact_id UUID REFERENCES crm_contacts(id) ON DELETE SET NULL,
    company_id UUID REFERENCES crm_companies(id) ON DELETE SET NULL,
    identity_method TEXT NOT NULL,
    identity_trust TEXT NOT NULL,
    verified_at TIMESTAMPTZ,
    verifier_version TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_identity_links_workspace_anonymous
    ON crm_identity_links (workspace_id, anonymous_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_crm_identity_links_contact ON crm_identity_links (contact_id);
CREATE INDEX IF NOT EXISTS idx_crm_identity_links_company ON crm_identity_links (company_id);

CREATE TABLE IF NOT EXISTS support_credential_rotation_audits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    installation_id UUID NOT NULL REFERENCES support_widget_installations(id) ON DELETE CASCADE,
    actor_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    rotation_kind TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_credential_rotation_workspace
    ON support_credential_rotation_audits (workspace_id, created_at DESC);

ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS detector_kind TEXT NOT NULL DEFAULT 'llm_extracted',
    ADD COLUMN IF NOT EXISTS signal_domain TEXT NOT NULL DEFAULT 'conversation',
    ADD COLUMN IF NOT EXISTS polarity TEXT NOT NULL DEFAULT 'neutral',
    ADD COLUMN IF NOT EXISTS rule_key TEXT,
    ADD COLUMN IF NOT EXISTS rule_version INTEGER,
    ADD COLUMN IF NOT EXISTS window_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS window_ended_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS evidence_identity_method TEXT NOT NULL DEFAULT 'connected_mailbox',
    ADD COLUMN IF NOT EXISTS evidence_identity_trust TEXT NOT NULL DEFAULT 'verified';

UPDATE crm_buyer_signals
SET detector_kind = 'llm_extracted',
    signal_domain = 'conversation',
    polarity = CASE
        WHEN signal_type IN ('buying_intent', 'budget_signal', 'timeline_signal', 'champion_signal') THEN 'positive'
        WHEN signal_type IN ('objection', 'competitor_mention', 'risk_signal') THEN 'negative'
        ELSE 'neutral'
    END,
    evidence_identity_method = CASE
        WHEN source_type = 'support' THEN 'verified_support'
        ELSE 'connected_mailbox'
    END,
    evidence_identity_trust = 'verified';

CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_rule_window
    ON crm_buyer_signals (workspace_id, rule_key, rule_version, window_ended_at);
CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_domain_polarity
    ON crm_buyer_signals (workspace_id, signal_domain, polarity, detected_at DESC);
