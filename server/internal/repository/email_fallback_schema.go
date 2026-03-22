package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateEmailFallbackSchema creates indexes that AutoMigrate cannot express.
func MigrateEmailFallbackSchema(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if db.Dialector.Name() == "sqlite" {
		return nil
	}

	stmt := `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_sel_postmark_msg_id
		ON support_email_logs(postmark_message_id)
		WHERE postmark_message_id IS NOT NULL
	`
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("create support email log postmark partial unique index: %w", err)
	}
	return nil
}
