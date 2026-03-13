package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateCRMSummarySchema applies idempotent summary-specific indexes and defaults.
func MigrateCRMSummarySchema(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.crm_entity_summaries') IS NOT NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_entity_summaries_ws_entity
            ON crm_entity_summaries(workspace_id, entity_type, entity_id);
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate crm summary schema: %w", err)
	}
	return nil
}
