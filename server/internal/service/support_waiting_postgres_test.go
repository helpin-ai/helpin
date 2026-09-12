//go:build integration

package service

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Uses a disposable schema. Never run migrations against the supplied database's application schema.
func TestSupportWaitingPostgresCounters(t *testing.T) {
	dsn := os.Getenv("SUPPORT_WAITING_TEST_DSN")
	if dsn == "" {
		t.Skip("SUPPORT_WAITING_TEST_DSN required")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "waiting_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		sqlDB, err := base.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	exec := func(sql string, args ...any) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE workspaces(id uuid PRIMARY KEY); CREATE TABLE users(id uuid PRIMARY KEY); CREATE TABLE support_inbox_views(id uuid PRIMARY KEY)`)
	if err := db.AutoMigrate(&model.SupportConversation{}, &model.SupportMessage{}); err != nil {
		t.Fatal(err)
	}
	migrate := func(name string) {
		t.Helper()
		migration, err := os.ReadFile("../dbmigrate/sql/" + name)
		if err != nil {
			t.Fatal(err)
		}
		exec(string(migration))
	}
	migrate("202609020003_support_inbox_state_foundation.sql")
	migrate("202609020007_support_inbox_core_counters.sql")
	ws, user, conv, legacy, mailbox := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO workspaces VALUES (?)`, ws)
	exec(`INSERT INTO users VALUES (?)`, user)
	for _, row := range []struct{ id, status, sender string }{{conv, "open", "user"}, {legacy, "waiting_on_customer", "user"}} {
		exec(`INSERT INTO support_conversations(id, workspace_id, display_id, subject, status, last_public_sender_type, customer_awaiting_response, needs_human_reply, assigned_user_id, support_state_version)
    VALUES (?, ?, 1, 'Waiting test', ?, ?, FALSE, FALSE, ?, 1)`, row.id, ws, row.status, row.sender, user)
		exec(`INSERT INTO support_conversation_user_states(workspace_id, conversation_id, user_id, manually_unread, relevance_mask) VALUES (?, ?, ?, TRUE, 1)`, ws, row.id, user)
	}
	exec(`INSERT INTO support_inbox_state_rollouts(workspace_id, mode) VALUES (?, 'v2')`, ws)
	repo := repository.NewSupportConversationRepository(db)
	check := func(total, unread int, scope *string) {
		t.Helper()
		// Both the production counter read and rollout fallback must agree.
		for _, mode := range []string{"v2", "legacy"} {
			exec(`UPDATE support_inbox_state_rollouts SET mode=? WHERE workspace_id=?`, mode, ws)
			stats, err := repo.GetUnreadStats(context.Background(), ws, user, "", model.RoleOwner, scope)
			if err != nil {
				t.Fatal(err)
			}
			if stats.WaitingTotal != total || (mode == "v2" && stats.Waiting != unread) || stats.WaitingNeedsHumanReply != 0 {
				t.Fatalf("%s stats: %+v, want total %d unread %d", mode, stats, total, unread)
			}
		}
	}
	// Existing legacy counters only know about the explicit Waiting status.
	var before int
	if err := db.Raw(`SELECT total_count FROM support_inbox_counter_buckets WHERE bucket_id='waiting' AND audience_type='shared'`).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	if before != 1 {
		t.Fatalf("pre-migration total = %d", before)
	}
	historical := uuid.NewString()
	exec(`INSERT INTO support_conversations(id,workspace_id,display_id,subject,status,support_state_version) VALUES (?, ?, 2, 'Historical', 'open', 0)`, historical, ws)
	exec(`INSERT INTO support_messages(id,workspace_id,conversation_id,sender_type,message_type,content,is_internal,created_at) VALUES (?, ?, ?, 'user','reply','Public answer',FALSE,'2026-09-11T09:00:00Z'), (?, ?, ?, 'user','reply','Internal note',TRUE,'2026-09-11T09:01:00Z')`, uuid.NewString(), ws, historical, uuid.NewString(), ws, historical)
	migrate("202609110002_support_waiting_view.sql")
	check(3, 2, nil)
	var historicalRow model.SupportConversation
	if err := db.First(&historicalRow, "id = ?", historical).Error; err != nil {
		t.Fatal(err)
	}
	if historicalRow.LastPublicSenderType == nil || *historicalRow.LastPublicSenderType != "user" || historicalRow.Status != "open" || historicalRow.SupportStateVersion != 0 {
		t.Fatalf("historical public projection: %+v", historicalRow)
	}
	exec(`UPDATE support_conversations SET status='resolved' WHERE id=?`, historical)

	for range 2 {
		migrate("202609110002_support_waiting_view.sql")
		check(2, 2, nil)
	}
	exec(`UPDATE support_conversation_user_states SET manually_unread=FALSE WHERE conversation_id=?`, conv)
	check(2, 1, nil)
	// A sender-only update must trigger counter maintenance, even with awaiting=false on both sides.
	exec(`UPDATE support_conversations SET last_public_sender_type='ai' WHERE id=?`, conv)
	check(1, 1, nil)
	exec(`UPDATE support_conversations SET last_public_sender_type='user' WHERE id=?`, conv)
	check(2, 1, nil)
	for range 2 {
		exec(`UPDATE support_conversations SET last_public_sender_type='customer', customer_awaiting_response=TRUE, needs_human_reply=TRUE WHERE id=?`, conv)
		check(1, 1, nil)
		exec(`UPDATE support_conversations SET last_public_sender_type='user', customer_awaiting_response=FALSE, needs_human_reply=FALSE WHERE id=?`, conv)
		check(2, 1, nil)
	}
	exec(`UPDATE support_conversations SET mailbox_id=? WHERE id=?`, mailbox, conv)
	check(1, 0, &mailbox)
	shared := ""
	check(1, 1, &shared)
	for _, status := range []string{"resolved", "spam", "open"} {
		exec(`UPDATE support_conversations SET status=? WHERE id=?`, status, conv)
		if status == "open" {
			check(2, 1, nil)
		} else {
			check(1, 1, nil)
		}
	}

}
