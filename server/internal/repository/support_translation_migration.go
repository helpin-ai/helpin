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

// MigrateSupportTranslationPolicyRevision repairs a column that AutoMigrate may
// have added as nullable before the versioned Live Translate migration ran.
// Run before AutoMigrate so it can safely apply the model's NOT NULL constraint.
func MigrateSupportTranslationPolicyRevision(db *gorm.DB) error {
	const table = "support_translations"
	if !db.Migrator().HasTable(table) || !db.Migrator().HasColumn(table, "policy_revision") {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE support_translations SET policy_revision = 0 WHERE policy_revision IS NULL`).Error; err != nil {
			return err
		}
		if tx.Dialector.Name() == "postgres" {
			return tx.Exec(`ALTER TABLE support_translations ALTER COLUMN policy_revision SET DEFAULT 0, ALTER COLUMN policy_revision SET NOT NULL`).Error
		}
		return nil
	})
}
