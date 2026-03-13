package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateAutomationHealthSchema applies idempotent indexes for automation health snapshots.
func MigrateAutomationHealthSchema(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.automation_health_snapshots') IS NOT NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_automation_health_scope
            ON automation_health_snapshots(workspace_id, catalog_id, scope_type, scope_id);
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate automation health schema: %w", err)
	}
	return nil
}
