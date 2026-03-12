package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateCRMEmailAssociations applies idempotent schema changes required for
// message-participant associations and CRM email lifecycle support.
func MigrateCRMEmailAssociations(db *gorm.DB) error {
	const stmt = `
DO $$
BEGIN
    IF to_regclass('public.crm_email_accounts') IS NOT NULL THEN
        ALTER TABLE crm_email_accounts
            ADD COLUMN IF NOT EXISTS normalized_email_address VARCHAR(255);
        ALTER TABLE crm_email_accounts
            ADD COLUMN IF NOT EXISTS last_history_id VARCHAR(255);
        ALTER TABLE crm_email_accounts
            ADD COLUMN IF NOT EXISTS status VARCHAR(32) NOT NULL DEFAULT 'connected';
        ALTER TABLE crm_email_accounts
            ADD COLUMN IF NOT EXISTS disconnected_at TIMESTAMPTZ;

        UPDATE crm_email_accounts
        SET status = CASE
            WHEN oauth_state IS NOT NULL
                OR LOWER(TRIM(COALESCE(email_address, ''))) = 'pending@oauth.local'
                OR COALESCE(sync_state->>'status', '') = 'pending_oauth'
                THEN 'pending_oauth'
            WHEN is_active THEN 'connected'
            ELSE 'disconnected'
        END
        WHERE COALESCE(status, '') = '';

        UPDATE crm_email_accounts
        SET normalized_email_address = CASE
            WHEN status = 'pending_oauth'
                OR TRIM(COALESCE(email_address, '')) = ''
                OR LOWER(TRIM(COALESCE(email_address, ''))) = 'pending@oauth.local'
                THEN NULL
            ELSE LOWER(TRIM(email_address))
        END
        WHERE normalized_email_address IS NULL
            OR normalized_email_address = '';

        UPDATE crm_email_accounts
        SET last_history_id = NULLIF(COALESCE(last_history_id, sync_state->>'last_history_id'), '')
        WHERE last_history_id IS NULL;

        UPDATE crm_email_accounts
        SET disconnected_at = COALESCE(disconnected_at, updated_at)
        WHERE status = 'disconnected' AND disconnected_at IS NULL;

        CREATE INDEX IF NOT EXISTS idx_crm_email_accounts_workspace_member
            ON crm_email_accounts (workspace_id, member_id);
        CREATE INDEX IF NOT EXISTS idx_crm_email_accounts_workspace_provider_email
            ON crm_email_accounts (workspace_id, provider, normalized_email_address);
        CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_email_accounts_workspace_provider_normalized_email_unique
            ON crm_email_accounts (workspace_id, provider, normalized_email_address)
            WHERE normalized_email_address IS NOT NULL;
    END IF;

    IF to_regclass('public.crm_email_message_contacts') IS NOT NULL THEN
        CREATE INDEX IF NOT EXISTS idx_crm_email_message_contacts_workspace_contact
            ON crm_email_message_contacts (workspace_id, contact_id);
        CREATE INDEX IF NOT EXISTS idx_crm_email_message_contacts_message
            ON crm_email_message_contacts (message_id);
    END IF;

    IF to_regclass('public.crm_contacts') IS NOT NULL THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_contacts_workspace_email_lower_unique
            ON crm_contacts (workspace_id, LOWER(email))
            WHERE email IS NOT NULL;
    END IF;
END $$;`

	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("migrate crm email associations: %w", err)
	}
	return nil
}
