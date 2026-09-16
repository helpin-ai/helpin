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
func TestSupportEmailNoticePostgresProjections(t *testing.T) {
	dsn := os.Getenv("SUPPORT_EMAIL_NOTICE_TEST_DSN")
	if dsn == "" {
		t.Skip("SUPPORT_EMAIL_NOTICE_TEST_DSN required")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "email_notice_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	migrate("202609020005_support_inbox_state_projection_triggers.sql")
	migrate("202609020007_support_inbox_core_counters.sql")
	migrate("202609110002_support_waiting_view.sql")
	for range 2 {
		migrate("202609150001_support_email_notices.sql")
	}

	ws, user, conv := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO workspaces VALUES (?)`, ws)
	exec(`INSERT INTO users VALUES (?)`, user)
	exec(`INSERT INTO support_conversations(id,workspace_id,display_id,subject,status,assigned_user_id) VALUES (?,?,1,'Account help','open',?)`, conv, ws, user)
	repo := repository.NewSupportMessageRepository(db)
	ctx := context.Background()
	for _, sender := range []string{"customer", "user"} {
		if err := repo.Create(ctx, &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv, SenderType: sender, MessageType: "reply", Content: "Real conversation", Metadata: "{}"}); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() string {
		t.Helper()
		var value string
		err := db.Raw(`SELECT jsonb_build_object('conversation',(SELECT to_jsonb(c) FROM support_conversations c WHERE id=?),'personal',(SELECT jsonb_agg(s ORDER BY user_id) FROM support_conversation_user_states s WHERE conversation_id=?),'counters',(SELECT jsonb_agg(b ORDER BY bucket_id,audience_type,audience_id) FROM support_inbox_counter_buckets b WHERE workspace_id=?),'changes',(SELECT count(*) FROM support_inbox_conversation_changes WHERE conversation_id=?))::text`, conv, conv, ws, conv).Scan(&value).Error
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, status := range []string{"open", "waiting_on_customer", "resolved"} {
		exec(`UPDATE support_conversations SET status=? WHERE id=?`, status, conv)
		before := snapshot()
		notice := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv, SenderType: "customer", MessageType: model.SupportMessageTypeEmailNotice, Content: "I am out of the office.", Metadata: `{"email_notice_kind":"out_of_office"}`}
		if err := repo.Create(ctx, notice); err != nil {
			t.Fatal(err)
		}
		if after := snapshot(); before != after {
			t.Fatalf("%s notice changed projections\nbefore: %s\nafter: %s", status, before, after)
		}
	}
	// The migration must not disable ordinary customer-reply projection.
	exec(`UPDATE support_conversations SET status='open' WHERE id=?`, conv)
	real := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: conv, SenderType: "customer", MessageType: "reply", Content: "I'm back, please help.", Metadata: "{}"}
	if err := repo.Create(ctx, real); err != nil {
		t.Fatal(err)
	}
	var c model.SupportConversation
	if err := db.First(&c, "id=?", conv).Error; err != nil {
		t.Fatal(err)
	}
	if !c.CustomerAwaitingResponse || !c.NeedsHumanReply || derefString(c.LastCustomerMessageID) != real.ID {
		t.Fatalf("real reply lost workload: %+v", c)
	}
	var unread int
	if err := db.Raw(`SELECT unread_customer_message_count FROM support_conversation_user_states WHERE conversation_id=? AND user_id=?`, conv, user).Scan(&unread).Error; err != nil {
		t.Fatal(err)
	}
	if unread != 2 {
		t.Fatalf("unread=%d, want two real customer messages", unread)
	}
}
