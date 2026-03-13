-- CRM Phase 1a: Buyer signal provenance and source-level dedupe

ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS source_thread_id UUID;

ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS evidence_excerpt TEXT;

ALTER TABLE crm_buyer_signals
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_crm_signals_source_thread
    ON crm_buyer_signals(source_thread_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signals_workspace_source_type_source_id_signal_type_unique
    ON crm_buyer_signals(workspace_id, source_type, source_id, signal_type)
    WHERE source_id IS NOT NULL;
