package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupSupportMessageActionsTestEnv(t *testing.T) (*SupportMessageActionsService, *repository.SupportMessageRepository, *repository.SupportEmailLogRepository, *redis.Client, *miniredis.Miniredis) {
	t.Helper()

	db := newTestDB(t)
	rdbServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: rdbServer.Addr()})

	messageRepo := repository.NewSupportMessageRepository(db)
	emailLogRepo := repository.NewSupportEmailLogRepository(db)
	emailFallback := NewEmailFallbackService(
		rdb,
		nil,
		nil,
		nil,
		messageRepo,
		repository.NewSupportConversationRepository(db),
		emailLogRepo,
		repository.NewSupportEmailWebhookEventRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewWorkspaceRepository(db),
		"replies.helpin.ai",
		"https://app.helpin.ai",
		"pod-test",
	)

	svc := NewSupportMessageActionsService(messageRepo, emailFallback, emailLogRepo, nil)
	return svc, messageRepo, emailLogRepo, rdb, rdbServer
}

func createActionMessage(t *testing.T, repo *repository.SupportMessageRepository, msg model.SupportMessage) model.SupportMessage {
	t.Helper()
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	if msg.ID == "" {
		msg.ID = "44444444-4444-4444-4444-444444444444"
	}
	if msg.WorkspaceID == "" {
		msg.WorkspaceID = "11111111-1111-1111-1111-111111111111"
	}
	if msg.ConversationID == "" {
		msg.ConversationID = "33333333-3333-3333-3333-333333333333"
	}
	if msg.SenderType == "" {
		msg.SenderType = "user"
	}
	if msg.MessageType == "" {
		msg.MessageType = "reply"
	}
	if msg.SenderUserID == nil {
		userID := "22222222-2222-2222-2222-222222222222"
		msg.SenderUserID = &userID
	}
	if msg.Content == "" {
		msg.Content = "Original draft"
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = now
	}
	if msg.UpdatedAt.IsZero() {
		msg.UpdatedAt = now
	}
	if err := repo.Create(context.Background(), &msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	return msg
}

func TestSupportMessageActionsDeleteUndoCancelsOutboxAndReturnsMarkdown(t *testing.T) {
	ctx := context.Background()
	svc, messageRepo, _, rdb, rdbServer := setupSupportMessageActionsTestEnv(t)
	defer rdbServer.Close()

	expiresAt := time.Now().UTC().Add(time.Minute)
	msg := createActionMessage(t, messageRepo, model.SupportMessage{
		Content:          "Fix the typo",
		CancellableUntil: &expiresAt,
	})
	if err := rdb.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(expiresAt.Unix()), Member: msg.ConversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := rdb.RPush(ctx, emailFallbackMsgsKeyPrefix+msg.ConversationID, msg.ID).Err(); err != nil {
		t.Fatalf("seed message list: %v", err)
	}

	result, err := svc.Delete(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, true)
	if err != nil {
		t.Fatalf("Delete undo: %v", err)
	}
	if result.Markdown != msg.Content {
		t.Fatalf("markdown = %q, want %q", result.Markdown, msg.Content)
	}
	if result.EmailAlreadySent {
		t.Fatalf("email_already_sent = true, want false")
	}
	if reloaded, err := messageRepo.GetByID(ctx, msg.ID); err != nil {
		t.Fatalf("reload message: %v", err)
	} else if reloaded != nil {
		t.Fatalf("message still visible after soft delete: %#v", reloaded)
	}
	if got, err := rdb.LLen(ctx, emailFallbackMsgsKeyPrefix+msg.ConversationID).Result(); err != nil {
		t.Fatalf("llen: %v", err)
	} else if got != 0 {
		t.Fatalf("queued message count = %d, want 0", got)
	}
}

func TestSupportMessageActionsDeleteUndoRejectsExpiredWindow(t *testing.T) {
	ctx := context.Background()
	svc, messageRepo, _, _, rdbServer := setupSupportMessageActionsTestEnv(t)
	defer rdbServer.Close()

	expiresAt := time.Now().UTC().Add(-time.Second)
	msg := createActionMessage(t, messageRepo, model.SupportMessage{CancellableUntil: &expiresAt})

	_, err := svc.Delete(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, true)
	if !errors.Is(err, ErrCancellableExpired) {
		t.Fatalf("Delete undo error = %v, want ErrCancellableExpired", err)
	}
}

func TestSupportMessageActionsDeleteRejectsNonOwner(t *testing.T) {
	ctx := context.Background()
	svc, messageRepo, _, _, rdbServer := setupSupportMessageActionsTestEnv(t)
	defer rdbServer.Close()

	msg := createActionMessage(t, messageRepo, model.SupportMessage{})

	_, err := svc.Delete(ctx, msg.WorkspaceID, msg.ConversationID, "99999999-9999-9999-9999-999999999999", msg.ID, false)
	if !errors.Is(err, ErrSupportMessageActionForbidden) {
		t.Fatalf("Delete error = %v, want ErrSupportMessageActionForbidden", err)
	}
}

func TestSupportMessageActionsDeleteAfterWindowReportsAlreadySent(t *testing.T) {
	ctx := context.Background()
	svc, messageRepo, emailLogRepo, _, rdbServer := setupSupportMessageActionsTestEnv(t)
	defer rdbServer.Close()

	expiresAt := time.Now().UTC().Add(-time.Minute)
	msg := createActionMessage(t, messageRepo, model.SupportMessage{CancellableUntil: &expiresAt})
	if err := emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "55555555-5555-5555-5555-555555555555",
		WorkspaceID:    msg.WorkspaceID,
		ConversationID: msg.ConversationID,
		Direction:      "outbound",
		MessageIDs:     model.DocsStringArray{msg.ID},
		Status:         "sent",
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create email log: %v", err)
	}

	result, err := svc.Delete(ctx, msg.WorkspaceID, msg.ConversationID, *msg.SenderUserID, msg.ID, false)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !result.EmailAlreadySent {
		t.Fatalf("email_already_sent = false, want true")
	}
	if result.Markdown != "" {
		t.Fatalf("markdown = %q, want empty for non-undo delete", result.Markdown)
	}
}

func TestSupportMessageActionsInfoReturnsDerivedFields(t *testing.T) {
	ctx := context.Background()
	svc, messageRepo, emailLogRepo, _, rdbServer := setupSupportMessageActionsTestEnv(t)
	defer rdbServer.Close()

	actorID := "22222222-2222-2222-2222-222222222222"
	createdAt := time.Date(2026, 4, 30, 12, 34, 0, 0, time.UTC)
	readAt := createdAt.Add(2 * time.Minute)
	avatarURL := "https://example.com/a.png"
	msg := createActionMessage(t, messageRepo, model.SupportMessage{
		SenderUserID:      &actorID,
		SenderDisplayName: strPtr("Agent Smith"),
		SenderAvatarURL:   &avatarURL,
		Content:           "Hello",
		EmailReadAt:       &readAt,
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	})
	deliveredAt := createdAt.Add(time.Minute)
	if err := emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "66666666-6666-6666-6666-666666666666",
		WorkspaceID:    msg.WorkspaceID,
		ConversationID: msg.ConversationID,
		Direction:      "outbound",
		MessageIDs:     model.DocsStringArray{msg.ID},
		FromEmail:      "support@example.com",
		ToEmail:        "customer@example.com",
		Status:         "delivered",
		DeliveredAt:    &deliveredAt,
		CreatedAt:      createdAt,
	}); err != nil {
		t.Fatalf("create email log: %v", err)
	}

	info, err := svc.Info(ctx, msg.WorkspaceID, msg.ConversationID, actorID, msg.ID)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.ID != msg.ID || info.Sender.Name != "Agent Smith" || info.Sender.AvatarURL == nil || *info.Sender.AvatarURL != avatarURL {
		t.Fatalf("unexpected sender info: %#v", info)
	}
	if info.From != "support@example.com" || info.Origin != "chat" || info.Type != "text" {
		t.Fatalf("unexpected origin fields: %#v", info)
	}
	if info.Delivered == nil || info.Delivered.Channel != "email" || !info.Delivered.DeliveredAt.Equal(deliveredAt) {
		t.Fatalf("unexpected delivered info: %#v", info.Delivered)
	}
	if !info.Read || info.ReadAt == nil || !info.ReadAt.Equal(readAt) {
		t.Fatalf("unexpected read info: read=%v readAt=%v", info.Read, info.ReadAt)
	}
	if info.Edited || info.Translated || info.Automated {
		t.Fatalf("v1 flags should be false: %#v", info)
	}
}

func TestSupportMessageActionsInfoDistinguishesEmailDeliveryStates(t *testing.T) {
	ctx := context.Background()
	svc, messageRepo, emailLogRepo, _, rdbServer := setupSupportMessageActionsTestEnv(t)
	defer rdbServer.Close()

	now := time.Date(2026, 4, 30, 12, 34, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	actorID := "22222222-2222-2222-2222-222222222222"

	queuedUntil := now.Add(time.Minute)
	queued := createActionMessage(t, messageRepo, model.SupportMessage{
		ID:               "77777777-7777-7777-7777-777777777777",
		SenderUserID:     &actorID,
		Content:          "Queued reply",
		CancellableUntil: &queuedUntil,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	queuedInfo, err := svc.Info(ctx, queued.WorkspaceID, queued.ConversationID, actorID, queued.ID)
	if err != nil {
		t.Fatalf("queued Info: %v", err)
	}
	if queuedInfo.EmailDeliveryStatus != "queued" || queuedInfo.EmailDeliveryStatusLabel != "Queued for email" {
		t.Fatalf("unexpected queued delivery status: %#v", queuedInfo)
	}
	if queuedInfo.Delivered != nil || queuedInfo.NotDeliveredReason != nil {
		t.Fatalf("queued message should not be delivered or failed: %#v", queuedInfo)
	}

	notifiedAt := now.Add(2 * time.Minute)
	sent := createActionMessage(t, messageRepo, model.SupportMessage{
		ID:              "88888888-8888-8888-8888-888888888888",
		SenderUserID:    &actorID,
		Content:         "Sent reply",
		EmailNotifiedAt: &notifiedAt,
		CreatedAt:       now.Add(time.Second),
		UpdatedAt:       notifiedAt,
	})
	if err := emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "99999999-9999-9999-9999-999999999999",
		WorkspaceID:    sent.WorkspaceID,
		ConversationID: sent.ConversationID,
		Direction:      "outbound",
		MessageIDs:     model.DocsStringArray{sent.ID},
		FromEmail:      "support@example.com",
		ToEmail:        "customer@example.com",
		Status:         "sent",
		CreatedAt:      notifiedAt,
	}); err != nil {
		t.Fatalf("create sent email log: %v", err)
	}
	sentInfo, err := svc.Info(ctx, sent.WorkspaceID, sent.ConversationID, actorID, sent.ID)
	if err != nil {
		t.Fatalf("sent Info: %v", err)
	}
	if sentInfo.EmailDeliveryStatus != "sent" || sentInfo.EmailDeliveryStatusLabel != "Sent via email" {
		t.Fatalf("unexpected sent delivery status: %#v", sentInfo)
	}
	if sentInfo.Delivered != nil || sentInfo.NotDeliveredReason != nil {
		t.Fatalf("accepted-only message should not be delivered or failed: %#v", sentInfo)
	}
}
