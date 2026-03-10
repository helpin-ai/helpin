package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type stubEmailSender struct {
	sent []sentEmail
	err  error
}

type sentEmail struct {
	to       string
	subject  string
	htmlBody string
	textBody string
}

func (s *stubEmailSender) SendEmail(to, subject, htmlBody, textBody string) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, sentEmail{
		to:       to,
		subject:  subject,
		htmlBody: htmlBody,
		textBody: textBody,
	})
	return nil
}

func TestLatestDigestCutoff(t *testing.T) {
	now := time.Date(2026, time.March, 10, 8, 0, 0, 0, time.UTC)

	dailyCutoff := latestDigestCutoff(now, &model.UserNotificationSettings{
		EmailDigestFrequency: "daily",
		EmailDigestTime:      "09:00",
		Timezone:             "UTC",
	})
	expectedDaily := time.Date(2026, time.March, 9, 9, 0, 0, 0, time.UTC)
	if !dailyCutoff.Equal(expectedDaily) {
		t.Fatalf("daily cutoff = %s, want %s", dailyCutoff, expectedDaily)
	}

	weeklyCutoff := latestDigestCutoff(now, &model.UserNotificationSettings{
		EmailDigestFrequency: "weekly",
		EmailDigestTime:      "09:00",
		EmailDigestDay:       1,
		Timezone:             "UTC",
	})
	expectedWeekly := time.Date(2026, time.March, 9, 9, 0, 0, 0, time.UTC)
	if !weeklyCutoff.Equal(expectedWeekly) {
		t.Fatalf("weekly cutoff = %s, want %s", weeklyCutoff, expectedWeekly)
	}
}

func TestProcessPendingDigests_SendsDueDigestAndSkipsResolvedNotifications(t *testing.T) {
	db := newNotificationDigestTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	mustExecDigest(t, db, `INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"user-1", "user@example.com", "hash", "Digest User", now, now)
	mustExecDigest(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)

	seedNotificationDigestCase(t, db, now)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		emailer,
	)

	if err := service.ProcessPendingDigests(ctx, now); err != nil {
		t.Fatalf("ProcessPendingDigests: %v", err)
	}

	if len(emailer.sent) != 1 {
		t.Fatalf("expected 1 digest email, got %d", len(emailer.sent))
	}
	if got := emailer.sent[0].to; got != "user@example.com" {
		t.Fatalf("digest recipient = %q, want %q", got, "user@example.com")
	}
	if !strings.Contains(emailer.sent[0].textBody, "Story A was updated") {
		t.Fatalf("digest body missing due unread notification: %q", emailer.sent[0].textBody)
	}
	if strings.Contains(emailer.sent[0].textBody, "Story B was resolved") {
		t.Fatalf("digest body should not include read notification: %q", emailer.sent[0].textBody)
	}
	if strings.Contains(emailer.sent[0].textBody, "Story C will wait") {
		t.Fatalf("digest body should not include future notification: %q", emailer.sent[0].textBody)
	}

	assertDeliveryStatus(t, db, "delivery-due-unread", "delivered")
	assertDeliveryStatus(t, db, "delivery-due-read", "skipped")
	assertDeliveryStatus(t, db, "delivery-future-unread", "pending")
}

func TestProcessPendingDigests_RespectsCurrentWorkspaceEmailPreferences(t *testing.T) {
	db := newNotificationDigestTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	mustExecDigest(t, db, `INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"user-1", "user@example.com", "hash", "Digest User", now, now)
	mustExecDigest(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)
	mustExecDigest(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", false, `{"comments":{"in_app":true,"email":false}}`, now, now)

	seedNotificationDigestCase(t, db, now)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		emailer,
	)

	if err := service.ProcessPendingDigests(ctx, now); err != nil {
		t.Fatalf("ProcessPendingDigests: %v", err)
	}

	if len(emailer.sent) != 0 {
		t.Fatalf("expected no digest email when workspace email category is disabled, got %d", len(emailer.sent))
	}

	assertDeliveryStatus(t, db, "delivery-due-unread", "skipped")
	assertDeliveryStatus(t, db, "delivery-due-read", "skipped")
	assertDeliveryStatus(t, db, "delivery-future-unread", "pending")
}

func TestProcessPendingDigests_SkipsWhenAccountEmailDisabled(t *testing.T) {
	db := newNotificationDigestTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	mustExecDigest(t, db, `INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"user-1", "user@example.com", "hash", "Digest User", now, now)
	mustExecDigest(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", false, "daily", "09:00", 1, false, "all", "UTC", now, now)

	seedNotificationDigestCase(t, db, now)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		emailer,
	)

	if err := service.ProcessPendingDigests(ctx, now); err != nil {
		t.Fatalf("ProcessPendingDigests: %v", err)
	}

	if len(emailer.sent) != 0 {
		t.Fatalf("expected no digest email when account email is disabled, got %d", len(emailer.sent))
	}
	assertDeliveryStatus(t, db, "delivery-due-unread", "skipped")
	assertDeliveryStatus(t, db, "delivery-due-read", "skipped")
	assertDeliveryStatus(t, db, "delivery-future-unread", "skipped")
}

func TestProcessPendingDigests_SkipsWhenDNDUntilIsActive(t *testing.T) {
	db := newNotificationDigestTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	future := now.Add(30 * time.Minute)

	mustExecDigest(t, db, `INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"user-1", "user@example.com", "hash", "Digest User", now, now)
	mustExecDigest(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, dnd_until, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, future, "all", "UTC", now, now)

	seedNotificationDigestCase(t, db, now)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		emailer,
	)

	if err := service.ProcessPendingDigests(ctx, now); err != nil {
		t.Fatalf("ProcessPendingDigests: %v", err)
	}

	if len(emailer.sent) != 0 {
		t.Fatalf("expected no digest email while dnd_until is active, got %d", len(emailer.sent))
	}
	assertDeliveryStatus(t, db, "delivery-due-unread", "skipped")
	assertDeliveryStatus(t, db, "delivery-due-read", "skipped")
	assertDeliveryStatus(t, db, "delivery-future-unread", "skipped")
}

func TestProcessPendingDigests_FailsWhenUserRepositoryMissing(t *testing.T) {
	db := newNotificationDigestTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	mustExecDigest(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)

	seedNotificationDigestCase(t, db, now)

	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		nil,
		nil,
		nil,
		nil,
		&stubEmailSender{},
	)

	if err := service.ProcessPendingDigests(ctx, now); err != nil {
		t.Fatalf("ProcessPendingDigests: %v", err)
	}

	assertDeliveryStatus(t, db, "delivery-due-unread", "failed")
	assertDeliveryStatus(t, db, "delivery-due-read", "failed")
	assertDeliveryStatus(t, db, "delivery-future-unread", "pending")
}

func TestProcessPendingDigests_MarksIncludedRowsFailedOnSendError(t *testing.T) {
	db := newNotificationDigestTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	mustExecDigest(t, db, `INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"user-1", "user@example.com", "hash", "Digest User", now, now)
	mustExecDigest(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-1", "user-1", true, "daily", "09:00", 1, false, "all", "UTC", now, now)

	seedNotificationDigestCase(t, db, now)

	emailer := &stubEmailSender{err: fmt.Errorf("postmark timeout")}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		emailer,
	)

	if err := service.ProcessPendingDigests(ctx, now); err != nil {
		t.Fatalf("ProcessPendingDigests: %v", err)
	}

	assertDeliveryStatus(t, db, "delivery-due-unread", "failed")
	assertDeliveryStatus(t, db, "delivery-due-read", "skipped")
	assertDeliveryStatus(t, db, "delivery-future-unread", "pending")
}

func newNotificationDigestTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification-digest-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			avatar_url TEXT,
			default_workspace_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
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
		`CREATE TABLE notifications (
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
			event_count INTEGER NOT NULL,
			last_event_at DATETIME,
			status TEXT NOT NULL,
			snoozed_until DATETIME,
			read_at DATETIME,
			archived_at DATETIME,
			priority TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE notification_events (
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
		)`,
		`CREATE TABLE notification_deliveries (
			id TEXT PRIMARY KEY,
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
		mustExecDigest(t, db, stmt)
	}

	return db
}

func seedNotificationDigestCase(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()

	dueAt := time.Date(2026, time.March, 10, 8, 30, 0, 0, time.UTC)
	futureAt := time.Date(2026, time.March, 10, 9, 30, 0, 0, time.UTC)

	mustExecDigest(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-due-unread", "ws-1", "user-1", "story", "story-1", "story.comment", "Story A was updated", "comments", 1, dueAt, "unread", "normal", dueAt, dueAt)
	mustExecDigest(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-due-read", "ws-1", "user-1", "story", "story-2", "story.comment", "Story B was resolved", "comments", 1, dueAt, "read", "normal", dueAt, dueAt)
	mustExecDigest(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-future-unread", "ws-1", "user-1", "story", "story-3", "story.comment", "Story C will wait", "comments", 1, futureAt, "unread", "normal", futureAt, futureAt)

	mustExecDigest(t, db, `INSERT INTO notification_events (id, notification_id, event_type, title, category, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-due-unread", "notif-due-unread", "story.comment", "Story A was updated", "comments", "normal", dueAt)
	mustExecDigest(t, db, `INSERT INTO notification_events (id, notification_id, event_type, title, category, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-due-read", "notif-due-read", "story.comment", "Story B was resolved", "comments", "normal", dueAt)
	mustExecDigest(t, db, `INSERT INTO notification_events (id, notification_id, event_type, title, category, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-future-unread", "notif-future-unread", "story.comment", "Story C will wait", "comments", "normal", futureAt)

	mustExecDigest(t, db, `INSERT INTO notification_deliveries (id, notification_event_id, channel, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"delivery-due-unread", "event-due-unread", "digest", "pending", dueAt, dueAt)
	mustExecDigest(t, db, `INSERT INTO notification_deliveries (id, notification_event_id, channel, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"delivery-due-read", "event-due-read", "digest", "pending", dueAt, dueAt)
	mustExecDigest(t, db, `INSERT INTO notification_deliveries (id, notification_event_id, channel, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"delivery-future-unread", "event-future-unread", "digest", "pending", futureAt, now)
}

func assertDeliveryStatus(t *testing.T, db *gorm.DB, deliveryID, expectedStatus string) {
	t.Helper()

	var row struct {
		Status string
	}
	if err := db.Raw(`SELECT status FROM notification_deliveries WHERE id = ?`, deliveryID).Scan(&row).Error; err != nil {
		t.Fatalf("load delivery %s: %v", deliveryID, err)
	}
	if row.Status != expectedStatus {
		t.Fatalf("delivery %s status = %q, want %q", deliveryID, row.Status, expectedStatus)
	}
}

func mustExecDigest(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
