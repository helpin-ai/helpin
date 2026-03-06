package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigratePMImportSchema applies idempotent schema changes required for Shortcut imports.
func MigratePMImportSchema(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.pm_epics') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='pm_epics' AND column_name='external_id') THEN
            ALTER TABLE pm_epics ADD COLUMN external_id text;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename='pm_epics' AND indexname='idx_pm_epics_external_id') THEN
            CREATE INDEX idx_pm_epics_external_id ON pm_epics (external_id);
        END IF;
    END IF;

    IF to_regclass('public.pm_objectives') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='pm_objectives' AND column_name='external_id') THEN
            ALTER TABLE pm_objectives ADD COLUMN external_id text;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename='pm_objectives' AND indexname='idx_pm_objectives_external_id') THEN
            CREATE INDEX idx_pm_objectives_external_id ON pm_objectives (external_id);
        END IF;
    END IF;

    IF to_regclass('public.pm_sprints') IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='pm_sprints' AND column_name='external_id') THEN
            ALTER TABLE pm_sprints ADD COLUMN external_id text;
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename='pm_sprints' AND indexname='idx_pm_sprints_external_id') THEN
            CREATE INDEX idx_pm_sprints_external_id ON pm_sprints (external_id);
        END IF;
        ALTER TABLE pm_sprints ALTER COLUMN start_date DROP NOT NULL;
        ALTER TABLE pm_sprints ALTER COLUMN end_date DROP NOT NULL;
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate pm import schema: %w", err)
	}
	return nil
}
