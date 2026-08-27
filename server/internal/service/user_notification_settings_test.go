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

func newUserSettingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:user-settings-svc-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	mustExecUserSettings(t, db, `CREATE TABLE user_notification_settings (
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
	)`)
	return db
}

func mustExecUserSettings(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// ---------------------------------------------------------------------------
// ValidateTimezone
// ---------------------------------------------------------------------------

func TestValidateTimezone_ValidIANA(t *testing.T) {
	valid := []string{"", "UTC", "Local", "America/New_York", "Europe/London", "Asia/Tokyo", "Pacific/Auckland", "Japan"}
	for _, tz := range valid {
		if err := ValidateTimezone(tz); err != nil {
			t.Errorf("ValidateTimezone(%q) returned error: %v", tz, err)
		}
	}
}

func TestValidateTimezone_InvalidIANA(t *testing.T) {
	invalid := []string{"Not/A/Zone", "Foo", "UTC+5"}
	for _, tz := range invalid {
		if err := ValidateTimezone(tz); err == nil {
			t.Errorf("ValidateTimezone(%q) expected error, got nil", tz)
		}
	}
}

// ---------------------------------------------------------------------------
// UserNotificationSettingsService.Get
// ---------------------------------------------------------------------------

func TestUserNotificationSettingsService_Get_DefaultsForNewUser(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	settings, err := svc.Get(ctx, "user-new")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settings.UserID != "user-new" {
		t.Fatalf("user_id = %q, want user-new", settings.UserID)
	}
	if !settings.EmailEnabled {
		t.Fatal("default email_enabled should be true")
	}
	if settings.Timezone != "UTC" {
		t.Fatalf("default timezone = %q, want UTC", settings.Timezone)
	}
}

func TestUserNotificationSettingsService_Get_ExistingUser(t *testing.T) {
	db := newUserSettingsTestDB(t)
	ctx := context.Background()
	now := time.Now()

	mustExecUserSettings(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", false, "weekly", "10:00", 3, true, "mentions_only", "America/Chicago", now, now)

	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	settings, err := svc.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settings.EmailEnabled {
		t.Fatal("expected email_enabled=false")
	}
	if settings.EmailDigestFrequency != "weekly" {
		t.Fatalf("digest_frequency = %q, want weekly", settings.EmailDigestFrequency)
	}
	if !settings.DoNotDisturb {
		t.Fatal("expected do_not_disturb=true")
	}
	if settings.BadgeMode != "mentions_only" {
		t.Fatalf("badge_mode = %q, want mentions_only", settings.BadgeMode)
	}
}

// ---------------------------------------------------------------------------
// UserNotificationSettingsService.Update
// ---------------------------------------------------------------------------

func TestUserNotificationSettingsService_Update_PartialFields(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	badge := "mentions_only"
	settings, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		BadgeMode: &badge,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if settings.BadgeMode != "mentions_only" {
		t.Fatalf("badge_mode = %q, want mentions_only after update", settings.BadgeMode)
	}
	// Other fields should remain defaults
	if settings.EmailDigestFrequency != "daily" {
		t.Fatalf("digest_frequency = %q, want daily", settings.EmailDigestFrequency)
	}
}

func TestUserNotificationSettingsService_Update_AllFields(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	freq := "weekly"
	digestTime := "14:00"
	day := 5
	dnd := true
	badge := "none"
	tz := "Europe/Berlin"

	settings, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		EmailDigestFrequency: &freq,
		EmailDigestTime:      &digestTime,
		EmailDigestDay:       &day,
		DoNotDisturb:         &dnd,
		BadgeMode:            &badge,
		Timezone:             &tz,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if settings.EmailDigestFrequency != "weekly" {
		t.Fatalf("digest_frequency = %q, want weekly", settings.EmailDigestFrequency)
	}
	if settings.EmailDigestTime != "14:00" {
		t.Fatalf("digest_time = %q, want 14:00", settings.EmailDigestTime)
	}
	if settings.EmailDigestDay != 5 {
		t.Fatalf("digest_day = %d, want 5", settings.EmailDigestDay)
	}
	if !settings.DoNotDisturb {
		t.Fatal("expected do_not_disturb=true")
	}
	if settings.BadgeMode != "none" {
		t.Fatalf("badge_mode = %q, want none", settings.BadgeMode)
	}
	if settings.Timezone != "Europe/Berlin" {
		t.Fatalf("timezone = %q, want Europe/Berlin", settings.Timezone)
	}
}

func TestUserNotificationSettingsService_Update_RejectsInvalidTimezone(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	badTZ := "Not/A/Timezone"
	_, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		Timezone: &badTZ,
	})
	if err == nil {
		t.Fatal("expected error for invalid timezone, got nil")
	}
}

func TestUserNotificationSettingsService_Update_AcceptsValidTimezone(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	tz := "Asia/Tokyo"
	settings, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		Timezone: &tz,
	})
	if err != nil {
		t.Fatalf("Update with valid timezone: %v", err)
	}
	if settings.Timezone != "Asia/Tokyo" {
		t.Fatalf("timezone = %q, want Asia/Tokyo", settings.Timezone)
	}
}

func TestUserNotificationSettingsService_Update_DNDUntilFutureSetsDoNotDisturb(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	future := time.Now().Add(1 * time.Hour)
	settings, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		DNDUntil: &future,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !settings.DoNotDisturb {
		t.Fatal("expected do_not_disturb=true when dnd_until is in the future")
	}
}

func TestUserNotificationSettingsService_Update_DisableDNDClearsDNDUntil(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	// First enable DND with a future time
	future := time.Now().Add(1 * time.Hour)
	dnd := true
	_, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		DoNotDisturb: &dnd,
		DNDUntil:     &future,
	})
	if err != nil {
		t.Fatalf("Enable DND: %v", err)
	}

	// Now disable DND without specifying DNDUntil — it should clear
	dnd = false
	settings, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		DoNotDisturb: &dnd,
	})
	if err != nil {
		t.Fatalf("Disable DND: %v", err)
	}
	if settings.DoNotDisturb {
		t.Fatal("expected do_not_disturb=false after disabling")
	}
	if settings.DNDUntil != nil {
		t.Fatal("expected dnd_until to be cleared when disabling DND")
	}
}

func TestUserNotificationSettingsService_Update_MultipleUpdatesPreserveValues(t *testing.T) {
	db := newUserSettingsTestDB(t)
	svc := NewUserNotificationSettingsService(repository.NewUserNotificationSettingsRepository(db))
	ctx := context.Background()

	// First update: change badge mode
	badge := "mentions_only"
	if _, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		BadgeMode: &badge,
	}); err != nil {
		t.Fatalf("Update 1: %v", err)
	}

	// Second update: change timezone
	tz := "Europe/Paris"
	settings, err := svc.Update(ctx, "user-1", model.UpdateUserNotificationSettingsRequest{
		Timezone: &tz,
	})
	if err != nil {
		t.Fatalf("Update 2: %v", err)
	}

	// badge_mode should still be mentions_only from first update
	if settings.BadgeMode != "mentions_only" {
		t.Fatalf("badge_mode = %q, want mentions_only preserved from first update", settings.BadgeMode)
	}
	if settings.Timezone != "Europe/Paris" {
		t.Fatalf("timezone = %q, want Europe/Paris", settings.Timezone)
	}
}
