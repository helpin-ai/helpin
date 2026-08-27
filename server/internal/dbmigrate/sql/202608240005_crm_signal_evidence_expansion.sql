CREATE TABLE IF NOT EXISTS crm_signal_external_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_evidence_id text NOT NULL,
    evidence_type text NOT NULL CHECK (evidence_type IN ('funding', 'hiring', 'job_change', 'technology', 'leadership', 'third_party_intent')),
    rule_key text NOT NULL,
    rule_version integer NOT NULL CHECK (rule_version > 0),
    signal_type text NOT NULL,
    signal_domain text NOT NULL,
    polarity text NOT NULL,
    summary text NOT NULL,
    evidence_excerpt text,
    source_url text,
    contact_id uuid REFERENCES crm_contacts(id) ON DELETE SET NULL,
    deal_id uuid REFERENCES crm_deals(id) ON DELETE SET NULL,
    company_id uuid REFERENCES crm_companies(id) ON DELETE SET NULL,
    identity_method text NOT NULL,
    identity_trust text NOT NULL,
    provenance jsonb NOT NULL DEFAULT '{}'::jsonb,
    observed_at timestamptz NOT NULL,
    signal_id uuid REFERENCES crm_buyer_signals(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider, provider_evidence_id, rule_key, rule_version)
);

CREATE INDEX IF NOT EXISTS idx_crm_signal_external_evidence_dimensions
    ON crm_signal_external_evidence (workspace_id, evidence_type, signal_domain, identity_trust, observed_at DESC);

INSERT INTO crm_signal_rule_configs
    (rule_key, version, cadence, enabled, shadow_mode, activation_eligible, thresholds, business_weight, half_life_days)
VALUES
    ('configured_form_submission', 1, 'micro_batch', true, true, true, '{"event_name":"$form"}', 15, 14),
    ('identified_article_view', 1, 'micro_batch', true, true, false, '{"event_name":"article_view"}', 6, 14),
    ('versioned_interaction', 1, 'micro_batch', false, true, false, '{"event_name":"$interaction"}', 5, 7),
    ('external_provider_evidence', 1, 'daily', true, true, false, '{"evidence_types":["funding","hiring","job_change","technology","leadership","third_party_intent"]}', 5, 30)
ON CONFLICT DO NOTHING;
