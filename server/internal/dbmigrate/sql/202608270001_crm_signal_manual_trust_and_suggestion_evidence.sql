ALTER TABLE crm_buyer_signals
    ALTER COLUMN evidence_identity_method SET DEFAULT 'manual_entry',
    ALTER COLUMN evidence_identity_trust SET DEFAULT 'untrusted';

UPDATE crm_buyer_signals
SET detector_kind = 'manual',
    evidence_identity_method = 'manual_entry',
    evidence_identity_trust = 'untrusted',
    rule_key = NULL,
    rule_version = NULL
WHERE source_type = 'manual';

ALTER TABLE crm_suggestions
    ADD COLUMN IF NOT EXISTS signal_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS dismissal_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_crm_suggestions_dismissal_reason
    ON crm_suggestions (workspace_id, dismissal_reason);
