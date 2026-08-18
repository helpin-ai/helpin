package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSupportEmailLogRepositoryListByMessageIDsScopesHydration(t *testing.T) {
	dbName := fmt.Sprintf("file:support_email_log_page_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_email_logs (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		conversation_id TEXT NOT NULL,
		direction TEXT NOT NULL,
		message_ids TEXT,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create support email logs: %v", err)
	}

	repo := NewSupportEmailLogRepository(db)
	logs := []model.SupportEmailLog{
		{ID: "log-visible", WorkspaceID: "ws-1", ConversationID: "conv-1", Direction: "inbound", MessageIDs: model.DocsStringArray{"msg-visible"}},
		{ID: "log-old", WorkspaceID: "ws-1", ConversationID: "conv-1", Direction: "inbound", MessageIDs: model.DocsStringArray{"msg-old"}},
		{ID: "log-other-workspace", WorkspaceID: "ws-2", ConversationID: "conv-2", Direction: "inbound", MessageIDs: model.DocsStringArray{"msg-visible"}},
	}
	for i := range logs {
		if err := db.Exec(
			`INSERT INTO support_email_logs (id, workspace_id, conversation_id, direction, message_ids, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			logs[i].ID, logs[i].WorkspaceID, logs[i].ConversationID, logs[i].Direction, logs[i].MessageIDs, time.Now(),
		).Error; err != nil {
			t.Fatalf("create log %s: %v", logs[i].ID, err)
		}
	}

	got, err := repo.ListByMessageIDs(context.Background(), "ws-1", []string{"msg-visible"})
	if err != nil {
		t.Fatalf("list logs by message IDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != "log-visible" {
		t.Fatalf("logs = %#v, want only log-visible", got)
	}
}
