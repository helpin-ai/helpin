package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestNotificationEmailVolume(t *testing.T) {
	for _, tc := range []struct {
		name, eventType, frequency string
		repeated                   bool
		emails, queued             int
	}{
		{"high CRM stays in digest", "crm.signal_ready", "daily", false, 0, 10},
		{"repeat mentions grouped", "comment.mention", "daily", true, 1, 9},
		{"mentions across items capped", "comment.mention", "daily", false, 5, 5},
		{"explicit immediate still capped", "comment.created", "immediate", false, 5, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newNotificationServiceTestDB(t)
			now := time.Now().UTC()
			seedNotificationServiceUser(t, db, "user", "user@example.com", "Recipient", now)
			seedNotificationServiceUserSettings(t, db, "user", true, tc.frequency, nil, now)
			mailer := &stubEmailSender{}
			svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, repository.NewUserRepository(db), nil, nil, mailer, "")
			for i := 0; i < 10; i++ {
				id := fmt.Sprintf("item-%d", i)
				if tc.repeated {
					id = "item"
				}
				err := svc.Emit(context.Background(), model.NotificationEventInput{WorkspaceID: "ws", EntityType: "task", EntityID: id, EventType: tc.eventType, Title: "Update", Priority: "high", ExplicitRecipients: []string{"user"}, SkipFollowers: true})
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(mailer.sent) != tc.emails {
				t.Fatalf("sent %d emails, want %d", len(mailer.sent), tc.emails)
			}
			var pending int64
			db.Table("notification_deliveries").Where("status = ? AND channel IN ?", "pending", []string{"digest", "email_overflow"}).Count(&pending)
			if int(pending) != tc.queued {
				t.Fatalf("queued %d, want %d", pending, tc.queued)
			}
		})
	}
}

func TestNotificationOverflowDigestHonorsReadAndFrequency(t *testing.T) {
	for _, frequency := range []string{"daily", "immediate", "never"} {
		t.Run(frequency, func(t *testing.T) {
			db := newNotificationServiceTestDB(t)
			now := time.Now().UTC()
			ctx := context.Background()
			seedNotificationServiceUser(t, db, "user", "user@example.com", "Recipient", now)
			seedNotificationServiceUserSettings(t, db, "user", true, frequency, nil, now)
			mailer := &stubEmailSender{}
			svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, repository.NewUserRepository(db), nil, nil, mailer, "")
			for i := 0; i < 8; i++ {
				if err := svc.Emit(ctx, model.NotificationEventInput{WorkspaceID: fmt.Sprintf("ws-%d", i%2), EntityType: "task", EntityID: fmt.Sprintf("task-%d", i), EventType: "comment.mention", Title: fmt.Sprintf("Mention %d", i), Priority: "high", ExplicitRecipients: []string{"user"}, SkipFollowers: true}); err != nil {
					t.Fatal(err)
				}
			}
			if len(mailer.sent) != 5 {
				t.Fatalf("got %d individual emails, want 5 across workspaces", len(mailer.sent))
			}
			mustExecNotificationService(t, db, "UPDATE notifications SET status='read' WHERE entity_id='task-7'")
			due := now.Add(24 * time.Hour)
			if err := svc.ProcessPendingDigests(ctx, due); err != nil {
				t.Fatal(err)
			}
			if len(mailer.sent) != 6 {
				t.Fatalf("got %d total emails, want 5 plus one digest", len(mailer.sent))
			}
			if strings.Contains(mailer.sent[5].textBody, "Mention 7") {
				t.Fatal("digest includes handled mention")
			}
			if !strings.Contains(mailer.sent[5].textBody, "Mention 5") || !strings.Contains(mailer.sent[5].textBody, "Mention 6") {
				t.Fatalf("missing overflow: %s", mailer.sent[5].textBody)
			}
			if err := svc.ProcessPendingDigests(ctx, due); err != nil {
				t.Fatal(err)
			}
			if len(mailer.sent) != 6 {
				t.Fatal("repeated worker sent duplicate digest")
			}
		})
	}
}

func TestNotificationSupportEmailsShareVolumeLimit(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedNotificationServiceUser(t, db, "user", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user", true, "daily", nil, now)
	mailer := &stubEmailSender{}
	svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, repository.NewUserRepository(db), nil, nil, mailer, "")
	for i := 0; i < 6; i++ {
		if err := svc.Emit(ctx, model.NotificationEventInput{WorkspaceID: "ws", EntityType: "support_conversation", EntityID: fmt.Sprintf("conv-%d", i), EventType: "support_conversation.customer_reply", Title: "Customer reply", Priority: "high", DelayedEmailChannel: "support_reply_email", ExplicitRecipients: []string{"user"}, SkipFollowers: true}); err != nil {
			t.Fatal(err)
		}
	}
	due := now.Add(4 * time.Minute)
	if err := svc.ProcessPendingSupportReplyEmails(ctx, due); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 5 {
		t.Fatalf("got %d emails, want 5", len(mailer.sent))
	}
	pending, err := svc.notifRepo.ListPendingDigestDeliveries(ctx)
	if err != nil || len(pending) != 1 || pending[0].Channel != "email_overflow" {
		t.Fatalf("missing overflow: %+v %v", pending, err)
	}
	if err := svc.ProcessPendingSupportReplyEmails(ctx, due); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 5 {
		t.Fatal("duplicate support emails")
	}
}

func TestNotificationEmailCooldownExpires(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedNotificationServiceUser(t, db, "user", "user@example.com", "Recipient", now)
	seedNotificationServiceUserSettings(t, db, "user", true, "daily", nil, now)
	mailer := &stubEmailSender{}
	svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, repository.NewUserRepository(db), nil, nil, mailer, "")
	event := model.NotificationEventInput{WorkspaceID: "ws", EntityType: "task", EntityID: "task", EventType: "comment.mention", Title: "Mention", ExplicitRecipients: []string{"user"}, SkipFollowers: true}
	if err := svc.Emit(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := svc.Emit(ctx, event); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("repeat sent %d emails", len(mailer.sent))
	}
	mustExecNotificationService(t, db, "UPDATE notification_deliveries SET delivered_at=? WHERE channel='email'", now.Add(-16*time.Minute))
	if err := svc.Emit(ctx, event); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 2 {
		t.Fatal("new mention did not send after cooldown")
	}
}

func TestNotificationOverflowRespectsNewDirectOnlyPreference(t *testing.T) {
	db := newNotificationServiceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seedNotificationServiceUserSettings(t, db, "user", true, "never", nil, now)
	svc := NewNotificationService(repository.NewNotificationRepository(db), repository.NewNotificationPreferenceRepository(db), repository.NewUserNotificationSettingsRepository(db), nil, nil, nil, nil, nil, "")
	kept, skipped, err := svc.filterDigestDeliveriesByCurrentPreferences(ctx, "user", []repository.PendingDigestDelivery{{DeliveryID: "routine", WorkspaceID: "ws", EventType: "comment.created", Channel: "email_overflow"}, {DeliveryID: "direct", WorkspaceID: "ws", EventType: "comment.mention", Channel: "email_overflow"}})
	if err != nil || len(kept) != 1 || kept[0].DeliveryID != "direct" || len(skipped) != 1 || skipped[0] != "routine" {
		t.Fatalf("preferences ignored: %+v %+v %v", kept, skipped, err)
	}
}
