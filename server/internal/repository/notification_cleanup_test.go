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

func setupCleanupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification_cleanup_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE notifications (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			recipient_id TEXT NOT NULL,
			actor_id TEXT,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT,
			metadata TEXT,
			latest_event_category TEXT NOT NULL,
			actor_snapshot TEXT,
			entity_snapshot TEXT,
			parent_entity_snapshot TEXT,
			event_count INTEGER DEFAULT 1,
			last_event_at DATETIME,
			status TEXT DEFAULT 'unread',
			snoozed_until DATETIME,
			read_at DATETIME,
			archived_at DATETIME,
			priority TEXT DEFAULT 'normal',
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create notifications table: %v", err)
	}
	return db
}

func TestDeleteArchivedOlderThan_DeletesOldArchived(t *testing.T) {
	db := setupCleanupTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	now := time.Now()
	oldDate := now.AddDate(0, 0, -100)
	recentDate := now.AddDate(0, 0, -30)
	cutoff := now.AddDate(0, 0, -90)

	notifs := []model.Notification{
		{
			ID: "old-archived", WorkspaceID: "ws-1", RecipientID: "user-1",
			EntityType: "story", EntityID: "story-1", EventType: "story.assigned",
			Title: "old", LatestEventCategory: "assignments", Status: "archived",
			Priority: "normal", LastEventAt: oldDate, CreatedAt: oldDate, UpdatedAt: oldDate,
		},
		{
			ID: "recent-archived", WorkspaceID: "ws-1", RecipientID: "user-1",
			EntityType: "story", EntityID: "story-2", EventType: "story.assigned",
			Title: "recent", LatestEventCategory: "assignments", Status: "archived",
			Priority: "normal", LastEventAt: recentDate, CreatedAt: recentDate, UpdatedAt: recentDate,
		},
		{
			ID: "old-unread", WorkspaceID: "ws-1", RecipientID: "user-1",
			EntityType: "story", EntityID: "story-3", EventType: "story.assigned",
			Title: "old unread", LatestEventCategory: "assignments", Status: "unread",
			Priority: "normal", LastEventAt: oldDate, CreatedAt: oldDate, UpdatedAt: oldDate,
		},
		{
			ID: "old-read", WorkspaceID: "ws-1", RecipientID: "user-1",
			EntityType: "story", EntityID: "story-4", EventType: "story.assigned",
			Title: "old read", LatestEventCategory: "assignments", Status: "read",
			Priority: "normal", LastEventAt: oldDate, CreatedAt: oldDate, UpdatedAt: oldDate,
		},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed notification %d: %v", i, err)
		}
	}

	count, err := repo.DeleteArchivedOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteArchivedOlderThan: %v", err)
	}
	if count != 1 {
		t.Fatalf("deleted count = %d, want 1", count)
	}

	var remaining []model.Notification
	if err := db.WithContext(ctx).Find(&remaining).Error; err != nil {
		t.Fatalf("load remaining: %v", err)
	}
	if len(remaining) != 3 {
		t.Fatalf("remaining count = %d, want 3", len(remaining))
	}
	for _, n := range remaining {
		if n.ID == "old-archived" {
			t.Fatal("old-archived should have been deleted")
		}
	}
}

func TestDeleteArchivedOlderThan_NoMatchReturnsZero(t *testing.T) {
	db := setupCleanupTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	now := time.Now()
	recentDate := now.AddDate(0, 0, -10)

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "recent-archived", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "story-1", EventType: "story.assigned",
		Title: "recent", LatestEventCategory: "assignments", Status: "archived",
		Priority: "normal", LastEventAt: recentDate, CreatedAt: recentDate, UpdatedAt: recentDate,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	cutoff := now.AddDate(0, 0, -90)
	count, err := repo.DeleteArchivedOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteArchivedOlderThan: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted count = %d, want 0", count)
	}
}

func TestDeleteArchivedOlderThan_EmptyTableReturnsZero(t *testing.T) {
	db := setupCleanupTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	cutoff := time.Now().AddDate(0, 0, -90)
	count, err := repo.DeleteArchivedOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteArchivedOlderThan: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted count = %d, want 0", count)
	}
}
