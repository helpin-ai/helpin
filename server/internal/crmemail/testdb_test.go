package crmemail

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCRMEmailTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:crmemail_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE crm_contacts (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			display_id TEXT NOT NULL,
			first_name TEXT NOT NULL,
			last_name TEXT,
			email TEXT,
			phone TEXT,
			job_title TEXT,
			lifecycle_stage TEXT NOT NULL DEFAULT 'subscriber',
			lead_status TEXT NOT NULL DEFAULT 'new',
			owner_member_id TEXT,
			avatar_url TEXT,
			source TEXT,
			custom_properties BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX idx_crm_contacts_workspace_email_lower_unique
			ON crm_contacts(workspace_id, lower(email))
			WHERE email IS NOT NULL`,
		`CREATE TABLE crm_email_accounts (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			member_id TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'gmail',
			email_address TEXT NOT NULL,
			normalized_email_address TEXT,
			access_token_encrypted TEXT,
			refresh_token_encrypted TEXT,
			sync_state BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
			last_history_id TEXT,
			last_synced_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'connected',
			disconnected_at DATETIME,
			oauth_state TEXT,
			token_expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX idx_crm_email_accounts_workspace_provider_normalized_email_unique
			ON crm_email_accounts(workspace_id, provider, normalized_email_address)
			WHERE normalized_email_address IS NOT NULL`,
		`CREATE TABLE crm_email_threads (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			thread_external_id TEXT NOT NULL,
			subject TEXT NOT NULL,
			last_message_at DATETIME NOT NULL,
			message_count INTEGER NOT NULL DEFAULT 0,
			contact_ids BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			deal_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_email_messages (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			thread_id TEXT,
			message_external_id TEXT,
			from_address TEXT NOT NULL,
			from_name TEXT,
			to_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			cc_addresses BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			subject TEXT,
			body_text TEXT,
			body_html TEXT,
			direction TEXT NOT NULL DEFAULT 'inbound',
			sent_at DATETIME NOT NULL,
			contact_id TEXT,
			deal_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE crm_email_message_contacts (
			message_id TEXT NOT NULL,
			contact_id TEXT NOT NULL,
			participant_role TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, contact_id, participant_role)
		)`,
		`CREATE TABLE crm_email_sync_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			workspace_id TEXT NOT NULL UNIQUE,
			historical_sync_days INTEGER NOT NULL DEFAULT 90,
			filter_mode TEXT NOT NULL DEFAULT 'blocklist',
			filter_patterns BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			internal_exclusion TEXT NOT NULL DEFAULT 'none',
			include_private_meetings BOOLEAN NOT NULL DEFAULT 0,
			include_solo_meetings BOOLEAN NOT NULL DEFAULT 0,
			record_creation_mode TEXT NOT NULL DEFAULT 'selective',
			blocked_record_prefixes BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply schema statement %q: %v", stmt, err)
		}
	}

	return db
}
