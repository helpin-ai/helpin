package repository

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// The channel schema's constraints carry the dedupe and one-live-bot rules
// that later slices rely on, so they are pinned here.
func TestSupportChannelSchemaConstraintsPostgres(t *testing.T) {
	db := openWorkspaceDeletePostgres(t)
	ownerID := uuid.NewString()
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name) VALUES (?, ?, 'x', 'Owner')`, ownerID, "owner-"+ownerID[:8]+"@example.com")
	wsA, wsB := uuid.NewString(), uuid.NewString()
	for i, ws := range []string{wsA, wsB} {
		mustExec(t, db, `INSERT INTO workspaces (id, name, slug, workspace_key, owner_id) VALUES (?, 'Channels', ?, ?, ?)`,
			ws, "channels-"+ws[:8], []string{"CHA", "CHB"}[i], ownerID)
	}
	bot := "bot-" + uuid.NewString()
	insertAccount := func(ws string) error {
		return db.Exec(`INSERT INTO support_channel_accounts (workspace_id, provider, external_account_id, token_encrypted, webhook_secret_hash)
			VALUES (?, 'telegram', ?, 'enc', 'hash')`, ws, bot).Error
	}

	t.Run("a bot is live in one workspace only", func(t *testing.T) {
		withSavepoint(t, db, func() {
			if err := insertAccount(wsA); err != nil {
				t.Fatal(err)
			}
			if err := insertAccount(wsB); err == nil {
				t.Fatal("the same bot connected live in two workspaces")
			}
		})
	})

	t.Run("a disconnected bot can be connected elsewhere", func(t *testing.T) {
		withSavepoint(t, db, func() {
			if err := insertAccount(wsA); err != nil {
				t.Fatal(err)
			}
			mustExec(t, db, `UPDATE support_channel_accounts SET disabled_at = now(), status = 'disabled' WHERE workspace_id = ?`, wsA)
			if err := insertAccount(wsB); err != nil {
				t.Fatalf("reconnect after disconnect: %v", err)
			}
		})
	})

	t.Run("unknown providers and statuses are rejected", func(t *testing.T) {
		withSavepoint(t, db, func() {
			if err := db.Exec(`INSERT INTO support_channel_accounts (workspace_id, provider, external_account_id, token_encrypted, webhook_secret_hash)
				VALUES (?, 'x', 'bot', 'enc', 'hash')`, wsA).Error; err == nil {
				t.Fatal("accepted an unknown provider")
			}
		})
		withSavepoint(t, db, func() {
			if err := db.Exec(`INSERT INTO support_channel_accounts (workspace_id, provider, external_account_id, token_encrypted, webhook_secret_hash, status)
				VALUES (?, 'telegram', 'bot', 'enc', 'hash', 'paused')`, wsA).Error; err == nil {
				t.Fatal("accepted an unknown status")
			}
		})
	})

	t.Run("an external message key is stored once per account", func(t *testing.T) {
		withSavepoint(t, db, func() {
			account, conversation := uuid.NewString(), uuid.NewString()
			mustExec(t, db, `INSERT INTO support_conversations (id, workspace_id, display_id, subject) VALUES (?, ?, 1, 'Telegram')`, conversation, wsA)
			insert := func(key string) error {
				return db.Exec(`INSERT INTO support_messages (workspace_id, conversation_id, sender_type, content, channel_account_id, external_message_key)
					VALUES (?, ?, 'customer', 'Hi', ?, ?)`, wsA, conversation, account, key).Error
			}
			if err := insert("100:7"); err != nil {
				t.Fatal(err)
			}
			if err := insert("200:7"); err != nil {
				t.Fatalf("same message id in another chat: %v", err)
			}
			if err := insert("100:7"); err == nil {
				t.Fatal("stored a duplicate provider message")
			}
		})
	})
}

func mustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// withSavepoint runs fn and rolls back to before it, so an expected
// constraint error does not abort the surrounding test transaction.
func withSavepoint(t *testing.T, db *gorm.DB, fn func()) {
	t.Helper()
	name := "sp_" + uuid.NewString()[:8]
	if err := db.SavePoint(name).Error; err != nil {
		t.Fatal(err)
	}
	fn()
	if err := db.RollbackTo(name).Error; err != nil {
		t.Fatal(err)
	}
}
