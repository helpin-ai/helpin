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

func TestEmit_CreatesNotificationAndImmediateEmailDelivery(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme Workspace", now)
	seedNotificationServiceUser(t, db, "user-1", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "immediate", nil, now)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	)

	if err := service.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        "ws-1",
		ActorID:            "actor-1",
		EventType:          "comment.created",
		EntityType:         "task",
		EntityID:           "task-1",
		Title:              "Task comment added",
		Body:               "Please review the new comment",
		Category:           model.NotifCategoryComments,
		Priority:           "normal",
		ActorSnapshot:      model.JSONB{"name": "Alice"},
		ExplicitRecipients: []string{"user-1", "actor-1"},
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	var notif model.Notification
	if err := db.WithContext(ctx).Where("recipient_id = ?", "user-1").First(&notif).Error; err != nil {
		t.Fatalf("load notification: %v", err)
	}
	if notif.Status != "unread" {
		t.Fatalf("notification status = %q, want unread", notif.Status)
	}
	if notif.EventCount != 1 {
		t.Fatalf("notification event_count = %d, want 1", notif.EventCount)
	}
	if notif.EventType != "comment.created" {
		t.Fatalf("notification event_type = %q, want comment.created", notif.EventType)
	}

	var eventCount int64
	if err := db.WithContext(ctx).Table("notification_events").Count(&eventCount).Error; err != nil {
		t.Fatalf("count notification events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("notification event count = %d, want 1", eventCount)
	}

	deliveries := loadNotificationServiceDeliveries(t, db)
	if len(deliveries) != 2 {
		t.Fatalf("delivery count = %d, want 2", len(deliveries))
	}
	if deliveries[0].Channel != "email" || deliveries[0].Status != "delivered" {
		t.Fatalf("email delivery = %+v, want delivered email", deliveries[0])
	}
	if deliveries[1].Channel != "in_app" || deliveries[1].Status != "delivered" {
		t.Fatalf("in_app delivery = %+v, want delivered in_app", deliveries[1])
	}

	if len(emailer.sent) != 1 {
		t.Fatalf("sent email count = %d, want 1", len(emailer.sent))
	}
	if emailer.sent[0].to != "user@example.com" {
		t.Fatalf("email recipient = %q, want user@example.com", emailer.sent[0].to)
	}
	if !strings.Contains(emailer.sent[0].subject, "[Acme Workspace]") || !strings.Contains(emailer.sent[0].subject, "Task comment added") {
		t.Fatalf("email subject = %q, want workspace-prefixed subject", emailer.sent[0].subject)
	}
}

func TestEmit_UpdatesExistingNotificationAndAddsEvent(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	earlier := now.Add(-2 * time.Hour)
	readAt := earlier.Add(10 * time.Minute)

	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme Workspace", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "none", nil, now)
	mustExecNotificationService(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, actor_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, read_at, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-1", "ws-1", "user-1", "actor-0", "task", "task-1", "task.assigned", "Initial assignment", model.NotifCategoryAssignments, 1, earlier, "read", readAt, "normal", earlier, earlier)

	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		nil,
		"",
	)

	if err := service.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        "ws-1",
		ActorID:            "actor-2",
		EventType:          "task.updated",
		EntityType:         "task",
		EntityID:           "task-1",
		Title:              "Task changed",
		Category:           model.NotifCategoryStatusChanges,
		Priority:           "normal",
		ExplicitRecipients: []string{"user-1"},
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	var notif model.Notification
	if err := db.WithContext(ctx).Where("id = ?", "notif-1").First(&notif).Error; err != nil {
		t.Fatalf("load notification: %v", err)
	}
	if notif.Status != "unread" {
		t.Fatalf("updated notification status = %q, want unread", notif.Status)
	}
	if notif.EventCount != 2 {
		t.Fatalf("updated notification event_count = %d, want 2", notif.EventCount)
	}
	if notif.EventType != "task.updated" {
		t.Fatalf("updated notification event_type = %q, want task.updated", notif.EventType)
	}

	var notifCount int64
	if err := db.WithContext(ctx).Table("notifications").Count(&notifCount).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if notifCount != 1 {
		t.Fatalf("notification row count = %d, want 1", notifCount)
	}

	var eventCount int64
	if err := db.WithContext(ctx).Table("notification_events").Where("notification_id = ?", "notif-1").Count(&eventCount).Error; err != nil {
		t.Fatalf("count notification events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("new notification event count = %d, want 1", eventCount)
	}

	deliveries := loadNotificationServiceDeliveries(t, db)
	if len(deliveries) != 1 || deliveries[0].Channel != "in_app" || deliveries[0].Status != "delivered" {
		t.Fatalf("deliveries = %+v, want one delivered in_app row", deliveries)
	}
}

func TestEmit_EmailStillSendsWhenInAppDisabled(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	seedNotificationServiceUserSettings(t, db, "user-1", true, "immediate", nil, now)
	mustExecNotificationService(t, db, `INSERT INTO notification_preferences (id, user_id, workspace_id, mute_workspace, channel_preferences, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"pref-1", "user-1", "ws-1", false, `{"comments":{"in_app":false,"email":true}}`, now, now)

	seedNotificationServiceUser(t, db, "user-1", "user@example.com", "Recipient", now)
	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	)

	if err := service.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        "ws-1",
		ActorID:            "actor-1",
		EventType:          "comment.created",
		EntityType:         "task",
		EntityID:           "task-1",
		Title:              "Task comment added",
		Category:           model.NotifCategoryComments,
		Priority:           "normal",
		ExplicitRecipients: []string{"user-1"},
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	assertNotificationServiceCount(t, db, "notifications", 1)
	assertNotificationServiceCount(t, db, "notification_events", 1)
	assertNotificationServiceCount(t, db, "notification_deliveries", 2)
	if len(emailer.sent) != 1 {
		t.Fatalf("sent email count = %d, want 1", len(emailer.sent))
	}
	page, err := service.List(ctx, "user-1", "ws-1", "", "", 20, nil)
	if err != nil || len(page.Data) != 0 || page.UnreadCount != 0 {
		t.Fatalf("email-only notification leaked into inbox: %+v %v", page, err)
	}

}

func TestEmit_SkipFollowersLimitsDeliveryToExplicitRecipients(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme Workspace", now)
	seedNotificationServiceUser(t, db, "user-1", "user1@example.com", "Explicit Recipient", now)
	seedNotificationServiceUser(t, db, "user-2", "user2@example.com", "Follower Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "none", nil, now)
	seedNotificationServiceUserSettings(t, db, "user-2", true, "none", nil, now)
	mustExecNotificationService(t, db, `INSERT INTO entity_followers (id, user_id, entity_type, entity_id, workspace_id, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"follower-1", "user-2", "task", "task-1", "ws-1", "watching", now)

	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		nil,
		"",
	)

	if err := service.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:        "ws-1",
		ActorID:            "actor-1",
		EventType:          "comment.mention",
		EntityType:         "task",
		EntityID:           "task-1",
		Title:              "Mentioned you in a comment",
		Category:           model.NotifCategoryMentions,
		Priority:           "high",
		ExplicitRecipients: []string{"user-1"},
		SkipFollowers:      true,
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	var notifications []model.Notification
	if err := db.WithContext(ctx).Order("recipient_id ASC").Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}
	if notifications[0].RecipientID != "user-1" {
		t.Fatalf("recipient_id = %q, want user-1", notifications[0].RecipientID)
	}
	if notifications[0].EventType != "comment.mention" {
		t.Fatalf("event_type = %q, want comment.mention", notifications[0].EventType)
	}
}

func TestEmit_SupportReplyCreatesPendingDelayedEmailDelivery(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)

	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme Workspace", now)
	seedNotificationServiceUser(t, db, "user-1", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "immediate", nil, now)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	)

	if err := service.Emit(ctx, model.NotificationEventInput{
		WorkspaceID:         "ws-1",
		EventType:           "support_conversation.customer_reply",
		EntityType:          "support_conversation",
		EntityID:            "conv-1",
		Title:               "Customer replied in Billing question",
		Category:            model.NotifCategorySupportReplies,
		Priority:            "high",
		ExplicitRecipients:  []string{"user-1"},
		SkipFollowers:       true,
		DelayedEmailChannel: "support_reply_email",
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	deliveries := loadNotificationServiceDeliveries(t, db)
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %+v, want in_app + delayed email rows", deliveries)
	}
	if deliveries[0].Channel != "in_app" || deliveries[0].Status != "delivered" {
		t.Fatalf("first delivery = %+v, want delivered in_app", deliveries[0])
	}
	if deliveries[1].Channel != "support_reply_email" || deliveries[1].Status != "pending" {
		t.Fatalf("second delivery = %+v, want pending support_reply_email", deliveries[1])
	}
	if len(emailer.sent) != 0 {
		t.Fatalf("sent email count = %d, want 0", len(emailer.sent))
	}
}

func TestProcessPendingSupportReplyEmails_SendsAfterDelay(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	dueAt := now.Add(-4 * time.Minute)

	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme Workspace", now)
	seedNotificationServiceUser(t, db, "user-1", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "immediate", nil, now)

	mustExecNotificationService(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, body, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-1", "ws-1", "user-1", "support_conversation", "conv-1", "support_conversation.customer_reply", "Customer replied in Billing question", "Need help with billing", model.NotifCategorySupportReplies, 1, dueAt, "unread", "high", dueAt, dueAt)
	mustExecNotificationService(t, db, `INSERT INTO notification_events (id, notification_id, event_type, title, category, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-1", "notif-1", "support_conversation.customer_reply", "Customer replied in Billing question", model.NotifCategorySupportReplies, "high", dueAt)
	mustExecNotificationService(t, db, `INSERT INTO notification_deliveries (id, notification_event_id, channel, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"delivery-1", "event-1", "support_reply_email", "pending", dueAt, dueAt)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	)

	if err := service.ProcessPendingSupportReplyEmails(ctx, now); err != nil {
		t.Fatalf("ProcessPendingSupportReplyEmails: %v", err)
	}

	if len(emailer.sent) != 1 {
		t.Fatalf("sent email count = %d, want 1", len(emailer.sent))
	}

	deliveries := loadNotificationServiceDeliveries(t, db)
	if len(deliveries) != 1 || deliveries[0].Channel != "support_reply_email" || deliveries[0].Status != "delivered" {
		t.Fatalf("deliveries = %+v, want delivered support_reply_email", deliveries)
	}
}

func TestProcessPendingSupportReplyEmails_SkipsHandledNotifications(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	dueAt := now.Add(-4 * time.Minute)

	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme Workspace", now)
	seedNotificationServiceUser(t, db, "user-1", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "immediate", nil, now)

	mustExecNotificationService(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, latest_event_category, event_count, last_event_at, status, read_at, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-1", "ws-1", "user-1", "support_conversation", "conv-1", "support_conversation.customer_reply", "Customer replied in Billing question", model.NotifCategorySupportReplies, 1, dueAt, "read", now, "high", dueAt, dueAt)
	mustExecNotificationService(t, db, `INSERT INTO notification_events (id, notification_id, event_type, title, category, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-1", "notif-1", "support_conversation.customer_reply", "Customer replied in Billing question", model.NotifCategorySupportReplies, "high", dueAt)
	mustExecNotificationService(t, db, `INSERT INTO notification_deliveries (id, notification_event_id, channel, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"delivery-1", "event-1", "support_reply_email", "pending", dueAt, dueAt)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	)

	if err := service.ProcessPendingSupportReplyEmails(ctx, now); err != nil {
		t.Fatalf("ProcessPendingSupportReplyEmails: %v", err)
	}

	if len(emailer.sent) != 0 {
		t.Fatalf("sent email count = %d, want 0", len(emailer.sent))
	}

	deliveries := loadNotificationServiceDeliveries(t, db)
	if len(deliveries) != 1 || deliveries[0].Status != "skipped" {
		t.Fatalf("deliveries = %+v, want skipped support_reply_email", deliveries)
	}
	if deliveries[0].Error == nil || !strings.Contains(*deliveries[0].Error, "already handled") {
		t.Fatalf("delivery error = %v, want handled message", deliveries[0].Error)
	}
}

func TestProcessPendingSupportReplyEmails_BatchesToLatestPendingReply(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	older := now.Add(-5 * time.Minute)
	newer := now.Add(-4 * time.Minute)

	seedNotificationServiceWorkspace(t, db, "ws-1", "Acme Workspace", now)
	seedNotificationServiceUser(t, db, "user-1", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user-1", true, "immediate", nil, now)

	mustExecNotificationService(t, db, `INSERT INTO notifications (id, workspace_id, recipient_id, entity_type, entity_id, event_type, title, body, latest_event_category, event_count, last_event_at, status, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"notif-1", "ws-1", "user-1", "support_conversation", "conv-1", "support_conversation.customer_reply", "Customer replied in Billing question", "Latest reply body", model.NotifCategorySupportReplies, 2, newer, "unread", "high", older, newer)
	mustExecNotificationService(t, db, `INSERT INTO notification_events (id, notification_id, event_type, title, category, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-1", "notif-1", "support_conversation.customer_reply", "Older reply", model.NotifCategorySupportReplies, "high", older)
	mustExecNotificationService(t, db, `INSERT INTO notification_events (id, notification_id, event_type, title, category, priority, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"event-2", "notif-1", "support_conversation.customer_reply", "Newer reply", model.NotifCategorySupportReplies, "high", newer)
	mustExecNotificationService(t, db, `INSERT INTO notification_deliveries (id, notification_event_id, channel, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"delivery-1", "event-1", "support_reply_email", "pending", older, older)
	mustExecNotificationService(t, db, `INSERT INTO notification_deliveries (id, notification_event_id, channel, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"delivery-2", "event-2", "support_reply_email", "pending", newer, newer)

	emailer := &stubEmailSender{}
	service := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	)

	if err := service.ProcessPendingSupportReplyEmails(ctx, now); err != nil {
		t.Fatalf("ProcessPendingSupportReplyEmails: %v", err)
	}

	if len(emailer.sent) != 1 {
		t.Fatalf("sent email count = %d, want 1", len(emailer.sent))
	}

	deliveries := loadNotificationServiceDeliveries(t, db)
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %+v, want 2 rows", deliveries)
	}
	if deliveries[0].Channel != "support_reply_email" || deliveries[0].Status != "skipped" {
		t.Fatalf("first delivery = %+v, want skipped older delivery", deliveries[0])
	}
	if deliveries[1].Channel != "support_reply_email" || deliveries[1].Status != "delivered" {
		t.Fatalf("second delivery = %+v, want delivered latest delivery", deliveries[1])
	}
}

func TestBuildDeliveryPlans_ImmediateEmailBranches(t *testing.T) {
	now := time.Date(2026, time.March, 10, 10, 0, 0, 0, time.UTC)
	event := model.NotificationEventInput{
		WorkspaceID: "ws-1",
		EventType:   "comment.created",
		EntityType:  "task",
		EntityID:    "task-1",
		Title:       "Task comment added",
		Category:    model.NotifCategoryComments,
	}

	tests := []struct {
		name           string
		emailEnabled   bool
		userEmail      string
		userRepo       bool
		emailClient    *stubEmailSender
		wantChannel    string
		wantStatus     string
		wantErrorMatch string
	}{
		{
			name:           "email client missing",
			emailEnabled:   true,
			userEmail:      "user@example.com",
			userRepo:       true,
			wantChannel:    "email",
			wantStatus:     "skipped",
			wantErrorMatch: "email client not configured",
		},
		{
			name:           "user repo missing",
			emailEnabled:   true,
			userEmail:      "user@example.com",
			userRepo:       false,
			emailClient:    &stubEmailSender{},
			wantChannel:    "email",
			wantStatus:     "failed",
			wantErrorMatch: "user repository not configured",
		},
		{
			name:           "recipient email unavailable",
			emailEnabled:   true,
			userEmail:      "",
			userRepo:       true,
			emailClient:    &stubEmailSender{},
			wantChannel:    "email",
			wantStatus:     "skipped",
			wantErrorMatch: "recipient email unavailable",
		},
		{
			name:           "email send failure",
			emailEnabled:   true,
			userEmail:      "user@example.com",
			userRepo:       true,
			emailClient:    &stubEmailSender{err: fmt.Errorf("smtp timeout")},
			wantChannel:    "email",
			wantStatus:     "failed",
			wantErrorMatch: "smtp timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newNotificationServiceTestDB(t)
			ctx := context.Background()
			seedNotificationServiceUserSettings(t, db, "user-1", tt.emailEnabled, "immediate", nil, now)
			if tt.userRepo {
				seedNotificationServiceUser(t, db, "user-1", tt.userEmail, "Recipient", now)
			}

			var userRepo *repository.UserRepository
			if tt.userRepo {
				userRepo = repository.NewUserRepository(db)
			}
			var emailClient emailSender
			if tt.emailClient != nil {
				emailClient = tt.emailClient
			}

			service := NewNotificationService(
				repository.NewNotificationRepository(db),
				repository.NewNotificationPreferenceRepository(db),
				repository.NewUserNotificationSettingsRepository(db),
				repository.NewFollowerRepository(db),
				userRepo,
				repository.NewWorkspaceRepository(db),
				nil,
				emailClient,
				"",
			)

			plans, err := service.buildDeliveryPlans(ctx, "user-1", event, "normal", now)
			if err != nil {
				t.Fatalf("buildDeliveryPlans: %v", err)
			}
			if len(plans) != 2 {
				t.Fatalf("plan count = %d, want 2", len(plans))
			}
			if plans[0].Channel != "in_app" || plans[0].Status != "delivered" {
				t.Fatalf("in_app plan = %+v, want delivered in_app", plans[0])
			}
			if plans[1].Channel != tt.wantChannel || plans[1].Status != tt.wantStatus {
				t.Fatalf("email plan = %+v, want channel=%q status=%q", plans[1], tt.wantChannel, tt.wantStatus)
			}
			if plans[1].Error == nil || !strings.Contains(*plans[1].Error, tt.wantErrorMatch) {
				t.Fatalf("email plan error = %v, want substring %q", plans[1].Error, tt.wantErrorMatch)
			}
		})
	}
}

func newNotificationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:notification-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
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
			email_verified_at DATETIME,
			google_subject TEXT,
			avatar_url TEXT,
			avatar_style TEXT,
			avatar_seed TEXT,
			avatar_background_mode TEXT,
			avatar_background_color TEXT,
			default_workspace_id TEXT,
			totp_secret_encrypted TEXT,
			totp_verified BOOLEAN NOT NULL DEFAULT 0,
			recovery_codes_encrypted TEXT,
			is_platform_admin BOOLEAN NOT NULL DEFAULT 0,
			is_server_admin BOOLEAN NOT NULL DEFAULT 0,
			signup_verification_pending BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			workspace_key TEXT,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			description TEXT,
			company_product_context TEXT,
			logo_url TEXT,
			timezone TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE user_notification_settings (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
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
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
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
		`CREATE TABLE entity_followers (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			user_id TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			reason TEXT,
			created_at DATETIME
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
			event_count INTEGER NOT NULL,
			last_event_at DATETIME,
			status TEXT NOT NULL,
			snoozed_until DATETIME,
			read_at DATETIME,
			archived_at DATETIME,
			priority TEXT NOT NULL,
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
		mustExecNotificationService(t, db, stmt)
	}

	return db
}

func seedNotificationServiceUser(t *testing.T, db *gorm.DB, userID, email, fullName string, now time.Time) {
	t.Helper()
	mustExecNotificationService(t, db, `INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, email, "hash", fullName, now, now)
}

func seedNotificationServiceWorkspace(t *testing.T, db *gorm.DB, workspaceID, name string, now time.Time) {
	t.Helper()
	mustExecNotificationService(t, db, `INSERT INTO workspaces (id, name, slug, owner_id, timezone, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workspaceID, name, strings.ToLower(strings.ReplaceAll(name, " ", "-")), "owner-1", "UTC", now, now)
}

func seedNotificationServiceUserSettings(t *testing.T, db *gorm.DB, userID string, emailEnabled bool, digestFrequency string, dndUntil *time.Time, now time.Time) {
	t.Helper()
	mustExecNotificationService(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, dnd_until, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-"+userID, userID, emailEnabled, digestFrequency, "09:00", 1, false, dndUntil, "all", "UTC", now, now)
}

func loadNotificationServiceDeliveries(t *testing.T, db *gorm.DB) []struct {
	Channel string
	Status  string
	Error   *string
} {
	t.Helper()

	var rows []struct {
		Channel string
		Status  string
		Error   *string
	}
	if err := db.Raw(`SELECT channel, status, error FROM notification_deliveries ORDER BY channel ASC, created_at ASC, id ASC`).Scan(&rows).Error; err != nil {
		t.Fatalf("load deliveries: %v", err)
	}
	return rows
}

func assertNotificationServiceCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()

	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s row count = %d, want %d", table, count, want)
	}
}

func mustExecNotificationService(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
