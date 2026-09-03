package repository

import (
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
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
	if err := EnsurePMExternalLinksTaskColumn(db); err != nil {
		return err
	}
	return nil
}

// EnsurePMExternalLinksTaskColumn ensures imports can write through the current
// task-based external-link model.
func EnsurePMExternalLinksTaskColumn(db *gorm.DB) error {
	tableName := model.PMExternalLink{}.TableName()
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	if !db.Migrator().HasColumn(tableName, "entity_type") {
		if err := db.Migrator().AddColumn(&model.PMExternalLink{}, "EntityType"); err != nil {
			return fmt.Errorf("add %s.entity_type column: %w", tableName, err)
		}
	}
	if !db.Migrator().HasColumn(tableName, "entity_id") {
		if err := db.Migrator().AddColumn(&model.PMExternalLink{}, "EntityID"); err != nil {
			return fmt.Errorf("add %s.entity_id column: %w", tableName, err)
		}
	}
	if err := db.Exec(`UPDATE pm_external_links SET entity_type = COALESCE(NULLIF(entity_type, ''), 'task') WHERE entity_type IS NULL OR entity_type = ''`).Error; err != nil {
		return fmt.Errorf("backfill pm_external_links.entity_type: %w", err)
	}
	backfillEntityID := `UPDATE pm_external_links SET entity_id = task_id WHERE (entity_id IS NULL OR entity_id = '') AND task_id IS NOT NULL AND task_id <> ''`
	if db.Dialector.Name() == "postgres" {
		backfillEntityID = `UPDATE pm_external_links SET entity_id = task_id::uuid WHERE entity_id IS NULL AND task_id::text ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'`
	}
	if err := db.Exec(backfillEntityID).Error; err != nil {
		return fmt.Errorf("backfill pm_external_links.entity_id from task_id: %w", err)
	}
	return nil
}
