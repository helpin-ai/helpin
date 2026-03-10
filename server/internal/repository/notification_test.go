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

func setupNotificationRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification_repo_%d?mode=memory&cache=shared", time.Now().UnixNano())
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

func TestNotificationRepositoryUnreadCount_RespectsBadgeMode(t *testing.T) {
	db := setupNotificationRepoTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	notifs := []model.Notification{
		{
			ID:                  "notif-1",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "story",
			EntityID:            "story-1",
			EventType:           "story.mention",
			Title:               "mentioned you",
			LatestEventCategory: "mention",
			Status:              "unread",
			LastEventAt:         time.Now(),
		},
		{
			ID:                  "notif-2",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "story",
			EntityID:            "story-2",
			EventType:           "story.assigned",
			Title:               "assigned you",
			LatestEventCategory: "assignment",
			Status:              "unread",
			LastEventAt:         time.Now(),
		},
		{
			ID:                  "notif-3",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "story",
			EntityID:            "story-3",
			EventType:           "comment.mention",
			Title:               "old mention",
			LatestEventCategory: "mention",
			Status:              "read",
			LastEventAt:         time.Now(),
		},
		{
			ID:                  "notif-4",
			WorkspaceID:         "ws-2",
			RecipientID:         "user-1",
			EntityType:          "story",
			EntityID:            "story-4",
			EventType:           "story.mention",
			Title:               "other workspace mention",
			LatestEventCategory: "mention",
			Status:              "unread",
			LastEventAt:         time.Now(),
		},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed notification %d: %v", i, err)
		}
	}

	allCount, err := repo.UnreadCount(ctx, "user-1", "ws-1", "all")
	if err != nil {
		t.Fatalf("unread count all: %v", err)
	}
	if allCount != 2 {
		t.Fatalf("expected unread count 2 for all, got %d", allCount)
	}

	mentionsOnlyCount, err := repo.UnreadCount(ctx, "user-1", "ws-1", "mentions_only")
	if err != nil {
		t.Fatalf("unread count mentions_only: %v", err)
	}
	if mentionsOnlyCount != 1 {
		t.Fatalf("expected unread count 1 for mentions_only, got %d", mentionsOnlyCount)
	}

	noneCount, err := repo.UnreadCount(ctx, "user-1", "ws-1", "none")
	if err != nil {
		t.Fatalf("unread count none: %v", err)
	}
	if noneCount != 0 {
		t.Fatalf("expected unread count 0 for none, got %d", noneCount)
	}
}

func TestNotificationRepositoryList_MentionsFilterMatchesMentionEvents(t *testing.T) {
	db := setupNotificationRepoTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	notifs := []model.Notification{
		{
			ID:                  "notif-1",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "story",
			EntityID:            "story-1",
			EventType:           "story.mention",
			Title:               "mentioned you in story",
			LatestEventCategory: "mention",
			Status:              "unread",
			LastEventAt:         time.Now(),
		},
		{
			ID:                  "notif-2",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "story",
			EntityID:            "story-2",
			EventType:           "comment.mention",
			Title:               "mentioned you in comment",
			LatestEventCategory: "mention",
			Status:              "unread",
			LastEventAt:         time.Now().Add(-time.Minute),
		},
		{
			ID:                  "notif-3",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "story",
			EntityID:            "story-3",
			EventType:           "story.assigned",
			Title:               "assigned you",
			LatestEventCategory: "assignment",
			Status:              "unread",
			LastEventAt:         time.Now().Add(-2 * time.Minute),
		},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed notification %d: %v", i, err)
		}
	}

	results, err := repo.List(ctx, "user-1", "ws-1", "", "mentions", 20, nil)
	if err != nil {
		t.Fatalf("list mentions: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 mention notifications, got %d", len(results))
	}
	for _, notif := range results {
		if notif.EventType != "story.mention" && notif.EventType != "comment.mention" {
			t.Fatalf("unexpected event type in mentions filter: %s", notif.EventType)
		}
	}
}
