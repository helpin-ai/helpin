//go:build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSupportConversationPostgresWithoutHumanUser(t *testing.T) {
	dsn := os.Getenv("SUPPORT_CONTEXT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SUPPORT_CONTEXT_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Error(err)
		}
	})
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() {
		if err := tx.Rollback().Error; err != nil {
			t.Error(err)
		}
	})
	schema := "support_test_" + uuid.NewString()[:8]
	if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.AutoMigrate(&model.SupportConversation{}, &model.SupportMessage{}, &model.SupportMailbox{}, &model.SupportConversationUserState{}, &model.SupportWidgetSession{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`CREATE TABLE support_inbox_state_rollouts (workspace_id uuid, mode text)`).Error; err != nil {
		t.Fatal(err)
	}
	ws, id := uuid.NewString(), uuid.NewString()
	if err := tx.Exec(`INSERT INTO support_conversations (id,workspace_id,display_id,subject,status,support_state_version) VALUES (?, ?, 1, 'Failed uploads', 'resolved', 1)`, id, ws).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSupportConversationRepository(tx)
	ctx := authorization.WithSupportAgentScope(context.Background(), authorization.SupportAgentScope{WorkspaceID: ws, AgentID: uuid.NewString()})
	rows, total, err := repo.List(ctx, ConversationRepositoryListParams{ConversationListParams: ConversationListParams{WorkspaceID: ws}})
	if err != nil {
		t.Fatalf("list without human user: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != id {
		t.Fatalf("missing earlier resolved conversation: %#v total %d", rows, total)
	}
	if rows[0].UnreadCount != 0 || rows[0].PersonalStateVersion != 0 {
		t.Fatal("system reader has personal state")
	}
	row, err := repo.GetByIDForUser(ctx, ws, id, "", "", "")
	if err != nil || row == nil {
		t.Fatalf("detail without human user: %v", err)
	}
}
