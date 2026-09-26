package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSupportConversationDeleteIfEmpty(t *testing.T) {
	for _, tc := range []struct {
		name        string
		messageType string
		workspaceID string
		wantDeleted bool
	}{
		{"empty", "", "workspace", true},
		{"system notice only", "system", "workspace", true},
		{"real reply", "reply", "workspace", false},
		{"internal note", "note", "workspace", false},
		{"other workspace", "", "other", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSupportConversationMessageTestDB(t)
			if err := db.AutoMigrate(&model.SupportConversationTag{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("INSERT INTO support_conversations (id, workspace_id) VALUES (?, ?)", "conversation", "workspace").Error; err != nil {
				t.Fatal(err)
			}
			if tc.messageType != "" {
				if err := db.Exec("INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, content) VALUES (?, ?, ?, ?, ?, ?)", "message", "workspace", "conversation", "user", tc.messageType, "Content").Error; err != nil {
					t.Fatal(err)
				}
			}
			deleted, err := NewSupportConversationRepository(db).DeleteIfEmpty(context.Background(), tc.workspaceID, "conversation")
			if err != nil || deleted != tc.wantDeleted {
				t.Fatalf("deleted=%v want=%v err=%v", deleted, tc.wantDeleted, err)
			}
			var remaining int64
			if err := db.Model(&model.SupportConversation{}).Where("id = ?", "conversation").Count(&remaining).Error; err != nil {
				t.Fatal(err)
			}
			if (remaining == 0) != tc.wantDeleted {
				t.Fatalf("remaining conversations=%d", remaining)
			}
		})
	}
}

func TestSupportConversationDeleteIfEmptyRollsBackFailure(t *testing.T) {
	for _, table := range []string{"support_conversation_tags", "support_messages", "support_conversations"} {
		t.Run(table, func(t *testing.T) {
			db := setupSupportConversationMessageTestDB(t)
			if err := db.AutoMigrate(&model.SupportConversationTag{}); err != nil {
				t.Fatal(err)
			}
			for _, sql := range []string{
				"INSERT INTO support_conversations (id, workspace_id) VALUES ('conversation', 'workspace')",
				"INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, content) VALUES ('message', 'workspace', 'conversation', 'system', 'system', 'Teammate joined')",
				"INSERT INTO support_conversation_tags (conversation_id, tag_id) VALUES ('conversation', 'tag')",
				fmt.Sprintf("CREATE TRIGGER reject_cleanup BEFORE %s ON %s BEGIN SELECT RAISE(FAIL, 'cleanup failed'); END", map[string]string{"support_messages": "UPDATE", "support_conversations": "DELETE", "support_conversation_tags": "DELETE"}[table], table),
			} {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			deleted, err := NewSupportConversationRepository(db).DeleteIfEmpty(context.Background(), "workspace", "conversation")
			if err == nil || deleted {
				t.Fatalf("expected cleanup failure, deleted=%v err=%v", deleted, err)
			}
			for _, model := range []any{&model.SupportConversation{}, &model.SupportMessage{}, &model.SupportConversationTag{}} {
				var count int64
				if err := db.Model(model).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if count != 1 {
					t.Errorf("cleanup failed to roll back %T: count=%d", model, count)
				}
			}
		})
	}
}
