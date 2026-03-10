package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNotificationPreferenceRepositoryShouldNotify_RespectsDNDUntil(t *testing.T) {
	db := newNotificationPreferenceTestDB(t)
	ctx := context.Background()
	now := time.Now()
	future := now.Add(30 * time.Minute)

	mustExecNotificationPref(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, dnd_until, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, future, "all", "UTC", now, now)
	mustExecNotificationPref(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", false, `{}`, now, now)

	repo := NewNotificationPreferenceRepository(db)
	shouldNotify, err := repo.ShouldNotify(ctx, "user-1", "ws-1", "story.assigned", "in_app", "")
	if err != nil {
		t.Fatalf("ShouldNotify: %v", err)
	}
	if shouldNotify {
		t.Fatal("expected in_app notification to be blocked while dnd_until is active")
	}
}

func TestNotificationPreferenceRepositoryShouldNotify_RespectsWorkspaceMute(t *testing.T) {
	db := newNotificationPreferenceTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecNotificationPref(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecNotificationPref(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", true, `{}`, now, now)

	repo := NewNotificationPreferenceRepository(db)
	shouldNotify, err := repo.ShouldNotify(ctx, "user-1", "ws-1", "story.assigned", "in_app", "")
	if err != nil {
		t.Fatalf("ShouldNotify: %v", err)
	}
	if shouldNotify {
		t.Fatal("expected workspace mute to block in_app notification")
	}
}

func TestNotificationPreferenceRepositoryShouldNotify_UsesWorkspaceCategoryPreference(t *testing.T) {
	db := newNotificationPreferenceTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecNotificationPref(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecNotificationPref(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", false, `{"comments":{"in_app":false,"email":true}}`, now, now)

	repo := NewNotificationPreferenceRepository(db)
	shouldNotify, err := repo.ShouldNotify(ctx, "user-1", "ws-1", "comment.created", "in_app", "")
	if err != nil {
		t.Fatalf("ShouldNotify: %v", err)
	}
	if shouldNotify {
		t.Fatal("expected workspace category preference to block in_app notification")
	}
}

func TestNotificationPreferenceRepositoryShouldNotify_TeamOverrideWins(t *testing.T) {
	db := newNotificationPreferenceTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecNotificationPref(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecNotificationPref(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", false, `{"comments":{"in_app":false,"email":false}}`, now, now)
	mustExecNotificationPref(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, team_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"pref-team-1", "user-1", "ws-1", "team-1", false, `{"comments":{"in_app":true,"email":true}}`, now, now)

	repo := NewNotificationPreferenceRepository(db)
	shouldNotify, err := repo.ShouldNotify(ctx, "user-1", "ws-1", "comment.created", "in_app", "team-1")
	if err != nil {
		t.Fatalf("ShouldNotify: %v", err)
	}
	if !shouldNotify {
		t.Fatal("expected team override to re-enable in_app notification")
	}
}

func TestNotificationPreferenceRepositoryShouldNotify_UsesLegacyExactEventFallback(t *testing.T) {
	db := newNotificationPreferenceTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecNotificationPref(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecNotificationPref(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", false, `{"story.created":{"in_app":false}}`, now, now)

	repo := NewNotificationPreferenceRepository(db)
	shouldNotify, err := repo.ShouldNotify(ctx, "user-1", "ws-1", "story.created", "in_app", "")
	if err != nil {
		t.Fatalf("ShouldNotify: %v", err)
	}
	if shouldNotify {
		t.Fatal("expected legacy exact event fallback to block in_app notification")
	}
}

func TestNotificationPreferenceRepositoryShouldNotify_EmailDisabledAtAccountLevelWins(t *testing.T) {
	db := newNotificationPreferenceTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecNotificationPref(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", false, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecNotificationPref(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", false, `{"comments":{"in_app":true,"email":true}}`, now, now)

	repo := NewNotificationPreferenceRepository(db)
	shouldNotify, err := repo.ShouldNotify(ctx, "user-1", "ws-1", "comment.created", "email", "")
	if err != nil {
		t.Fatalf("ShouldNotify: %v", err)
	}
	if shouldNotify {
		t.Fatal("expected account email_enabled=false to block email delivery")
	}
}

func newNotificationPreferenceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification-pref-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE user_notification_settings (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			email_enabled BOOLEAN NOT NULL,
			email_digest_frequency TEXT NOT NULL,
			email_digest_time TEXT NOT NULL,
			email_digest_day INTEGER NOT NULL,
			do_not_disturb BOOLEAN NOT NULL,
			dnd_until DATETIME,
			badge_mode TEXT NOT NULL,
			timezone TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE notification_preferences (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			team_id TEXT,
			mute_workspace BOOLEAN NOT NULL,
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
	}

	for _, stmt := range statements {
		mustExecNotificationPref(t, db, stmt)
	}

	return db
}

func mustExecNotificationPref(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
