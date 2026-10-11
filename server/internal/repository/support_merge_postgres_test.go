//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestMergePostgresConcurrentRouting(t *testing.T) {
	db := contactPrivacyDB(t)
	ctx := context.Background()
	ws := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	actor := uuid.NewString()
	privacyExec(t, db, `INSERT INTO users(id,email,full_name,password_hash) VALUES (?,?,'Test teammate','test')`, actor, actor+"@example.invalid")
	email := "sam@example.invalid"
	a := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Original A", Status: "open", Source: "widget", CustomerEmail: &email}
	b := model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Original B", Status: "open", Source: "widget", CustomerEmail: &email}
	for _, c := range []*model.SupportConversation{&a, &b} {
		if err := NewSupportConversationRepository(db).Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewSupportMergeRepository(db)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, pair := range [][2]string{{a.ID, b.ID}, {b.ID, a.ID}} {
		wg.Add(1)
		go func(pair [2]string) {
			defer wg.Done()
			<-start
			_, err := repo.Merge(ctx, ws, pair[0], []string{pair[1]}, actor, uuid.NewString(), false)
			results <- err
		}(pair)
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("opposing merges: %d successful, want one", success)
	}
	edges, err := repo.Active(ctx, ws)
	if err != nil || len(edges) != 1 {
		t.Fatalf("edges=%v err=%v", edges, err)
	}
	edge := edges[0]
	var sourceCount int64
	if err := db.Table("support_inbox_counter_contributions").Where("conversation_id = ? AND total_count > 0", edge.SourceID).Count(&sourceCount).Error; err != nil || sourceCount != 0 {
		t.Fatalf("merged source still in inbox counters: %d %v", sourceCount, err)
	}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := model.SupportMessage{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: edge.SourceID, SenderType: "customer", MessageType: "reply", Content: "Concurrent incoming", Metadata: "{}"}
			if err := NewSupportMessageRepository(db).Create(ctx, &msg); err != nil {
				t.Error(err)
			}
		}()
	}
	if err := repo.Unmerge(ctx, ws, edge.ID, actor, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if err := db.Table("support_inbox_counter_contributions").Where("conversation_id = ? AND total_count > 0", edge.SourceID).Count(&sourceCount).Error; err != nil || sourceCount == 0 {
		t.Fatalf("restored source missing from inbox counters: %d %v", sourceCount, err)
	}
	var auditCount int64
	if err := db.Table("support_events").Where("workspace_id = ? AND event_type IN ?", ws, []string{model.SupportEventConversationMerged, model.SupportEventConversationUnmerged}).Count(&auditCount).Error; err != nil || auditCount != 2 {
		t.Fatalf("routing audit events: %d %v", auditCount, err)
	}
	var messages []model.SupportMessage
	if err := db.Where("workspace_id = ? AND sender_type = 'customer'", ws).Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if len(messages) != 12 {
		t.Fatalf("lost messages: %d", len(messages))
	}
	for _, msg := range messages {
		var grants int64
		db.Model(&model.SupportMergeMessage{}).Where("message_id = ?", msg.ID).Count(&grants)
		if msg.ConversationID == edge.TargetID && grants != 1 {
			t.Errorf("routed message lost history grant: %s", msg.ID)
		}
		if msg.ConversationID == edge.SourceID && grants != 0 {
			t.Errorf("post-unmerge message gained shared audience: %s", msg.ID)
		}
	}
}
