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
	if err := db.Exec(`
		CREATE TABLE notification_events (
			id TEXT PRIMARY KEY,
			notification_id TEXT NOT NULL,
			actor_id TEXT,
			event_type TEXT NOT NULL,
			title TEXT NOT NULL,
			metadata TEXT,
			category TEXT NOT NULL,
			actor_snapshot TEXT,
			priority TEXT NOT NULL,
			created_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create notification_events table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE notification_deliveries (
			id TEXT PRIMARY KEY,
			notification_event_id TEXT NOT NULL,
			channel TEXT NOT NULL,
			status TEXT NOT NULL,
			delivered_at DATETIME,
			error TEXT,
			external_message_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create notification_deliveries table: %v", err)
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
			EntityType:          "task",
			EntityID:            "task-1",
			EventType:           "task.mention",
			Title:               "mentioned you",
			LatestEventCategory: "mention",
			Status:              "unread",
			LastEventAt:         time.Now(),
		},
		{
			ID:                  "notif-2",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "task",
			EntityID:            "task-2",
			EventType:           "task.assigned",
			Title:               "assigned you",
			LatestEventCategory: "assignment",
			Status:              "unread",
			LastEventAt:         time.Now(),
		},
		{
			ID:                  "notif-3",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "task",
			EntityID:            "task-3",
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
			EntityType:          "task",
			EntityID:            "task-4",
			EventType:           "task.mention",
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
			EntityType:          "task",
			EntityID:            "task-1",
			EventType:           "task.mention",
			Title:               "mentioned you in task",
			LatestEventCategory: "mention",
			Status:              "unread",
			LastEventAt:         time.Now(),
		},
		{
			ID:                  "notif-2",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "task",
			EntityID:            "task-2",
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
			EntityType:          "task",
			EntityID:            "task-3",
			EventType:           "task.assigned",
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
		if notif.EventType != "task.mention" && notif.EventType != "comment.mention" {
			t.Fatalf("unexpected event type in mentions filter: %s", notif.EventType)
		}
	}
}

func TestNotificationRepositoryListPendingDigestDeliveries(t *testing.T) {
	db := setupNotificationRepoTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()
	later := now.Add(time.Minute)

	notifs := []model.Notification{
		{
			ID:                  "notif-1",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "task",
			EntityID:            "task-1",
			EventType:           "comment.created",
			Title:               "Task comment",
			LatestEventCategory: model.NotifCategoryComments,
			Status:              "unread",
			LastEventAt:         now,
			Priority:            "normal",
		},
		{
			ID:                  "notif-2",
			WorkspaceID:         "ws-1",
			RecipientID:         "user-1",
			EntityType:          "task",
			EntityID:            "task-2",
			EventType:           "comment.created",
			Title:               "Task comment 2",
			LatestEventCategory: model.NotifCategoryComments,
			Status:              "read",
			LastEventAt:         later,
			Priority:            "normal",
		},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed notification %d: %v", i, err)
		}
	}

	events := []model.NotificationEvent{
		{ID: "event-1", NotificationID: "notif-1", EventType: "comment.created", Title: "Task comment", Category: model.NotifCategoryComments, Priority: "normal", CreatedAt: now},
		{ID: "event-2", NotificationID: "notif-2", EventType: "comment.created", Title: "Task comment 2", Category: model.NotifCategoryComments, Priority: "normal", CreatedAt: later},
	}
	for i := range events {
		if err := db.WithContext(ctx).Create(&events[i]).Error; err != nil {
			t.Fatalf("seed event %d: %v", i, err)
		}
	}

	deliveries := []model.NotificationDelivery{
		{ID: "delivery-1", NotificationEventID: "event-1", Channel: "digest", Status: "pending", CreatedAt: now, UpdatedAt: now},
		{ID: "delivery-2", NotificationEventID: "event-2", Channel: "digest", Status: "pending", CreatedAt: later, UpdatedAt: later},
		{ID: "delivery-3", NotificationEventID: "event-2", Channel: "email", Status: "pending", CreatedAt: later, UpdatedAt: later},
	}
	for i := range deliveries {
		if err := db.WithContext(ctx).Create(&deliveries[i]).Error; err != nil {
			t.Fatalf("seed delivery %d: %v", i, err)
		}
	}

	results, err := repo.ListPendingDigestDeliveries(ctx)
	if err != nil {
		t.Fatalf("ListPendingDigestDeliveries: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("pending digest delivery count = %d, want 2", len(results))
	}
	if results[0].DeliveryID != "delivery-1" || results[0].EventType != "comment.created" || results[0].NotificationStatus != "unread" {
		t.Fatalf("first pending digest row = %+v, want delivery-1 unread comment.created", results[0])
	}
	if results[1].DeliveryID != "delivery-2" || results[1].NotificationStatus != "read" {
		t.Fatalf("second pending digest row = %+v, want delivery-2 read", results[1])
	}
}

func TestNotificationRepositoryUpdateDeliveryStatus(t *testing.T) {
	db := setupNotificationRepoTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	if err := db.WithContext(ctx).Create(&model.NotificationDelivery{
		ID:                  "delivery-1",
		NotificationEventID: "event-1",
		Channel:             "digest",
		Status:              "pending",
		CreatedAt:           now,
		UpdatedAt:           now,
	}).Error; err != nil {
		t.Fatalf("seed delivery 1: %v", err)
	}
	if err := db.WithContext(ctx).Create(&model.NotificationDelivery{
		ID:                  "delivery-2",
		NotificationEventID: "event-2",
		Channel:             "digest",
		Status:              "pending",
		CreatedAt:           now,
		UpdatedAt:           now,
	}).Error; err != nil {
		t.Fatalf("seed delivery 2: %v", err)
	}

	deliveredAt := now.Add(5 * time.Minute)
	errorMessage := "sent in digest"
	if err := repo.UpdateDeliveryStatus(ctx, []string{"delivery-1", "delivery-2"}, "delivered", &deliveredAt, &errorMessage); err != nil {
		t.Fatalf("UpdateDeliveryStatus: %v", err)
	}

	var rows []struct {
		ID          string
		Status      string
		DeliveredAt *time.Time
		Error       *string
	}
	if err := db.Raw(`SELECT id, status, delivered_at, error FROM notification_deliveries ORDER BY id`).Scan(&rows).Error; err != nil {
		t.Fatalf("load updated deliveries: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("updated delivery count = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.Status != "delivered" {
			t.Fatalf("delivery %s status = %q, want delivered", row.ID, row.Status)
		}
		if row.DeliveredAt == nil || !row.DeliveredAt.Equal(deliveredAt) {
			t.Fatalf("delivery %s delivered_at = %v, want %v", row.ID, row.DeliveredAt, deliveredAt)
		}
		if row.Error == nil || *row.Error != errorMessage {
			t.Fatalf("delivery %s error = %v, want %q", row.ID, row.Error, errorMessage)
		}
	}
}
