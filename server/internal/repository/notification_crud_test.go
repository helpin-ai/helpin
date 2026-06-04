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

func setupNotificationCRUDTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification_crud_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE notifications (
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
			event_count INTEGER DEFAULT 1,
			last_event_at DATETIME,
			status TEXT DEFAULT 'unread',
			snoozed_until DATETIME,
			read_at DATETIME,
			archived_at DATETIME,
			priority TEXT DEFAULT 'normal',
			created_at DATETIME,
			updated_at DATETIME,
			UNIQUE(recipient_id, entity_type, entity_id, workspace_id)
		)`,
		`CREATE TABLE notification_events (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			notification_id TEXT NOT NULL,
			actor_id TEXT,
			event_type TEXT NOT NULL,
			title TEXT NOT NULL,
			metadata TEXT,
			category TEXT NOT NULL,
			actor_snapshot TEXT,
			priority TEXT NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE notification_deliveries (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			notification_event_id TEXT NOT NULL,
			channel TEXT NOT NULL,
			status TEXT NOT NULL,
			delivered_at DATETIME,
			error TEXT,
			external_message_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	return db
}

func mustExecCRUD(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// ---------------------------------------------------------------------------
// Upsert
// ---------------------------------------------------------------------------

func TestNotificationRepository_Upsert_CreateNew(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	notif := &model.Notification{
		ID: "notif-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "story-1", EventType: "story.assigned",
		Title: "Assigned", LatestEventCategory: "assignments",
		EventCount: 1, LastEventAt: now, Status: "unread", Priority: "normal",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Upsert(ctx, notif); err != nil {
		t.Fatalf("Upsert create: %v", err)
	}

	fetched, err := repo.GetByID(ctx, "notif-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Title != "Assigned" {
		t.Fatalf("title = %q, want Assigned", fetched.Title)
	}
}

func TestNotificationRepository_Upsert_UpdateExisting(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	notif := &model.Notification{
		ID: "notif-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "story-1", EventType: "story.assigned",
		Title: "Original", LatestEventCategory: "assignments",
		EventCount: 1, LastEventAt: now, Status: "unread", Priority: "normal",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Upsert(ctx, notif); err != nil {
		t.Fatalf("Upsert create: %v", err)
	}

	// Update via upsert (same composite key)
	notif.Title = "Updated"
	notif.EventCount = 2
	notif.Status = "read"
	if err := repo.Upsert(ctx, notif); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}

	fetched, err := repo.GetByID(ctx, "notif-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Title != "Updated" {
		t.Fatalf("title = %q, want Updated", fetched.Title)
	}
	if fetched.EventCount != 2 {
		t.Fatalf("event_count = %d, want 2", fetched.EventCount)
	}
}

// ---------------------------------------------------------------------------
// GetByID / GetExisting
// ---------------------------------------------------------------------------

func TestNotificationRepository_GetByID_NotFound(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent notification")
	}
}

func TestNotificationRepository_GetExisting_Found(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "notif-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "story-1", EventType: "story.assigned",
		Title: "Test", LatestEventCategory: "assignments",
		EventCount: 1, LastEventAt: now, Status: "unread", Priority: "normal",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	existing, err := repo.GetExisting(ctx, "user-1", "story", "story-1", "ws-1")
	if err != nil {
		t.Fatalf("GetExisting: %v", err)
	}
	if existing == nil {
		t.Fatal("expected non-nil existing notification")
	}
	if existing.ID != "notif-1" {
		t.Fatalf("id = %q, want notif-1", existing.ID)
	}
}

func TestNotificationRepository_GetExisting_NotFound(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	existing, err := repo.GetExisting(ctx, "user-1", "story", "story-999", "ws-1")
	if err != nil {
		t.Fatalf("GetExisting: %v", err)
	}
	if existing != nil {
		t.Fatal("expected nil for nonexistent notification")
	}
}

func TestNotificationRepository_MarkEntityEventTypeAsReadForWorkspace(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUD(t, db, `INSERT INTO notifications (
		id, workspace_id, recipient_id, entity_type, entity_id, event_type, title,
		latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-1", "ws-1", "user-1", "agent_run", "run-1", "task.agent_attention_required",
		"Agent waiting for input", model.NotifCategoryAgentAttention, 1, now, "unread", "high", now, now,
	)
	mustExecCRUD(t, db, `INSERT INTO notifications (
		id, workspace_id, recipient_id, entity_type, entity_id, event_type, title,
		latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-2", "ws-1", "user-2", "agent_run", "run-1", "task.agent_attention_required",
		"Agent waiting for input", model.NotifCategoryAgentAttention, 1, now, "unread", "high", now, now,
	)
	mustExecCRUD(t, db, `INSERT INTO notifications (
		id, workspace_id, recipient_id, entity_type, entity_id, event_type, title,
		latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-3", "ws-1", "user-3", "agent_run", "run-1", "comment.created",
		"Comment added", model.NotifCategoryComments, 1, now, "unread", "normal", now, now,
	)

	if err := repo.MarkEntityEventTypeAsReadForWorkspace(ctx, "ws-1", "agent_run", "run-1", "task.agent_attention_required"); err != nil {
		t.Fatalf("MarkEntityEventTypeAsReadForWorkspace: %v", err)
	}

	fetched1, err := repo.GetByID(ctx, "notif-1")
	if err != nil {
		t.Fatalf("GetByID notif-1: %v", err)
	}
	if fetched1.Status != "read" || fetched1.ReadAt == nil {
		t.Fatalf("notif-1 = %+v, want read with timestamp", fetched1)
	}

	fetched2, err := repo.GetByID(ctx, "notif-2")
	if err != nil {
		t.Fatalf("GetByID notif-2: %v", err)
	}
	if fetched2.Status != "read" || fetched2.ReadAt == nil {
		t.Fatalf("notif-2 = %+v, want read with timestamp", fetched2)
	}

	fetched3, err := repo.GetByID(ctx, "notif-3")
	if err != nil {
		t.Fatalf("GetByID notif-3: %v", err)
	}
	if fetched3.Status != "unread" {
		t.Fatalf("notif-3 status = %q, want unread", fetched3.Status)
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestNotificationRepository_List_DefaultExcludesArchived(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	notifs := []model.Notification{
		{ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
		{ID: "n-2", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "story.assigned", Title: "T2", LatestEventCategory: "assignments", Status: "read", LastEventAt: now.Add(-time.Minute), Priority: "normal"},
		{ID: "n-3", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-3", EventType: "story.assigned", Title: "T3", LatestEventCategory: "assignments", Status: "archived", LastEventAt: now.Add(-2 * time.Minute), Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	results, err := repo.List(ctx, "user-1", "ws-1", "", "", 20, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("count = %d, want 2 (excluding archived)", len(results))
	}
}

func TestNotificationRepository_List_StatusFilter(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	notifs := []model.Notification{
		{ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
		{ID: "n-2", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "story.assigned", Title: "T2", LatestEventCategory: "assignments", Status: "archived", LastEventAt: now, Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	results, err := repo.List(ctx, "user-1", "ws-1", "archived", "", 20, nil)
	if err != nil {
		t.Fatalf("List archived: %v", err)
	}
	if len(results) != 1 || results[0].ID != "n-2" {
		t.Fatalf("archived list = %v, want [n-2]", results)
	}
}

func TestNotificationRepository_List_AssignedFilter(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	notifs := []model.Notification{
		{ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
		{ID: "n-2", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "comment.created", Title: "T2", LatestEventCategory: "comments", Status: "unread", LastEventAt: now, Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	results, err := repo.List(ctx, "user-1", "ws-1", "", "assigned", 20, nil)
	if err != nil {
		t.Fatalf("List assigned: %v", err)
	}
	if len(results) != 1 || results[0].EventType != "story.assigned" {
		t.Fatalf("assigned list = %+v, want story.assigned only", results)
	}
}

func TestNotificationRepository_List_CursorPagination(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 5; i++ {
		if err := db.WithContext(ctx).Create(&model.Notification{
			ID: fmt.Sprintf("n-%d", i), WorkspaceID: "ws-1", RecipientID: "user-1",
			EntityType: "story", EntityID: fmt.Sprintf("s-%d", i), EventType: "story.assigned",
			Title: fmt.Sprintf("T%d", i), LatestEventCategory: "assignments",
			Status: "unread", LastEventAt: now.Add(time.Duration(-i) * time.Minute), Priority: "normal",
		}).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	// First page
	page1, err := repo.List(ctx, "user-1", "ws-1", "", "", 2, nil)
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1 count = %d, want 2", len(page1))
	}

	// Second page using cursor
	cursor := page1[1].LastEventAt
	page2, err := repo.List(ctx, "user-1", "ws-1", "", "", 2, &cursor)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page 2 count = %d, want 2", len(page2))
	}
	if page2[0].ID == page1[0].ID || page2[0].ID == page1[1].ID {
		t.Fatal("page 2 should not overlap with page 1")
	}
}

func TestNotificationRepository_List_HidesSnoozedNotifications(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()
	future := now.Add(1 * time.Hour)

	notifs := []model.Notification{
		{ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
		{ID: "n-2", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "story.assigned", Title: "T2", LatestEventCategory: "assignments", Status: "unread", SnoozedUntil: &future, LastEventAt: now, Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	results, err := repo.List(ctx, "user-1", "ws-1", "", "", 20, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("count = %d, want 1 (snoozed hidden)", len(results))
	}
	if results[0].ID != "n-1" {
		t.Fatalf("expected n-1, got %s", results[0].ID)
	}
}

// ---------------------------------------------------------------------------
// MarkAsRead / MarkAllAsRead / ArchiveAllRead
// ---------------------------------------------------------------------------

func TestNotificationRepository_MarkAsRead(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "s-1", EventType: "story.assigned",
		Title: "T1", LatestEventCategory: "assignments",
		Status: "unread", LastEventAt: now, Priority: "normal",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := repo.MarkAsRead(ctx, "n-1", "user-1"); err != nil {
		t.Fatalf("MarkAsRead: %v", err)
	}

	fetched, err := repo.GetByID(ctx, "n-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Status != "read" {
		t.Fatalf("status = %q, want read", fetched.Status)
	}
	if fetched.ReadAt == nil {
		t.Fatal("expected read_at to be set")
	}
}

func TestNotificationRepository_MarkAllAsRead(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	notifs := []model.Notification{
		{ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
		{ID: "n-2", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "story.assigned", Title: "T2", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
		{ID: "n-3", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-3", EventType: "story.assigned", Title: "T3", LatestEventCategory: "assignments", Status: "read", LastEventAt: now, Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	if err := repo.MarkAllAsRead(ctx, "user-1", "ws-1"); err != nil {
		t.Fatalf("MarkAllAsRead: %v", err)
	}

	var rows []struct{ ID, Status string }
	if err := db.Raw(`SELECT id, status FROM notifications ORDER BY id`).Scan(&rows).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, row := range rows {
		if row.Status != "read" {
			t.Fatalf("notification %s status = %q, want read", row.ID, row.Status)
		}
	}
}

func TestNotificationRepository_ArchiveAllRead(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	notifs := []model.Notification{
		{ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "read", LastEventAt: now, Priority: "normal"},
		{ID: "n-2", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "story.assigned", Title: "T2", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	if err := repo.ArchiveAllRead(ctx, "user-1", "ws-1"); err != nil {
		t.Fatalf("ArchiveAllRead: %v", err)
	}

	n1, _ := repo.GetByID(ctx, "n-1")
	if n1.Status != "archived" {
		t.Fatalf("n-1 status = %q, want archived", n1.Status)
	}
	n2, _ := repo.GetByID(ctx, "n-2")
	if n2.Status != "unread" {
		t.Fatalf("n-2 status = %q, want unread (unchanged)", n2.Status)
	}
}

// ---------------------------------------------------------------------------
// Update / Delete
// ---------------------------------------------------------------------------

func TestNotificationRepository_Update(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "s-1", EventType: "story.assigned",
		Title: "T1", LatestEventCategory: "assignments",
		Status: "unread", LastEventAt: now, Priority: "normal",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	snoozeTime := now.Add(2 * time.Hour)
	if err := repo.Update(ctx, "n-1", "user-1", map[string]interface{}{
		"status":        "snoozed",
		"snoozed_until": snoozeTime,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	fetched, err := repo.GetByID(ctx, "n-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Status != "snoozed" {
		t.Fatalf("status = %q, want snoozed", fetched.Status)
	}
}

func TestNotificationRepository_Update_WrongRecipient(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "s-1", EventType: "story.assigned",
		Title: "T1", LatestEventCategory: "assignments",
		Status: "unread", LastEventAt: now, Priority: "normal",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Update with wrong recipient should not change anything
	if err := repo.Update(ctx, "n-1", "user-wrong", map[string]interface{}{
		"status": "read",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	fetched, _ := repo.GetByID(ctx, "n-1")
	if fetched.Status != "unread" {
		t.Fatalf("status = %q, should remain unread with wrong recipient", fetched.Status)
	}
}

func TestNotificationRepository_Delete(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "s-1", EventType: "story.assigned",
		Title: "T1", LatestEventCategory: "assignments",
		Status: "unread", LastEventAt: now, Priority: "normal",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := repo.Delete(ctx, "n-1", "user-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var count int64
	db.Table("notifications").Count(&count)
	if count != 0 {
		t.Fatalf("count = %d, want 0 after delete", count)
	}
}

func TestNotificationRepository_Delete_WrongRecipient(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "s-1", EventType: "story.assigned",
		Title: "T1", LatestEventCategory: "assignments",
		Status: "unread", LastEventAt: now, Priority: "normal",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := repo.Delete(ctx, "n-1", "user-wrong"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var count int64
	db.Table("notifications").Count(&count)
	if count != 1 {
		t.Fatalf("count = %d, want 1 (wrong recipient should not delete)", count)
	}
}

// ---------------------------------------------------------------------------
// CreateEvent / CreateDelivery
// ---------------------------------------------------------------------------

func TestNotificationRepository_CreateEvent(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()

	event := &model.NotificationEvent{
		ID:             "event-1",
		NotificationID: "notif-1",
		EventType:      "story.assigned",
		Title:          "Assigned to you",
		Category:       "assignments",
		Priority:       "normal",
	}
	if err := repo.CreateEvent(ctx, event); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	var count int64
	db.Table("notification_events").Count(&count)
	if count != 1 {
		t.Fatalf("event count = %d, want 1", count)
	}
}

func TestNotificationRepository_CreateDelivery(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()

	delivery := &model.NotificationDelivery{
		ID:                  "delivery-1",
		NotificationEventID: "event-1",
		Channel:             "in_app",
		Status:              "delivered",
		DeliveredAt:         &now,
	}
	if err := repo.CreateDelivery(ctx, delivery); err != nil {
		t.Fatalf("CreateDelivery: %v", err)
	}

	var count int64
	db.Table("notification_deliveries").Count(&count)
	if count != 1 {
		t.Fatalf("delivery count = %d, want 1", count)
	}
}

// ---------------------------------------------------------------------------
// WakeExpiredSnoozes
// ---------------------------------------------------------------------------

func TestNotificationRepository_WakeExpiredSnoozes(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	notifs := []model.Notification{
		{ID: "n-expired", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "snoozed", SnoozedUntil: &past, LastEventAt: now, Priority: "normal"},
		{ID: "n-active", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "story.assigned", Title: "T2", LatestEventCategory: "assignments", Status: "snoozed", SnoozedUntil: &future, LastEventAt: now, Priority: "normal"},
		{ID: "n-unread", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-3", EventType: "story.assigned", Title: "T3", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	woken, err := repo.WakeExpiredSnoozes(ctx)
	if err != nil {
		t.Fatalf("WakeExpiredSnoozes: %v", err)
	}
	if len(woken) != 1 {
		t.Fatalf("woken count = %d, want 1", len(woken))
	}
	if woken[0].ID != "n-expired" {
		t.Fatalf("woken[0].ID = %q, want n-expired", woken[0].ID)
	}

	// Verify the expired snooze is now unread
	fetched, _ := repo.GetByID(ctx, "n-expired")
	if fetched.Status != "unread" {
		t.Fatalf("n-expired status = %q, want unread", fetched.Status)
	}

	// Verify the active snooze is still snoozed
	active, _ := repo.GetByID(ctx, "n-active")
	if active.Status != "snoozed" {
		t.Fatalf("n-active status = %q, want snoozed", active.Status)
	}
}

func TestNotificationRepository_WakeExpiredSnoozes_NoneExpired(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()
	future := now.Add(1 * time.Hour)

	if err := db.WithContext(ctx).Create(&model.Notification{
		ID: "n-active", WorkspaceID: "ws-1", RecipientID: "user-1",
		EntityType: "story", EntityID: "s-1", EventType: "story.assigned",
		Title: "T1", LatestEventCategory: "assignments",
		Status: "snoozed", SnoozedUntil: &future, LastEventAt: now, Priority: "normal",
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	woken, err := repo.WakeExpiredSnoozes(ctx)
	if err != nil {
		t.Fatalf("WakeExpiredSnoozes: %v", err)
	}
	if len(woken) != 0 {
		t.Fatalf("woken count = %d, want 0", len(woken))
	}
}

// ---------------------------------------------------------------------------
// UnreadCount
// ---------------------------------------------------------------------------

func TestNotificationRepository_UnreadCount_ExcludesSnoozed(t *testing.T) {
	db := setupNotificationCRUDTestDB(t)
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	now := time.Now()
	future := now.Add(1 * time.Hour)

	notifs := []model.Notification{
		{ID: "n-1", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-1", EventType: "story.assigned", Title: "T1", LatestEventCategory: "assignments", Status: "unread", LastEventAt: now, Priority: "normal"},
		{ID: "n-2", WorkspaceID: "ws-1", RecipientID: "user-1", EntityType: "story", EntityID: "s-2", EventType: "story.assigned", Title: "T2", LatestEventCategory: "assignments", Status: "unread", SnoozedUntil: &future, LastEventAt: now, Priority: "normal"},
	}
	for i := range notifs {
		if err := db.WithContext(ctx).Create(&notifs[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	count, err := repo.UnreadCount(ctx, "user-1", "ws-1", "all")
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (snoozed excluded)", count)
	}
}
