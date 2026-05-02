package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateSupportEmailRouteSchema creates indexes that AutoMigrate cannot express.
func MigrateSupportEmailRouteSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() == "sqlite" {
		return nil
	}

	statements := []string{
		`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_ser_active_shared_route
		ON support_email_routes(workspace_id)
		WHERE mailbox_id IS NULL AND active = true
		`,
		`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_ser_active_mailbox_route
		ON support_email_routes(mailbox_id)
		WHERE mailbox_id IS NOT NULL AND active = true
		`,
		`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_sesd_active_workspace
		ON support_email_sender_domains(workspace_id)
		WHERE active = true
		`,
		`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_ses_active_workspace_default
		ON support_email_senders(workspace_id)
		WHERE active = true AND default_scope = 'workspace'
		`,
		`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_ses_active_mailbox_default
		ON support_email_senders(mailbox_id)
		WHERE active = true AND default_scope = 'mailbox' AND mailbox_id IS NOT NULL
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_sel_rfc_message_id
		ON support_email_logs(workspace_id, rfc_message_id)
		WHERE rfc_message_id IS NOT NULL AND rfc_message_id <> ''
		`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migrate support email route schema: %w", err)
		}
	}
	return nil
}
