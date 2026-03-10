package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func newNotificationCRUDTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification-crud-svc-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			avatar_url TEXT,
			default_workspace_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			description TEXT,
			logo_url TEXT,
			timezone TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE user_notification_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL UNIQUE,
			email_enabled BOOLEAN NOT NULL DEFAULT 1,
			email_digest_frequency TEXT NOT NULL DEFAULT 'daily',
			email_digest_time TEXT NOT NULL DEFAULT '09:00',
			email_digest_day INTEGER NOT NULL DEFAULT 1,
			do_not_disturb BOOLEAN NOT NULL DEFAULT 0,
			dnd_until DATETIME,
			badge_mode TEXT NOT NULL DEFAULT 'all',
			timezone TEXT NOT NULL DEFAULT 'UTC',
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE notification_preferences (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			mute_workspace BOOLEAN NOT NULL DEFAULT 0,
			do_not_disturb BOOLEAN NOT NULL DEFAULT 0,
			dnd_until DATETIME,
			email_enabled BOOLEAN NOT NULL DEFAULT 1,
			email_digest_frequency TEXT,
			email_digest_time TEXT,
			email_digest_day INTEGER,
			timezone TEXT,
			channel_preferences TEXT,
			badge_mode TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE entity_followers (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			reason TEXT,
			created_at DATETIME,
			UNIQUE(user_id, entity_type, entity_id)
		)`,
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
			t.Fatalf("exec: %v", err)
		}
	}
	return db
}

func mustExecCRUDSvc(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func newCRUDNotificationService(db *gorm.DB) *NotificationService {
	return NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil, nil, "",
	)
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestNotificationService_List_ReturnsPaginated(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)

	for i := 0; i < 5; i++ {
		mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("n-%d", i), "ws-1", "user-1", "story", fmt.Sprintf("s-%d", i), "story.assigned", fmt.Sprintf("T%d", i), "assignments", 1, now.Add(time.Duration(-i)*time.Minute), "unread", "normal", now, now)
	}

	svc := newCRUDNotificationService(db)
	result, err := svc.List(ctx, "user-1", "ws-1", "", "", 3, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Data) != 3 {
		t.Fatalf("data count = %d, want 3", len(result.Data))
	}
	if result.NextCursor == nil {
		t.Fatal("expected next_cursor for paginated result")
	}
	if result.UnreadCount != 5 {
		t.Fatalf("unread_count = %d, want 5", result.UnreadCount)
	}
}

func TestNotificationService_List_NoCursorWhenLastPage(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-1", "ws-1", "user-1", "story", "s-1", "story.assigned", "T1", "assignments", 1, now, "unread", "normal", now, now)

	svc := newCRUDNotificationService(db)
	result, err := svc.List(ctx, "user-1", "ws-1", "", "", 20, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.NextCursor != nil {
		t.Fatal("expected nil next_cursor on last page")
	}
}

// ---------------------------------------------------------------------------
// UnreadCount
// ---------------------------------------------------------------------------

func TestNotificationService_UnreadCount_RespectsBadgeMode(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "none", "UTC", now, now)
	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-1", "ws-1", "user-1", "story", "s-1", "story.assigned", "T1", "assignments", 1, now, "unread", "normal", now, now)

	svc := newCRUDNotificationService(db)
	count, err := svc.UnreadCount(ctx, "user-1", "ws-1")
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 (badge_mode=none)", count)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestNotificationService_Update_MarkRead(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-1", "ws-1", "user-1", "story", "s-1", "story.assigned", "T1", "assignments", 1, now, "unread", "normal", now, now)

	svc := newCRUDNotificationService(db)
	status := "read"
	if err := svc.Update(ctx, "n-1", "user-1", model.UpdateNotificationRequest{Status: &status}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var row struct{ Status string }
	db.Raw(`SELECT status FROM notifications WHERE id = ?`, "n-1").Scan(&row)
	if row.Status != "read" {
		t.Fatalf("status = %q, want read", row.Status)
	}
}

func TestNotificationService_Update_Snooze(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-1", "ws-1", "user-1", "story", "s-1", "story.assigned", "T1", "assignments", 1, now, "unread", "normal", now, now)

	svc := newCRUDNotificationService(db)
	snoozeTime := now.Add(2 * time.Hour)
	if err := svc.Update(ctx, "n-1", "user-1", model.UpdateNotificationRequest{SnoozedUntil: &snoozeTime}); err != nil {
		t.Fatalf("Update snooze: %v", err)
	}

	var row struct{ Status string }
	db.Raw(`SELECT status FROM notifications WHERE id = ?`, "n-1").Scan(&row)
	if row.Status != "snoozed" {
		t.Fatalf("status = %q, want snoozed", row.Status)
	}
}

func TestNotificationService_Update_Archive(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-1", "ws-1", "user-1", "story", "s-1", "story.assigned", "T1", "assignments", 1, now, "read", "normal", now, now)

	svc := newCRUDNotificationService(db)
	status := "archived"
	if err := svc.Update(ctx, "n-1", "user-1", model.UpdateNotificationRequest{Status: &status}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var row struct{ Status string }
	db.Raw(`SELECT status FROM notifications WHERE id = ?`, "n-1").Scan(&row)
	if row.Status != "archived" {
		t.Fatalf("status = %q, want archived", row.Status)
	}
}

// ---------------------------------------------------------------------------
// MarkAllAsRead / ArchiveAllRead / Delete
// ---------------------------------------------------------------------------

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 3; i++ {
		mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("n-%d", i), "ws-1", "user-1", "story", fmt.Sprintf("s-%d", i), "story.assigned", "T", "assignments", 1, now, "unread", "normal", now, now)
	}

	svc := newCRUDNotificationService(db)
	if err := svc.MarkAllAsRead(ctx, "user-1", "ws-1"); err != nil {
		t.Fatalf("MarkAllAsRead: %v", err)
	}

	var unread int64
	db.Table("notifications").Where("status = 'unread'").Count(&unread)
	if unread != 0 {
		t.Fatalf("unread = %d, want 0", unread)
	}
}

func TestNotificationService_ArchiveAllRead(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-read", "ws-1", "user-1", "story", "s-1", "story.assigned", "T", "assignments", 1, now, "read", "normal", now, now)
	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-unread", "ws-1", "user-1", "story", "s-2", "story.assigned", "T", "assignments", 1, now, "unread", "normal", now, now)

	svc := newCRUDNotificationService(db)
	if err := svc.ArchiveAllRead(ctx, "user-1", "ws-1"); err != nil {
		t.Fatalf("ArchiveAllRead: %v", err)
	}

	var archived int64
	db.Table("notifications").Where("status = 'archived'").Count(&archived)
	if archived != 1 {
		t.Fatalf("archived = %d, want 1", archived)
	}
}

func TestNotificationService_Delete(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"n-1", "ws-1", "user-1", "story", "s-1", "story.assigned", "T", "assignments", 1, now, "unread", "normal", now, now)

	svc := newCRUDNotificationService(db)
	if err := svc.Delete(ctx, "n-1", "user-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var count int64
	db.Table("notifications").Count(&count)
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// GetPreferences / UpdatePreferences
// ---------------------------------------------------------------------------

func TestNotificationService_GetPreferences_OverlaysAccountSettings(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", false, "weekly", "10:00", 3, true, "mentions_only", "America/Chicago", now, now)
	mustExecCRUDSvc(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", true, `{"comments":{"in_app":false}}`, now, now)

	svc := newCRUDNotificationService(db)
	prefs, err := svc.GetPreferences(ctx, "user-1", "ws-1")
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}

	// Workspace-level field
	if !prefs.MuteWorkspace {
		t.Fatal("expected mute_workspace=true from workspace pref")
	}
	// Account-level overlays
	if prefs.EmailEnabled {
		t.Fatal("expected email_enabled=false from account settings overlay")
	}
	if prefs.EmailDigestFrequency != "weekly" {
		t.Fatalf("digest_frequency = %q, want weekly from account overlay", prefs.EmailDigestFrequency)
	}
	if !prefs.DoNotDisturb {
		t.Fatal("expected do_not_disturb=true from account overlay")
	}
	if prefs.BadgeMode != "mentions_only" {
		t.Fatalf("badge_mode = %q, want mentions_only from account overlay", prefs.BadgeMode)
	}
	if prefs.Timezone != "America/Chicago" {
		t.Fatalf("timezone = %q, want America/Chicago from account overlay", prefs.Timezone)
	}
}

func TestNotificationService_GetPreferences_DefaultsWhenNoPrefs(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()

	svc := newCRUDNotificationService(db)
	prefs, err := svc.GetPreferences(ctx, "user-new", "ws-1")
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if prefs.MuteWorkspace {
		t.Fatal("expected default mute_workspace=false")
	}
	if !prefs.EmailEnabled {
		t.Fatal("expected default email_enabled=true")
	}
}

func TestNotificationService_UpdatePreferences_WorkspaceFields(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()

	svc := newCRUDNotificationService(db)
	mute := true
	if err := svc.UpdatePreferences(ctx, "user-1", "ws-1", model.UpdateNotificationPreferenceRequest{
		MuteWorkspace: &mute,
		ChannelPreferences: map[string]interface{}{
			"comments": map[string]interface{}{"in_app": false, "email": true},
		},
	}); err != nil {
		t.Fatalf("UpdatePreferences: %v", err)
	}

	prefs, err := svc.GetPreferences(ctx, "user-1", "ws-1")
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if !prefs.MuteWorkspace {
		t.Fatal("expected mute_workspace=true after update")
	}
}

func TestNotificationService_UpdatePreferences_ForwardsAccountFields(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()

	svc := newCRUDNotificationService(db)
	tz := "Asia/Tokyo"
	badge := "mentions_only"
	if err := svc.UpdatePreferences(ctx, "user-1", "ws-1", model.UpdateNotificationPreferenceRequest{
		Timezone:  &tz,
		BadgeMode: &badge,
	}); err != nil {
		t.Fatalf("UpdatePreferences: %v", err)
	}

	// Verify account-level settings were updated
	prefs, err := svc.GetPreferences(ctx, "user-1", "ws-1")
	if err != nil {
		t.Fatalf("GetPreferences: %v", err)
	}
	if prefs.Timezone != "Asia/Tokyo" {
		t.Fatalf("timezone = %q, want Asia/Tokyo forwarded to account settings", prefs.Timezone)
	}
	if prefs.BadgeMode != "mentions_only" {
		t.Fatalf("badge_mode = %q, want mentions_only forwarded to account settings", prefs.BadgeMode)
	}
}

func TestNotificationService_UpdatePreferences_RejectsInvalidTimezone(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()

	svc := newCRUDNotificationService(db)
	badTZ := "Fake/Zone"
	err := svc.UpdatePreferences(ctx, "user-1", "ws-1", model.UpdateNotificationPreferenceRequest{
		Timezone: &badTZ,
	})
	if err == nil {
		t.Fatal("expected error for invalid timezone in UpdatePreferences")
	}
}

// ---------------------------------------------------------------------------
// Emit: self-notification exclusion
// ---------------------------------------------------------------------------

func TestNotificationService_Emit_ExcludesActor(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "actor-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)

	svc := newCRUDNotificationService(db)

	// Actor is the only explicit recipient — should result in no notification
	if err := svc.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        "ws-1",
		ActorID:            "actor-1",
		EventType:          "story.assigned",
		EntityType:         "story",
		EntityID:           "story-1",
		Title:              "Self assign",
		Category:           model.NotifCategoryAssignments,
		Priority:           "normal",
		ExplicitRecipients: []string{"actor-1"},
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	var count int64
	db.Table("notifications").Count(&count)
	if count != 0 {
		t.Fatalf("count = %d, want 0 (actor excluded)", count)
	}
}

// ---------------------------------------------------------------------------
// Emit: via followers
// ---------------------------------------------------------------------------

func TestNotificationService_Emit_NotifiesFollowers(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "follower-1", true, "none", "09:00", 1, false, "all", "UTC", now, now)
	mustExecCRUDSvc(t, db, `INSERT INTO entity_followers (id, user_id, entity_type, entity_id, workspace_id, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"f-1", "follower-1", "story", "story-1", "ws-1", "manual", now)

	svc := newCRUDNotificationService(db)
	if err := svc.Emit(ctx, model.NotificationEventInput{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		EventType:   "story.updated",
		EntityType:  "story",
		EntityID:    "story-1",
		Title:       "Story updated",
		Category:    model.NotifCategoryStatusChanges,
		Priority:    "normal",
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	var count int64
	db.Table("notifications").Where("recipient_id = ?", "follower-1").Count(&count)
	if count != 1 {
		t.Fatalf("follower notification count = %d, want 1", count)
	}
}

// ---------------------------------------------------------------------------
// Emit: DND blocks notification
// ---------------------------------------------------------------------------

func TestNotificationService_Emit_DNDBlocksNotification(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()
	future := now.Add(1 * time.Hour)

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, dnd_until, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, true, future, "all", "UTC", now, now)

	svc := newCRUDNotificationService(db)
	if err := svc.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        "ws-1",
		ActorID:            "actor-1",
		EventType:          "story.assigned",
		EntityType:         "story",
		EntityID:           "story-1",
		Title:              "Assigned",
		Category:           model.NotifCategoryAssignments,
		Priority:           "normal",
		ExplicitRecipients: []string{"user-1"},
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	var count int64
	db.Table("notifications").Count(&count)
	if count != 0 {
		t.Fatalf("count = %d, want 0 (DND active)", count)
	}
}

// ---------------------------------------------------------------------------
// Emit: workspace mute blocks notification
// ---------------------------------------------------------------------------

func TestNotificationService_Emit_WorkspaceMuteBlocksNotification(t *testing.T) {
	db := newNotificationCRUDTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecCRUDSvc(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecCRUDSvc(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", true, `{}`, now, now)

	svc := newCRUDNotificationService(db)
	if err := svc.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        "ws-1",
		ActorID:            "actor-1",
		EventType:          "story.assigned",
		EntityType:         "story",
		EntityID:           "story-1",
		Title:              "Assigned",
		Category:           model.NotifCategoryAssignments,
		Priority:           "normal",
		ExplicitRecipients: []string{"user-1"},
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	var count int64
	db.Table("notifications").Count(&count)
	if count != 0 {
		t.Fatalf("count = %d, want 0 (workspace muted)", count)
	}
}
