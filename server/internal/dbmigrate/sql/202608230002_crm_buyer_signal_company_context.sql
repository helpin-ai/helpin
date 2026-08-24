ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS company_id uuid;

CREATE INDEX IF NOT EXISTS idx_crm_buyer_signals_workspace_company_detected
    ON crm_buyer_signals (workspace_id, company_id, detected_at DESC)
    WHERE company_id IS NOT NULL;
