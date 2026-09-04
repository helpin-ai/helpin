-- A new immutable detector version requires business relevance and revenue consequence.
-- Existing source evidence remains available as unqualified context; no data is deleted.
INSERT INTO crm_signal_rule_configs (
 workspace_id, rule_key, version, cadence, enabled, shadow_mode, activation_eligible,
 thresholds, business_weight, half_life_days
)
SELECT workspace_id, rule_key, 4, cadence, enabled, true, activation_eligible,
 thresholds || '{"detector_version":"commercial-v4-en","commercial_evidence_required":true}'::jsonb,
 business_weight, half_life_days
FROM crm_signal_rule_configs
WHERE rule_key = 'conversation_signal_extraction' AND version = 3
ON CONFLICT DO NOTHING;

INSERT INTO crm_signal_interpretation_configs (
 workspace_id, rule_key, rule_version, motion, observation_signal_type,
 version, signal_type, polarity, business_weight, half_life_days,
 recommended_action_key, recommended_action_label, enabled
)
SELECT cfg.workspace_id, cfg.rule_key, 4, motion, '*', 1,
 'inherit', 'inherit', cfg.business_weight, cfg.half_life_days,
 'review_commercial_evidence', 'Review the commercial evidence', true
FROM crm_signal_rule_configs cfg
CROSS JOIN unnest(ARRAY['prospecting','conversion','onboarding','adoption','expansion','renewal','retention','needs_context']) AS motion
WHERE cfg.rule_key = 'conversation_signal_extraction' AND cfg.version = 4
ON CONFLICT DO NOTHING;
