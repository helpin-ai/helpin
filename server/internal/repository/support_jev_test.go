package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gorm.io/gorm"
)

func setupSupportJevStore(t *testing.T) (*SupportJevRepository, *gorm.DB) {
	t.Helper()
	_, db := setupPMTriageStore(t)
	if err := db.Exec(`CREATE TABLE support_conversation_triage_events (
		id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL,
		triage_id TEXT, event_type TEXT NOT NULL, source TEXT, cached BOOLEAN DEFAULT false,
		from_mailbox_id TEXT, to_mailbox_id TEXT, actor_user_id TEXT, input_hash TEXT,
		payload TEXT NOT NULL DEFAULT '{}', created_at DATETIME NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return NewSupportJevRepository(db), db
}

func TestSupportJevAbandonedAttemptCannotSettle(t *testing.T) {
	repo, db := setupSupportJevStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, "workspace", "conversation", "hash", 3)
	if err != nil || !first.CallProvider {
		t.Fatalf("first admission=%+v %v", first, err)
	}
	pending, err := repo.Reserve(ctx, "workspace", "conversation", "hash", 3)
	if err != nil || pending.CallProvider || pending.Event.ID != first.Event.ID {
		t.Fatalf("pending request duplicated: %+v %v", pending, err)
	}
	if err := db.Model(first.Event).Update("created_at", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	retry, err := repo.Reserve(ctx, "workspace", "conversation", "hash", 3)
	if err != nil || !retry.CallProvider || retry.Event.ID == first.Event.ID {
		t.Fatalf("abandoned request did not retry: %+v %v", retry, err)
	}
	if err := repo.Finish(ctx, "workspace", first.Event.ID, json.RawMessage(`{"status":"ok"}`)); err == nil {
		t.Fatal("late result settled abandoned request")
	}
	if err := repo.Finish(ctx, "other", retry.Event.ID, json.RawMessage(`{"status":"ok"}`)); err == nil {
		t.Fatal("cross-workspace settlement accepted")
	}
	if err := repo.Finish(ctx, "workspace", retry.Event.ID, json.RawMessage(`{"status":"ok"}`)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Finish(ctx, "workspace", retry.Event.ID, json.RawMessage(`{"status":"provider_error"}`)); err == nil {
		t.Fatal("completed result overwritten")
	}
}

func TestSupportJevDailyCapIsSharedAcrossConversations(t *testing.T) {
	repo, _ := setupSupportJevStore(t)
	ctx := context.Background()
	first, err := repo.Reserve(ctx, "workspace", "conversation", "routing-hash", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Finish(ctx, "workspace", first.Event.ID, json.RawMessage(`{"status":"provider_error"}`)); err != nil {
		t.Fatal(err)
	}
	next, err := repo.Reserve(ctx, "workspace", "other-conversation", "tagging-hash", 1)
	if err != nil || !next.Limited || next.CallProvider {
		t.Fatalf("failed attempt escaped shared cap: %+v %v", next, err)
	}
}
