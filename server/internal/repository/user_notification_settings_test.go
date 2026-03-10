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

func setupUserNotificationSettingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:user_notif_settings_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE user_notification_settings (
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
		)
	`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestUserNotificationSettingsRepository_Get_ReturnsDefaults(t *testing.T) {
	db := setupUserNotificationSettingsTestDB(t)
	repo := NewUserNotificationSettingsRepository(db)
	ctx := context.Background()

	settings, err := repo.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settings.UserID != "user-1" {
		t.Fatalf("user_id = %q, want user-1", settings.UserID)
	}
	if !settings.EmailEnabled {
		t.Fatal("expected default email_enabled=true")
	}
	if settings.EmailDigestFrequency != "daily" {
		t.Fatalf("digest_frequency = %q, want daily", settings.EmailDigestFrequency)
	}
	if settings.BadgeMode != "all" {
		t.Fatalf("badge_mode = %q, want all", settings.BadgeMode)
	}
	if settings.Timezone != "UTC" {
		t.Fatalf("timezone = %q, want UTC", settings.Timezone)
	}
}

func TestUserNotificationSettingsRepository_Upsert_Creates(t *testing.T) {
	db := setupUserNotificationSettingsTestDB(t)
	repo := NewUserNotificationSettingsRepository(db)
	ctx := context.Background()

	settings := &model.UserNotificationSettings{
		UserID:               "user-1",
		EmailEnabled:         false,
		EmailDigestFrequency: "weekly",
		EmailDigestTime:      "10:00",
		EmailDigestDay:       3,
		BadgeMode:            "mentions_only",
		Timezone:             "America/New_York",
	}
	if err := repo.Upsert(ctx, settings); err != nil {
		t.Fatalf("Upsert create: %v", err)
	}

	fetched, err := repo.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after create: %v", err)
	}
	if fetched.EmailEnabled {
		t.Fatal("expected email_enabled=false")
	}
	if fetched.EmailDigestFrequency != "weekly" {
		t.Fatalf("digest_frequency = %q, want weekly", fetched.EmailDigestFrequency)
	}
	if fetched.BadgeMode != "mentions_only" {
		t.Fatalf("badge_mode = %q, want mentions_only", fetched.BadgeMode)
	}
}

func TestUserNotificationSettingsRepository_Upsert_Updates(t *testing.T) {
	db := setupUserNotificationSettingsTestDB(t)
	repo := NewUserNotificationSettingsRepository(db)
	ctx := context.Background()

	settings := &model.UserNotificationSettings{
		UserID:               "user-1",
		EmailEnabled:         true,
		EmailDigestFrequency: "daily",
		EmailDigestTime:      "09:00",
		EmailDigestDay:       1,
		BadgeMode:            "all",
		Timezone:             "UTC",
	}
	if err := repo.Upsert(ctx, settings); err != nil {
		t.Fatalf("Upsert create: %v", err)
	}

	// Update
	settings.EmailEnabled = false
	settings.BadgeMode = "none"
	if err := repo.Upsert(ctx, settings); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}

	fetched, err := repo.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if fetched.EmailEnabled {
		t.Fatal("expected email_enabled=false after update")
	}
	if fetched.BadgeMode != "none" {
		t.Fatalf("badge_mode = %q, want none", fetched.BadgeMode)
	}

	// Ensure only one row exists
	var count int64
	if err := db.Table("user_notification_settings").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func TestUserNotificationSettingsRepository_Get_WithDNDUntil(t *testing.T) {
	db := setupUserNotificationSettingsTestDB(t)
	repo := NewUserNotificationSettingsRepository(db)
	ctx := context.Background()
	now := time.Now()
	future := now.Add(2 * time.Hour)

	settings := &model.UserNotificationSettings{
		UserID:               "user-1",
		EmailEnabled:         true,
		EmailDigestFrequency: "daily",
		EmailDigestTime:      "09:00",
		EmailDigestDay:       1,
		DoNotDisturb:         false,
		DNDUntil:             &future,
		BadgeMode:            "all",
		Timezone:             "UTC",
	}
	if err := repo.Upsert(ctx, settings); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	fetched, err := repo.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.DNDUntil == nil {
		t.Fatal("expected dnd_until to be set")
	}
	if !model.IsDNDActive(fetched.DoNotDisturb, fetched.DNDUntil, now) {
		t.Fatal("expected DND to be active with future dnd_until")
	}
}
