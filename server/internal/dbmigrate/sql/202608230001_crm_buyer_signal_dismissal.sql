ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS evidence_fingerprint text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS dismissed_at timestamptz,
    ADD COLUMN IF NOT EXISTS dismissed_by_member_id uuid;

CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_evidence_fingerprint
    ON crm_buyer_signals (evidence_fingerprint);

CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_dismissed_at
    ON crm_buyer_signals (dismissed_at);
