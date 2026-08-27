CREATE TABLE IF NOT EXISTS crm_signal_rule_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    rule_key text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    cadence text NOT NULL CHECK (cadence IN ('daily', 'micro_batch')),
    enabled boolean NOT NULL DEFAULT true,
    shadow_mode boolean NOT NULL DEFAULT true,
    activation_eligible boolean NOT NULL DEFAULT false,
    thresholds jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signal_rule_configs_scope_version
    ON crm_signal_rule_configs (COALESCE(workspace_id::text, ''), rule_key, version);
CREATE INDEX IF NOT EXISTS idx_crm_signal_rule_configs_active
    ON crm_signal_rule_configs (cadence, rule_key, version DESC)
    WHERE enabled = true;

CREATE TABLE IF NOT EXISTS crm_signal_evaluation_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cadence text NOT NULL,
    rule_key text NOT NULL,
    rule_version integer NOT NULL,
    window_started_at timestamptz NOT NULL,
    window_ended_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    candidate_count integer NOT NULL DEFAULT 0,
    inserted_count integer NOT NULL DEFAULT 0,
    error_message text NULL,
    started_at timestamptz NOT NULL,
    completed_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_crm_signal_evaluation_runs_rule_window
    ON crm_signal_evaluation_runs (rule_key, rule_version, window_ended_at DESC);

CREATE TABLE IF NOT EXISTS crm_signal_evaluator_watermarks (
    cadence text PRIMARY KEY,
    watermark timestamptz NOT NULL,
    lease_owner text NULL,
    lease_until timestamptz NULL,
    last_started_at timestamptz NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_rule_signal_evidence
    ON crm_buyer_signals (
        workspace_id,
        rule_key,
        rule_version,
        COALESCE(contact_id::text, ''),
        COALESCE(deal_id::text, ''),
        COALESCE(company_id::text, ''),
        window_started_at,
        window_ended_at,
        evidence_fingerprint
    )
    WHERE rule_key IS NOT NULL;

INSERT INTO crm_signal_rule_configs
    (rule_key, version, cadence, enabled, shadow_mode, activation_eligible, thresholds)
VALUES
    ('support_volume_spike', 1, 'daily', true, true, true, '{"recent_days":7,"baseline_days":28,"minimum_recent":3,"multiplier":2.0}'),
    ('urgent_issue_open_deal', 1, 'daily', true, true, true, '{"priorities":["urgent","high"]}'),
    ('support_ai_escalation', 1, 'daily', true, true, true, '{}'),
    ('support_csat_deterioration', 1, 'daily', true, true, true, '{"recent_days":30,"baseline_days":90,"minimum_ratings":2,"minimum_drop":1.0}'),
    ('requested_feature_shipped', 1, 'daily', true, true, true, '{}'),
    ('deal_stage_stalled', 1, 'daily', true, true, true, '{"default_stage_days":30}'),
    ('deal_gone_dark', 1, 'daily', true, true, true, '{"default_silence_days":21}'),
    ('champion_quiet', 1, 'daily', true, true, true, '{"silence_days":21,"lookback_days":180}'),
    ('timeline_followup_lapsed', 1, 'daily', true, true, true, '{"grace_days":3}'),
    ('deal_single_threaded', 1, 'daily', true, true, true, '{"minimum_amount":10000}'),
    ('renewal_approaching', 1, 'daily', true, true, true, '{"lead_days":90,"engagement_days":30,"date_keys":["renewal_date","contract_end_date"]}'),
    ('buying_committee_expanded', 1, 'daily', true, true, true, '{"baseline_days":90}'),
    ('buying_committee_shrank', 1, 'daily', true, true, true, '{"silence_days":30,"lookback_days":180}'),
    ('repeated_pricing_activity', 1, 'micro_batch', true, true, true, '{"paths":["/pricing","/plans"],"minimum_sessions":2,"lookback_days":7}'),
    ('procurement_page_activity', 1, 'micro_batch', true, true, true, '{"paths":["/security","/compliance","/procurement","/trust"],"lookback_days":7}'),
    ('known_contact_returned', 1, 'micro_batch', true, true, true, '{"dormancy_days":30,"lookback_days":180}'),
    ('high_intent_product_event', 1, 'micro_batch', true, true, true, '{"event_names":["demo_requested","trial_started","pricing_plan_selected","upgrade_requested"],"lookback_days":7}'),
    ('session_depth_spike', 1, 'micro_batch', true, true, false, '{"minimum_pageviews":5,"lookback_days":1}'),
    ('new_account_stakeholder', 1, 'micro_batch', true, true, false, '{"lookback_days":90}'),
    ('anonymous_account_traffic', 1, 'micro_batch', true, true, false, '{"minimum_events":3,"lookback_days":1}'),
    ('campaign_attributed_return', 1, 'micro_batch', true, true, false, '{"lookback_days":30}'),
    ('pre_identification_history', 1, 'micro_batch', true, true, false, '{"lookback_days":180}')
ON CONFLICT DO NOTHING;
