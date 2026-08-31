UPDATE crm_signal_rule_configs
SET enabled = false, updated_at = now()
WHERE rule_key = 'support_ai_escalation'
  AND version = 1
  AND enabled = true;

INSERT INTO crm_signal_rule_configs
    (rule_key, version, cadence, enabled, shadow_mode, activation_eligible, thresholds, business_weight, half_life_days)
VALUES
    ('support_ai_escalation', 2, 'daily', true, true, false, '{"classification":"operational_context"}', 6, 7)
ON CONFLICT DO NOTHING;

UPDATE crm_buyer_signals
SET signal_type = 'timeline_signal',
    polarity = 'neutral'
WHERE rule_key = 'support_ai_escalation'
  AND rule_version = 1
  AND (signal_type <> 'timeline_signal' OR polarity <> 'neutral');
