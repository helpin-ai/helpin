-- Version 1 treated each moving evaluator boundary as a new identification
-- moment. Retire its unreliable context rows and activate the detector that is
-- anchored to the visitor's actual first identified event.

UPDATE crm_buyer_signals
SET dismissed_at = COALESCE(dismissed_at, NOW()),
    dismissal_reason = COALESCE(dismissal_reason, 'duplicate')
WHERE rule_key = 'pre_identification_history'
  AND rule_version = 1
  AND dismissed_at IS NULL;

INSERT INTO crm_signal_rule_configs
    (rule_key, version, cadence, enabled, shadow_mode, activation_eligible,
     thresholds, business_weight, half_life_days)
VALUES
    ('pre_identification_history', 2, 'micro_batch', true, true, false,
     '{"lookback_days":180,"identification_anchor":"first_identified_event"}', 10, 30)
ON CONFLICT DO NOTHING;
