//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPMTriagePostgresSupportReviewLocksAndRollback(t *testing.T) {
	env := setupPMTriagePostgres(t)
	for _, statement := range []string{
		`CREATE TABLE support_inbox_views(id uuid PRIMARY KEY)`,
		`CREATE TABLE crm_associations (id uuid PRIMARY KEY,workspace_id uuid,from_object_type text,from_object_id uuid,to_object_type text,to_object_id uuid,association_label text,created_at timestamptz)`,
	} {
		if err := env.db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := env.db.AutoMigrate(&model.SupportConversation{}, &model.SupportMessage{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"202609020003_support_inbox_state_foundation.sql", "202609020005_support_inbox_state_projection_triggers.sql", "202609020007_support_inbox_core_counters.sql", "202609110002_support_waiting_view.sql", "202609150001_support_email_notices.sql"} {
		migration, err := os.ReadFile("../dbmigrate/sql/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.db.Exec(string(migration)).Error; err != nil {
			t.Fatal(err)
		}
	}
	conversationID := uuid.NewString()
	if err := env.db.Exec(`INSERT INTO support_conversations(id,workspace_id,subject,display_id) VALUES (?,?,?,1)`, conversationID, env.workspace, "Export fails").Error; err != nil {
		t.Fatal(err)
	}
	messageID := uuid.NewString()
	if err := env.db.Create(&model.SupportMessage{ID: messageID, WorkspaceID: env.workspace, ConversationID: conversationID, SenderType: "customer", MessageType: "reply", Content: "Export fails", Metadata: "{}"}).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSupportConversationRepository(env.db)
	ctx := context.Background()
	refusal := errors.New("review rejected")
	err := repo.LinkTaskReviewed(ctx, env.workspace, conversationID, env.task, func(conversation *model.SupportConversation, messages []model.SupportMessage, task *model.PMTask) error {
		// A second connection cannot change either locked row while validation and
		// mutation are in progress. A bounded server timeout avoids hanging tests.
		for _, table := range []string{"pm_tasks", "support_conversations"} {
			id := env.task
			if table == "support_conversations" {
				id = conversationID
			}
			lockErr := env.db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec(`SET LOCAL lock_timeout = '100ms'`).Error; err != nil {
					return err
				}
				return tx.Exec("UPDATE "+table+" SET updated_at = now() WHERE id = ?", id).Error
			})
			if lockErr == nil || !strings.Contains(lockErr.Error(), "55P03") {
				t.Errorf("%s was not locked: %v", table, lockErr)
			}
		}
		// Run the actual production projection triggers: public message edits and
		// inserts must also wait for the reviewed conversation transaction.
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{"UPDATE support_messages SET content = 'Changed issue' WHERE id = ?", []any{messageID}},
			{"INSERT INTO support_messages(id,workspace_id,conversation_id,sender_type,message_type,content,metadata) VALUES (?,?,?,'customer','reply','New evidence','{}')", []any{uuid.NewString(), env.workspace, conversationID}},
		} {
			lockErr := env.db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec(`SET LOCAL lock_timeout = '100ms'`).Error; err != nil {
					return err
				}
				return tx.Exec(statement.sql, statement.args...).Error
			})
			if lockErr == nil || !strings.Contains(lockErr.Error(), "55P03") {
				t.Errorf("public evidence was not serialized: %v", lockErr)
			}
		}
		return refusal
	})
	if !errors.Is(err, refusal) {
		t.Fatalf("validation error = %v", err)
	}
	var conversation model.SupportConversation
	if err := env.db.First(&conversation, "id = ?", conversationID).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.LinkedTaskID != nil {
		t.Fatal("refused review changed link")
	}
	if err := repo.LinkTaskReviewed(ctx, env.workspace, conversationID, env.task, func(*model.SupportConversation, []model.SupportMessage, *model.PMTask) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := env.db.First(&conversation, "id = ?", conversationID).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := env.db.Model(&model.CRMAssociation{}).Where("from_object_id = ? AND to_object_id = ?", conversationID, env.task).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.LinkedTaskID == nil || *conversation.LinkedTaskID != env.task || count != 1 {
		t.Fatal("link and association did not commit")
	}
}
