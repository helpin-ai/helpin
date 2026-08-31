ALTER TABLE crm_identity_links
    ADD COLUMN IF NOT EXISTS external_user_id text;

CREATE INDEX IF NOT EXISTS idx_crm_identity_links_workspace_external_user
    ON crm_identity_links (workspace_id, external_user_id, created_at DESC)
    WHERE external_user_id IS NOT NULL AND external_user_id <> '';

ALTER TABLE crm_signal_deliveries
    ADD COLUMN IF NOT EXISTS attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_attempted_at timestamptz,
    ADD COLUMN IF NOT EXISTS last_error text;

UPDATE crm_signal_deliveries
SET status = 'sent',
    delivered_at = COALESCE(delivered_at, created_at)
WHERE status = 'routed';

ALTER TABLE crm_signal_deliveries
    ALTER COLUMN status SET DEFAULT 'pending';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'crm_signal_deliveries_status_check'
    ) THEN
        ALTER TABLE crm_signal_deliveries
            ADD CONSTRAINT crm_signal_deliveries_status_check
            CHECK (status IN ('pending', 'sending', 'sent', 'failed'));
    END IF;
END $$;

ALTER TABLE crm_signal_rule_configs
    DROP CONSTRAINT IF EXISTS crm_signal_rule_configs_cadence_check;

ALTER TABLE crm_signal_rule_configs
    ADD CONSTRAINT crm_signal_rule_configs_cadence_check
    CHECK (cadence IN ('daily', 'micro_batch', 'event_driven'));

INSERT INTO crm_signal_rule_configs
    (rule_key, version, cadence, enabled, shadow_mode, activation_eligible,
     thresholds, business_weight, half_life_days)
VALUES
    ('conversation_signal_extraction', 3, 'event_driven', true, true, true,
     '{"detector_version":"verified-v3-en","minimum_confidence":0.6,"verified_evidence_required":true}',
     10, 30)
ON CONFLICT DO NOTHING;

UPDATE crm_buyer_signals
SET rule_key = 'conversation_signal_extraction',
    rule_version = 3
WHERE detector_kind = 'llm_extracted'
  AND rule_key IS NULL
  AND metadata->>'detector_version' = 'verified-v3-en';
