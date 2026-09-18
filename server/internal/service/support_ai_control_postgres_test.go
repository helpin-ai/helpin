//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Uses an isolated schema in the explicitly configured test database.
func TestSupportAIControlPostgresPublicationFence(t *testing.T) {
	_, db, run, episode, _ := setupFollowUpPostgres(t)
	if err := db.AutoMigrate(&model.AIMessageProcessing{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	source := model.SupportMessage{ID: uuid.NewString(), WorkspaceID: run.WorkspaceID, ConversationID: episode.ConversationID, SenderType: "customer", Content: "A new question", MessageType: "reply"}
	if err := repository.NewSupportMessageRepository(db).Create(ctx, &source); err != nil {
		t.Fatal(err)
	}
	turn := model.AIMessageProcessing{ID: uuid.NewString(), WorkspaceID: run.WorkspaceID, ConversationID: episode.ConversationID, SourceMessageID: source.ID, Status: "processing"}
	if err := db.Create(&turn).Error; err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	var conv model.SupportConversation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&conv, "id = ?", episode.ConversationID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&conv).Updates(map[string]any{"human_takeover": true, "ai_control_version": 1}).Error; err != nil {
		t.Fatal(err)
	}
	reply := &model.SupportMessage{ID: uuid.NewString(), WorkspaceID: run.WorkspaceID, ConversationID: episode.ConversationID, SenderType: "ai", MessageType: "reply", Content: "Too late"}
	type result struct {
		created bool
		err     error
	}
	done := make(chan result, 1)
	replyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	go func() {
		created, err := repository.NewAIMessageProcessingRepository(db).CreateReply(replyCtx, turn.ID, reply, run.ID)
		done <- result{created, err}
	}()
	select {
	case result := <-done:
		t.Fatalf("publication bypassed ownership lock: %+v", result)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || result.created {
			t.Fatalf("paused publication: %+v", result)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("publication did not finish")
	}
	var count int64
	if err := db.Model(&model.SupportMessage{}).Where("id = ?", reply.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("late reply persisted")
	}
	// Production follow-up cancellation trigger participates in the same commit.
	var saved model.SupportAIFollowUp
	if err := db.First(&saved, "id = ?", episode.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != "cancelled" {
		t.Fatalf("follow-up not cancelled: %s", saved.Status)
	}
}
