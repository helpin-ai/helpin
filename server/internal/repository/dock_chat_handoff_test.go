package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func handoffTestRepository(t *testing.T) *DockChatHandoffRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:handoff-%d?mode=memory&cache=shared&_foreign_keys=on", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, query := range []string{
		`CREATE TABLE dock_chats( flow_builder TEXT,id text PRIMARY KEY, workspace_id text, archived_at datetime)`,
		`CREATE TABLE dock_chat_handoffs(workspace_id text, dock_chat_id text REFERENCES dock_chats(id) ON DELETE CASCADE, format_version integer, revision integer, previous_revision integer, covered_sequence integer, access_scope text, payload blob, generator text, prompt_version text, lease_token text, lease_through_sequence integer NOT NULL DEFAULT 0, lease_expires_at datetime, failure_code text, updated_at datetime, PRIMARY KEY(workspace_id,dock_chat_id))`,
		`CREATE TABLE agent_run_messages(id text PRIMARY KEY, workspace_id text, dock_chat_id text, dock_chat_sequence integer, delivery_status text)`,
		`INSERT INTO dock_chats (id, workspace_id, archived_at) VALUES ('chat','ws',NULL),('foreign','other',NULL)`,
		`INSERT INTO agent_run_messages VALUES ('first','ws','chat',1,'sent'),('correction','ws','chat',2,'sent'),('secret','other','foreign',1,'sent')`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := NewDockChatHandoffRepository(db)
	r.now = func() time.Time { return time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC) }
	return r
}

func handoffContent(id string) model.DockChatHandoffContent {
	return model.DockChatHandoffContent{Objective: []model.HandoffClaim{{Text: "Document the service; do not deploy", SourceIDs: []string{id}}}}
}

func TestHandoffLeaseRevisionAndFailure(t *testing.T) {
	r, ctx := handoffTestRepository(t), context.Background()
	lease, err := r.Acquire(ctx, "ws", "chat", "scope-v1", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Acquire(ctx, "ws", "chat", "scope-v1", 0, time.Minute); !errors.Is(err, ErrHandoffConflict) {
		t.Fatalf("competing generator: %v", err)
	}
	if err := r.Publish(ctx, *lease, 1, handoffContent("first"), "model-test", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(ctx, *lease, 2, handoffContent("correction"), "model-test", "v1"); !errors.Is(err, ErrHandoffConflict) {
		t.Fatalf("stale generator: %v", err)
	}
	state, err := r.Get(ctx, "ws", "chat")
	if err != nil || state.Revision != 1 || state.CoveredSequence != 1 || state.PreviousRevision != 0 {
		t.Fatalf("state: %+v %v", state, err)
	}
	before := string(state.Payload)
	lease, err = r.Acquire(ctx, "ws", "chat", "scope-v1", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Fail(ctx, *lease, "generation_failed"); err != nil {
		t.Fatal(err)
	}
	state, err = r.Get(ctx, "ws", "chat")
	if err != nil || state.Revision != 1 || state.CoveredSequence != 1 || string(state.Payload) != before || state.FailureCode != "generation_failed" {
		t.Fatalf("failure advanced coverage: %+v %v", state, err)
	}
}

func TestHandoffInvalidationRevokesLeaseAndErasesText(t *testing.T) {
	r, ctx := handoffTestRepository(t), context.Background()
	lease, err := r.Acquire(ctx, "ws", "chat", "scope-v1", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(ctx, *lease, 1, handoffContent("first"), "model", "v1"); err != nil {
		t.Fatal(err)
	}
	lease, err = r.Acquire(ctx, "ws", "chat", "scope-v1", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Invalidate(ctx, "ws", "chat", "scope-v2", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.Publish(ctx, *lease, 2, handoffContent("correction"), "model", "v1"); !errors.Is(err, ErrHandoffConflict) {
		t.Fatalf("revoked writer: %v", err)
	}
	if _, err := r.Acquire(ctx, "ws", "chat", "scope-v1", 2, time.Minute); !errors.Is(err, ErrHandoffConflict) {
		t.Fatalf("old access snapshot: %v", err)
	}
	state, err := r.Get(ctx, "ws", "chat")
	if err != nil || len(state.Payload) != 0 || state.CoveredSequence != 0 || state.Revision != 2 {
		t.Fatalf("invalidation: %+v %v", state, err)
	}
	if _, err := r.Acquire(ctx, "ws", "chat", "scope-v2", 2, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := r.db.Exec("DELETE FROM dock_chats WHERE id = 'chat'").Error; err != nil {
		t.Fatal(err)
	}
	if state, err := r.Get(ctx, "ws", "chat"); err != nil || state != nil {
		t.Fatalf("deleted chat retained memory: %+v %v", state, err)
	}
}

func TestHandoffExpiredLeaseCannotPublishOrReleaseReplacement(t *testing.T) {
	r, ctx := handoffTestRepository(t), context.Background()
	lease, err := r.Acquire(ctx, "ws", "chat", "scope", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	later := r.now().Add(time.Minute)
	r.now = func() time.Time { return later }
	if err := r.Publish(ctx, *lease, 1, handoffContent("first"), "model", "v1"); !errors.Is(err, ErrHandoffConflict) {
		t.Fatalf("expired publish: %v", err)
	}
	next, err := r.Acquire(ctx, "ws", "chat", "scope", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Fail(ctx, *lease, "generation_failed"); !errors.Is(err, ErrHandoffConflict) {
		t.Fatalf("stale release: %v", err)
	}
	if err := r.Publish(ctx, *next, 2, handoffContent("correction"), "model", "v1"); err != nil {
		t.Fatal(err)
	}
}

func TestHandoffRejectsForeignSourcesAndOversizedNarrative(t *testing.T) {
	r, ctx := handoffTestRepository(t), context.Background()
	if _, err := r.Acquire(ctx, "ws", "foreign", "scope", 0, time.Minute); err == nil {
		t.Fatal("foreign chat accepted")
	}
	lease, err := r.Acquire(ctx, "ws", "chat", "scope", 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []model.DockChatHandoffContent{
		{},
		handoffContent("secret"), handoffContent("missing"), handoffContent("correction"),
		{Objective: []model.HandoffClaim{{Text: "unsourced action claim"}}},
		{Objective: []model.HandoffClaim{{Text: strings.Repeat("x", MaxHandoffBytes), Assumption: true}}},
	} {
		if err := r.Publish(ctx, *lease, 1, content, "model", "v1"); err == nil {
			t.Fatalf("invalid handoff accepted: %+v", content)
		}
	}
	if state, err := r.Get(ctx, "other", "chat"); err != nil || state != nil {
		t.Fatalf("foreign read: %+v %v", state, err)
	}
	state, err := r.Get(ctx, "ws", "chat")
	if err != nil || state.Revision != 0 || state.CoveredSequence != 0 {
		t.Fatalf("invalid writes changed state: %+v %v", state, err)
	}
}
