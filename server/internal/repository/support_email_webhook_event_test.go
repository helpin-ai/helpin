package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func setupWebhookEventTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:webhook_event_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.Exec(`CREATE TABLE support_email_webhook_events (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT,
		conversation_id TEXT,
		email_log_id TEXT,
		provider TEXT NOT NULL DEFAULT 'postmark',
		event_type TEXT NOT NULL,
		postmark_message_id TEXT,
		message_stream TEXT,
		raw_payload TEXT NOT NULL,
		received_at DATETIME,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	return db
}

func seedWebhookEvent(t *testing.T, db *gorm.DB, event *model.SupportEmailWebhookEvent) {
	t.Helper()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if event.Provider == "" {
		event.Provider = "postmark"
	}
	if err := db.Create(event).Error; err != nil {
		t.Fatalf("seed webhook event: %v", err)
	}
}

func TestWebhookEventCreate(t *testing.T) {
	db := setupWebhookEventTestDB(t)
	repo := NewSupportEmailWebhookEventRepository(db)
	ctx := context.Background()

	event := &model.SupportEmailWebhookEvent{
		Provider:   "postmark",
		EventType:  "inbound",
		RawPayload: `{"MessageID":"test-1"}`,
	}

	if err := repo.Create(ctx, event); err != nil {
		t.Fatalf("create: %v", err)
	}

	if event.ID == "" {
		t.Fatal("expected ID to be set after create")
	}
}

func TestWebhookEventGetByID(t *testing.T) {
	db := setupWebhookEventTestDB(t)
	repo := NewSupportEmailWebhookEventRepository(db)
	ctx := context.Background()

	event := &model.SupportEmailWebhookEvent{
		Provider:   "postmark",
		EventType:  "open",
		RawPayload: `{"MessageID":"test-get"}`,
	}
	seedWebhookEvent(t, db, event)

	got, err := repo.GetByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got == nil {
		t.Fatal("expected event, got nil")
	}
	if got.EventType != "open" {
		t.Fatalf("expected event_type=open, got %q", got.EventType)
	}
}

func TestWebhookEventGetByIDNotFound(t *testing.T) {
	db := setupWebhookEventTestDB(t)
	repo := NewSupportEmailWebhookEventRepository(db)
	ctx := context.Background()

	got, err := repo.GetByID(ctx, "nonexistent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for nonexistent id, got %+v", got)
	}
}

func TestWebhookEventListPaginated(t *testing.T) {
	db := setupWebhookEventTestDB(t)
	repo := NewSupportEmailWebhookEventRepository(db)
	ctx := context.Background()

	// Seed 5 events with different types and staggered times.
	wsID := "ws-1"
	base := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		eventType := "inbound"
		if i%2 == 0 {
			eventType = "open"
		}
		seedWebhookEvent(t, db, &model.SupportEmailWebhookEvent{
			WorkspaceID: &wsID,
			Provider:    "postmark",
			EventType:   eventType,
			RawPayload:  fmt.Sprintf(`{"index":%d}`, i),
			CreatedAt:   base.Add(time.Duration(i) * time.Minute),
		})
	}

	t.Run("page 1 default", func(t *testing.T) {
		result, err := repo.ListPaginated(ctx, 1, 3, "", "")
		if err != nil {
			t.Fatalf("list paginated: %v", err)
		}
		if result.Total != 5 {
			t.Fatalf("expected total=5, got %d", result.Total)
		}
		if result.TotalPages != 2 {
			t.Fatalf("expected total_pages=2, got %d", result.TotalPages)
		}
		if len(result.Data) != 3 {
			t.Fatalf("expected 3 items on page 1, got %d", len(result.Data))
		}
		// Should be newest first.
		if result.Data[0].CreatedAt.Before(result.Data[1].CreatedAt) {
			t.Fatal("expected newest first ordering")
		}
	})

	t.Run("page 2", func(t *testing.T) {
		result, err := repo.ListPaginated(ctx, 2, 3, "", "")
		if err != nil {
			t.Fatalf("list paginated: %v", err)
		}
		if len(result.Data) != 2 {
			t.Fatalf("expected 2 items on page 2, got %d", len(result.Data))
		}
	})

	t.Run("filter by event type", func(t *testing.T) {
		result, err := repo.ListPaginated(ctx, 1, 10, "open", "")
		if err != nil {
			t.Fatalf("list paginated: %v", err)
		}
		if result.Total != 3 {
			t.Fatalf("expected 3 open events, got %d", result.Total)
		}
		for _, e := range result.Data {
			if e.EventType != "open" {
				t.Fatalf("expected all events to be 'open', got %q", e.EventType)
			}
		}
	})

	t.Run("filter by provider", func(t *testing.T) {
		result, err := repo.ListPaginated(ctx, 1, 10, "", "postmark")
		if err != nil {
			t.Fatalf("list paginated: %v", err)
		}
		if result.Total != 5 {
			t.Fatalf("expected 5 postmark events, got %d", result.Total)
		}
	})

	t.Run("filter by nonexistent provider", func(t *testing.T) {
		result, err := repo.ListPaginated(ctx, 1, 10, "", "sendgrid")
		if err != nil {
			t.Fatalf("list paginated: %v", err)
		}
		if result.Total != 0 {
			t.Fatalf("expected 0 sendgrid events, got %d", result.Total)
		}
		if len(result.Data) != 0 {
			t.Fatalf("expected empty data, got %d items", len(result.Data))
		}
	})

	t.Run("empty result", func(t *testing.T) {
		result, err := repo.ListPaginated(ctx, 1, 10, "bounce", "")
		if err != nil {
			t.Fatalf("list paginated: %v", err)
		}
		if result.Total != 0 {
			t.Fatalf("expected 0 bounce events, got %d", result.Total)
		}
		if result.TotalPages != 0 {
			t.Fatalf("expected 0 total pages, got %d", result.TotalPages)
		}
	})
}

func TestWebhookEventListByConversation(t *testing.T) {
	db := setupWebhookEventTestDB(t)
	repo := NewSupportEmailWebhookEventRepository(db)
	ctx := context.Background()

	wsID := "ws-1"
	convID := "conv-1"
	otherConvID := "conv-2"

	seedWebhookEvent(t, db, &model.SupportEmailWebhookEvent{
		WorkspaceID:    &wsID,
		ConversationID: &convID,
		EventType:      "inbound",
		RawPayload:     `{"test":1}`,
	})
	seedWebhookEvent(t, db, &model.SupportEmailWebhookEvent{
		WorkspaceID:    &wsID,
		ConversationID: &convID,
		EventType:      "open",
		RawPayload:     `{"test":2}`,
	})
	seedWebhookEvent(t, db, &model.SupportEmailWebhookEvent{
		WorkspaceID:    &wsID,
		ConversationID: &otherConvID,
		EventType:      "inbound",
		RawPayload:     `{"test":3}`,
	})

	events, err := repo.ListByConversation(ctx, wsID, convID)
	if err != nil {
		t.Fatalf("list by conversation: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for conv-1, got %d", len(events))
	}
}
