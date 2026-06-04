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

func setupSupportEventsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_events_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_events (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		conversation_id TEXT,
		message_id TEXT,
		widget_session_id TEXT,
		anonymous_id TEXT,
		document_id TEXT,
		article_id TEXT,
		article_public_id TEXT,
		actor_type TEXT NOT NULL DEFAULT '',
		channel TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT '',
		issue_key TEXT NOT NULL DEFAULT '',
		issue_summary TEXT NOT NULL DEFAULT '',
		failure_mode TEXT NOT NULL DEFAULT '',
		source_signal TEXT NOT NULL DEFAULT '',
		can_answer TEXT,
		can_resolve TEXT,
		metadata TEXT NOT NULL DEFAULT '{}',
		occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestSupportEventRepository_Create(t *testing.T) {
	db := setupSupportEventsTestDB(t)
	repo := NewSupportEventRepository(db)
	ctx := context.Background()

	event := &model.SupportEvent{
		ID:          "evt-1",
		WorkspaceID: "ws-1",
		EventType:   model.SupportEventAIHandoffTriggered,
		IssueKey:    "billing_refund",
		FailureMode: model.SupportCoverageFailureNoRetrieval,
		OccurredAt:  time.Now(),
	}
	if err := repo.Create(ctx, event); err != nil {
		t.Fatalf("Create: %v", err)
	}

	events, err := repo.List(ctx, "ws-1", model.SupportEventFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != model.SupportEventAIHandoffTriggered {
		t.Errorf("expected event type %q, got %q", model.SupportEventAIHandoffTriggered, events[0].EventType)
	}
}

func TestSupportEventRepository_CreateRequiresWorkspace(t *testing.T) {
	db := setupSupportEventsTestDB(t)
	repo := NewSupportEventRepository(db)

	err := repo.Create(context.Background(), &model.SupportEvent{
		EventType:  model.SupportEventAIAnswerSent,
		OccurredAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestSupportEventRepository_ListFiltersByType(t *testing.T) {
	db := setupSupportEventsTestDB(t)
	repo := NewSupportEventRepository(db)
	ctx := context.Background()

	for i, et := range []string{model.SupportEventAIAnswerSent, model.SupportEventAIHandoffTriggered, model.SupportEventAIAnswerSent} {
		repo.Create(ctx, &model.SupportEvent{
			ID:          fmt.Sprintf("evt-%d", i),
			WorkspaceID: "ws-1",
			EventType:   et,
			OccurredAt:  time.Now(),
		})
	}

	events, err := repo.List(ctx, "ws-1", model.SupportEventFilter{EventType: model.SupportEventAIAnswerSent})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestSupportEventRepository_ListScopedToWorkspace(t *testing.T) {
	db := setupSupportEventsTestDB(t)
	repo := NewSupportEventRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &model.SupportEvent{ID: "evt-a", WorkspaceID: "ws-1", EventType: "test", OccurredAt: time.Now()})
	repo.Create(ctx, &model.SupportEvent{ID: "evt-b", WorkspaceID: "ws-2", EventType: "test", OccurredAt: time.Now()})

	events, _ := repo.List(ctx, "ws-1", model.SupportEventFilter{})
	if len(events) != 1 {
		t.Fatalf("expected 1 event for ws-1, got %d", len(events))
	}
}
