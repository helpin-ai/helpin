package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

func newCleanupServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification-cleanup-svc-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	mustExecCleanup(t, db, `CREATE TABLE notifications (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
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
		event_count INTEGER NOT NULL DEFAULT 1,
		last_event_at DATETIME,
		status TEXT NOT NULL DEFAULT 'unread',
		snoozed_until DATETIME,
		read_at DATETIME,
		archived_at DATETIME,
		priority TEXT NOT NULL DEFAULT 'normal',
		created_at DATETIME,
		updated_at DATETIME,
		UNIQUE(recipient_id, entity_type, entity_id, workspace_id)
	)`)

	return db
}

func mustExecCleanup(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func seedCleanupNotification(t *testing.T, db *gorm.DB, id, status string, updatedAt time.Time) {
	t.Helper()
	mustExecCleanup(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, "ws-1", "user-1", "story", "entity-"+id, "story.assigned", "Title "+id, "assignments", 1, updatedAt, status, "normal", updatedAt, updatedAt)
}

func TestCleanupArchivedNotifications_DeletesOldArchived(t *testing.T) {
	db := newCleanupServiceTestDB(t)
	ctx := context.Background()

	now := time.Now()
	oldDate := now.AddDate(0, 0, -100)
	recentDate := now.AddDate(0, 0, -30)

	seedCleanupNotification(t, db, "old-archived", "archived", oldDate)
	seedCleanupNotification(t, db, "recent-archived", "archived", recentDate)
	seedCleanupNotification(t, db, "old-unread", "unread", oldDate)
	seedCleanupNotification(t, db, "old-read", "read", oldDate)

	svc := NewNotificationService(
		repository.NewNotificationRepository(db),
		nil, nil, nil, nil, nil, nil, nil,
	)

	count, err := svc.CleanupArchivedNotifications(ctx, 90)
	if err != nil {
		t.Fatalf("CleanupArchivedNotifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("deleted = %d, want 1", count)
	}

	var remaining int64
	if err := db.Table("notifications").Count(&remaining).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 3 {
		t.Fatalf("remaining = %d, want 3", remaining)
	}
}

func TestCleanupArchivedNotifications_DefaultRetention(t *testing.T) {
	db := newCleanupServiceTestDB(t)
	ctx := context.Background()

	now := time.Now()
	oldDate := now.AddDate(0, 0, -100)

	seedCleanupNotification(t, db, "old-archived", "archived", oldDate)

	svc := NewNotificationService(
		repository.NewNotificationRepository(db),
		nil, nil, nil, nil, nil, nil, nil,
	)

	// 0 should default to 90
	count, err := svc.CleanupArchivedNotifications(ctx, 0)
	if err != nil {
		t.Fatalf("CleanupArchivedNotifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("deleted = %d, want 1", count)
	}
}

func TestCleanupArchivedNotifications_NothingToDelete(t *testing.T) {
	db := newCleanupServiceTestDB(t)
	ctx := context.Background()

	now := time.Now()
	recentDate := now.AddDate(0, 0, -10)

	seedCleanupNotification(t, db, "recent-archived", "archived", recentDate)
	seedCleanupNotification(t, db, "recent-unread", "unread", recentDate)

	svc := NewNotificationService(
		repository.NewNotificationRepository(db),
		nil, nil, nil, nil, nil, nil, nil,
	)

	count, err := svc.CleanupArchivedNotifications(ctx, 90)
	if err != nil {
		t.Fatalf("CleanupArchivedNotifications: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted = %d, want 0", count)
	}
}

func TestCleanupArchivedNotifications_CustomRetentionDays(t *testing.T) {
	db := newCleanupServiceTestDB(t)
	ctx := context.Background()

	now := time.Now()
	date40DaysAgo := now.AddDate(0, 0, -40)
	date20DaysAgo := now.AddDate(0, 0, -20)

	seedCleanupNotification(t, db, "archived-40d", "archived", date40DaysAgo)
	seedCleanupNotification(t, db, "archived-20d", "archived", date20DaysAgo)

	svc := NewNotificationService(
		repository.NewNotificationRepository(db),
		nil, nil, nil, nil, nil, nil, nil,
	)

	// 30-day retention: should delete the 40-day-old one
	count, err := svc.CleanupArchivedNotifications(ctx, 30)
	if err != nil {
		t.Fatalf("CleanupArchivedNotifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("deleted = %d, want 1", count)
	}

	var remaining int64
	if err := db.Table("notifications").Count(&remaining).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining = %d, want 1", remaining)
	}
}

func TestCleanupArchivedNotifications_EmptyTable(t *testing.T) {
	db := newCleanupServiceTestDB(t)
	ctx := context.Background()

	svc := NewNotificationService(
		repository.NewNotificationRepository(db),
		nil, nil, nil, nil, nil, nil, nil,
	)

	count, err := svc.CleanupArchivedNotifications(ctx, 90)
	if err != nil {
		t.Fatalf("CleanupArchivedNotifications: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted = %d, want 0", count)
	}
}
