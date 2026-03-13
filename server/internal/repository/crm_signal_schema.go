package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateCRMSignalSchema applies idempotent schema changes required for buyer
// signal provenance and source-level dedupe.
func MigrateCRMSignalSchema(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.crm_buyer_signals') IS NOT NULL THEN
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
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate crm signal schema: %w", err)
	}
	return nil
}
