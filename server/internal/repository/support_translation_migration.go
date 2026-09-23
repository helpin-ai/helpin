package repository

import "gorm.io/gorm"

// MigrateSupportTranslationConversationDefaults repairs rows and defaults that
// predate Live Translate. Run before AutoMigrate: it can make these columns
// required before the versioned SQL migration has run.
func MigrateSupportTranslationConversationDefaults(db *gorm.DB) error {
	const table = "support_translation_conversations"
	if !db.Migrator().HasTable(table) {
		return nil
	}
	if db.Migrator().HasColumn(table, "customer_language") {
		if err := db.Exec(`UPDATE support_translation_conversations SET customer_language = '' WHERE customer_language IS NULL`).Error; err != nil {
			return err
		}
		if db.Dialector.Name() == "postgres" {
			if err := db.Exec(`ALTER TABLE support_translation_conversations ALTER COLUMN customer_language SET DEFAULT ''`).Error; err != nil {
				return err
			}
		}
	}
	if db.Migrator().HasColumn(table, "revision") {
		if err := db.Exec(`UPDATE support_translation_conversations SET revision = 1 WHERE revision IS NULL`).Error; err != nil {
			return err
		}
		if db.Dialector.Name() == "postgres" {
			return db.Exec(`ALTER TABLE support_translation_conversations ALTER COLUMN revision SET DEFAULT 1, ALTER COLUMN revision SET NOT NULL`).Error
		}
	}
	return nil
}
