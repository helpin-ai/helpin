package repository

import "gorm.io/gorm"

// MigrateSupportTranslationCustomerLanguage repairs rows written before the
// customer_language column became required. Run this before GORM AutoMigrate,
// which otherwise tries to add the NOT NULL constraint while NULL rows exist.
func MigrateSupportTranslationCustomerLanguage(db *gorm.DB) error {
	if !db.Migrator().HasTable("support_translation_conversations") ||
		!db.Migrator().HasColumn("support_translation_conversations", "customer_language") {
		return nil
	}
	return db.Exec(`UPDATE support_translation_conversations SET customer_language = '' WHERE customer_language IS NULL`).Error
}
