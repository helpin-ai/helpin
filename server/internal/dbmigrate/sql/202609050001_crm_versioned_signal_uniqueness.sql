-- Versioned interpretations may share a source and signal type. The legacy
-- source-only unique index was recreated by the table-rename migration and
-- silently rejected those rows via ON CONFLICT DO NOTHING.
-- Keep its protection only for legacy rows without an interpretation fingerprint.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_index
        WHERE indexrelid = to_regclass('public.idx_crm_signals_unique_meaning')
          AND indrelid = 'public.crm_signals'::regclass AND indisunique AND indisvalid
    ) OR NOT EXISTS (
        SELECT 1 FROM pg_index
        WHERE indexrelid = to_regclass('public.idx_crm_rule_signal_evidence')
          AND indrelid = 'public.crm_signals'::regclass AND indisunique AND indisvalid
    ) THEN
        RAISE EXCEPTION 'versioned CRM signal uniqueness guards are missing or invalid';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_signals_legacy_source_unique
    ON crm_signals (workspace_id, source_type, source_id, signal_type)
    WHERE source_id IS NOT NULL AND meaning_fingerprint = '';

DROP INDEX IF EXISTS idx_crm_signals_workspace_source_type_source_id_signal_type_uni;
