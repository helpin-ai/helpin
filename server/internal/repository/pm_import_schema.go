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
	if err := EnsurePMChecklistItemsTaskColumn(db); err != nil {
		return err
	}
	if err := EnsurePMExternalLinksTaskColumn(db); err != nil {
		return err
	}
	return nil
}

// EnsurePMChecklistItemsTaskColumn reconciles legacy story_id drift on checklist
// items so imports can write through the current task_id-based model.
func EnsurePMChecklistItemsTaskColumn(db *gorm.DB) error {
	tableName := model.PMChecklistItem{}.TableName()
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	hasStoryID := db.Migrator().HasColumn(tableName, "story_id")
	if !hasStoryID {
		return nil
	}

	hasTaskID := db.Migrator().HasColumn(tableName, "task_id")
	if !hasTaskID {
		if err := db.Migrator().RenameColumn(tableName, "story_id", "task_id"); err != nil {
			return fmt.Errorf("rename %s.story_id to task_id: %w", tableName, err)
		}
		return nil
	}

	if err := db.Exec(`UPDATE pm_checklist_items SET task_id = story_id WHERE task_id IS NULL AND story_id IS NOT NULL`).Error; err != nil {
		return fmt.Errorf("backfill pm_checklist_items.task_id from story_id: %w", err)
	}
	if err := db.Exec(`ALTER TABLE pm_checklist_items DROP COLUMN story_id`).Error; err != nil {
		return fmt.Errorf("drop legacy %s.story_id column: %w", tableName, err)
	}
	return nil
}

// EnsurePMExternalLinksTaskColumn reconciles legacy story_id drift on external
// links so imports can write through the current task_id-based model.
func EnsurePMExternalLinksTaskColumn(db *gorm.DB) error {
	tableName := model.PMExternalLink{}.TableName()
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	hasStoryID := db.Migrator().HasColumn(tableName, "story_id")
	if !hasStoryID {
		return nil
	}

	hasTaskID := db.Migrator().HasColumn(tableName, "task_id")
	if !hasTaskID {
		if err := db.Migrator().RenameColumn(tableName, "story_id", "task_id"); err != nil {
			return fmt.Errorf("rename %s.story_id to task_id: %w", tableName, err)
		}
		return nil
	}

	if err := db.Exec(`UPDATE pm_external_links SET task_id = story_id WHERE task_id IS NULL AND story_id IS NOT NULL`).Error; err != nil {
		return fmt.Errorf("backfill pm_external_links.task_id from story_id: %w", err)
	}
	if err := db.Exec(`ALTER TABLE pm_external_links DROP COLUMN story_id`).Error; err != nil {
		return fmt.Errorf("drop legacy %s.story_id column: %w", tableName, err)
	}
	return nil
}
