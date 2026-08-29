-- Server-authenticated customer state, baseline, and level-rule foundations.

CREATE TABLE IF NOT EXISTS crm_company_commercial_states (
    company_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    version integer NOT NULL DEFAULT 1,
    state jsonb NOT NULL DEFAULT '{}'::jsonb,
    state_updated_at timestamptz NOT NULL,
    source_event_id text NOT NULL,
    applied_to_current boolean NOT NULL DEFAULT true,
    accepted_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_crm_company_commercial_state_workspace
    ON crm_company_commercial_states (workspace_id, state_updated_at DESC);

CREATE TABLE IF NOT EXISTS crm_company_commercial_state_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    company_id uuid NOT NULL,
    version integer NOT NULL,
    patch jsonb NOT NULL DEFAULT '{}'::jsonb,
    state_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    state_updated_at timestamptz NOT NULL,
    source_event_id text NOT NULL,
    accepted_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_commercial_state_history_workspace_event
    ON crm_company_commercial_state_history (workspace_id, source_event_id);
CREATE INDEX IF NOT EXISTS idx_crm_company_commercial_state_history_company
    ON crm_company_commercial_state_history (workspace_id, company_id, state_updated_at DESC);

CREATE TABLE IF NOT EXISTS crm_company_commercial_state_health (
    company_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    last_accepted_at timestamptz,
    last_rejected_at timestamptz,
    last_rejection_reason text,
    rejected_update_count integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS crm_signal_condition_states (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    company_id uuid NOT NULL,
    rule_key text NOT NULL,
    rule_version integer NOT NULL,
    motion text NOT NULL,
    scope_key text NOT NULL DEFAULT '',
    armed boolean NOT NULL DEFAULT true,
    triggered_at timestamptz,
    rearmed_at timestamptz,
    last_value double precision,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, company_id, rule_key, rule_version, motion, scope_key)
);

CREATE TABLE IF NOT EXISTS crm_usage_weekday_baselines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    company_id uuid NOT NULL,
    metric_key text NOT NULL,
    weekday integer NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    median_value double precision NOT NULL,
    observation_count integer NOT NULL,
    complete_weeks integer NOT NULL,
    identity_method text NOT NULL DEFAULT 'unknown',
    window_started_at timestamptz NOT NULL,
    window_ended_at timestamptz NOT NULL,
    calculated_at timestamptz NOT NULL,
    UNIQUE (workspace_id, company_id, metric_key, weekday, window_ended_at)
);
CREATE INDEX IF NOT EXISTS idx_crm_usage_weekday_baseline_retention
    ON crm_usage_weekday_baselines (window_ended_at);

CREATE TABLE IF NOT EXISTS crm_signal_batch_suppressions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    rule_key text NOT NULL,
    rule_version integer NOT NULL,
    eligible_accounts integer NOT NULL,
    tripped_accounts integer NOT NULL,
    tripped_ratio double precision NOT NULL,
    reason text NOT NULL,
    evaluation_window_started_at timestamptz NOT NULL,
    evaluation_window_ended_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO crm_signal_rule_configs
    (rule_key, version, cadence, enabled, shadow_mode, activation_eligible,
     thresholds, business_weight, half_life_days)
VALUES
    ('account_usage_decline', 1, 'daily', true, true, true,
     '{"metric_key":"feature_used","baseline_weeks":8,"minimum_complete_weeks":8,"minimum_weekday_observations":3,"days_below":5,"decline_ratio":0.60}', 20, 21),
    ('activation_stalled', 1, 'daily', true, true, true,
     '{"days_without_first_value":7}', 18, 14),
    ('workflow_failure_spike', 1, 'daily', true, true, true,
     '{"metric_key":"workflow_failed","baseline_weeks":8,"minimum_complete_weeks":8,"spike_ratio":2.0,"minimum_recent":3}', 18, 14),
    ('capacity_saturation', 1, 'daily', true, true, true,
     '{"trigger_ratio":0.85,"rearm_ratio":0.80}', 16, 30),
    ('payment_failed', 1, 'micro_batch', true, true, true,
     '{"event_names":["payment_failed"]}', 25, 14),
    ('downgrade_requested', 1, 'micro_batch', true, true, true,
     '{"event_names":["downgrade_requested"]}', 25, 30)
ON CONFLICT DO NOTHING;

WITH mappings(rule_key, motion, signal_type, polarity, action_key, action_label) AS (VALUES
    ('account_usage_decline', 'retention', 'risk_signal', 'negative', 'investigate_usage_decline', 'Investigate the usage decline'),
    ('activation_stalled', 'onboarding', 'risk_signal', 'negative', 'unstick_activation', 'Unstick account activation'),
    ('workflow_failure_spike', 'retention', 'risk_signal', 'negative', 'investigate_workflow_failures', 'Investigate workflow failures'),
    ('capacity_saturation', 'expansion', 'buying_intent', 'positive', 'review_capacity_expansion', 'Review a capacity expansion'),
    ('capacity_saturation', 'adoption', 'timeline_signal', 'neutral', 'review_capacity', 'Review account capacity'),
    ('payment_failed', 'retention', 'risk_signal', 'negative', 'resolve_payment_failure', 'Resolve the payment failure'),
    ('downgrade_requested', 'retention', 'risk_signal', 'negative', 'respond_to_downgrade', 'Respond to the downgrade request')
)
INSERT INTO crm_signal_interpretation_configs (
    workspace_id, rule_key, rule_version, motion, observation_signal_type,
    version, signal_type, polarity, business_weight, half_life_days,
    recommended_action_key, recommended_action_label, enabled
)
SELECT NULL, mappings.rule_key, 1, mappings.motion, '*', 1,
       mappings.signal_type, mappings.polarity, cfg.business_weight, cfg.half_life_days,
       mappings.action_key, mappings.action_label, true
FROM mappings
JOIN crm_signal_rule_configs cfg ON cfg.rule_key = mappings.rule_key AND cfg.version = 1
ON CONFLICT DO NOTHING;

-- The shadow rollout gate is intentionally strict: every enabled global rule
-- version must have at least one explicit commercial interpretation. Fail the
-- migration instead of leaving a workspace permanently unmapped in shadow.
DO $$
DECLARE
    missing_rule_versions text;
BEGIN
    SELECT string_agg(cfg.rule_key || '@' || cfg.version::text, ', ' ORDER BY cfg.rule_key, cfg.version)
    INTO missing_rule_versions
    FROM crm_signal_rule_configs cfg
    WHERE cfg.workspace_id IS NULL
      AND cfg.enabled = true
      AND NOT EXISTS (
          SELECT 1
          FROM crm_signal_interpretation_configs interpretation
          WHERE interpretation.workspace_id IS NULL
            AND interpretation.rule_key = cfg.rule_key
            AND interpretation.rule_version = cfg.version
            AND interpretation.enabled = true
      );

    IF missing_rule_versions IS NOT NULL THEN
        RAISE EXCEPTION 'enabled CRM signal rules lack interpretation mappings: %', missing_rule_versions;
    END IF;
END $$;
