package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"html"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/email/inboundhtml"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type capturedPostmarkRequest struct {
	From       string `json:"From"`
	To         string `json:"To"`
	Subject    string `json:"Subject"`
	HtmlBody   string `json:"HtmlBody"`
	TextBody   string `json:"TextBody"`
	ReplyTo    string `json:"ReplyTo"`
	TrackOpens bool   `json:"TrackOpens"`
	Headers    []struct {
		Name  string `json:"Name"`
		Value string `json:"Value"`
	} `json:"Headers"`
}

type emailFallbackTestEnv struct {
	redis        *redis.Client
	redisServer  *miniredis.Miniredis
	service      *EmailFallbackService
	routeRepo    *repository.SupportEmailRouteRepository
	senderRepo   *repository.SupportEmailSenderRepository
	messageRepo  *repository.SupportMessageRepository
	convRepo     *repository.SupportConversationRepository
	emailLogRepo *repository.SupportEmailLogRepository
	webhookRepo  *repository.SupportEmailWebhookEventRepository
	installRepo  *repository.SupportInboxInstallationRepository
	sessionRepo  *repository.SupportInboxSessionRepository
}

func setupEmailFallbackTestEnv(t *testing.T, settings model.SupportInboxSettings) *emailFallbackTestEnv {
	t.Helper()
	return setupEmailFallbackTestEnvWithRedis(t, settings, true)
}

func setupEmailFallbackInboundTestEnv(t *testing.T, settings model.SupportInboxSettings) *emailFallbackTestEnv {
	t.Helper()
	return setupEmailFallbackTestEnvWithRedis(t, settings, false)
}

func setupEmailFallbackTestEnvWithRedis(t *testing.T, settings model.SupportInboxSettings, withRedis bool) *emailFallbackTestEnv {
	t.Helper()

	db := newTestDB(t)
	if err := db.AutoMigrate(&model.SupportInboundJob{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE support_attachments (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, message_id TEXT, file_name TEXT, file_size INTEGER, content_type TEXT, storage_key TEXT DEFAULT '', public_url TEXT DEFAULT '', is_uploaded BOOLEAN DEFAULT false, uploaded_by_type TEXT, uploaded_by_id TEXT, session_id TEXT, created_at DATETIME, processing_status TEXT NOT NULL DEFAULT '', processing_error TEXT NOT NULL DEFAULT '', content_id TEXT NOT NULL DEFAULT '')`).Error; err != nil {
		t.Fatal(err)
	}
	var redisServer *miniredis.Miniredis
	var redisClient *redis.Client
	if withRedis {
		redisServer = miniredis.RunT(t)
		redisClient = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	}

	workspaceID := "11111111-1111-1111-1111-111111111111"
	ownerID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, ownerID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", ownerID)

	messageRepo := repository.NewSupportMessageRepository(db)
	convRepo := repository.NewSupportConversationRepository(db)
	routeRepo := repository.NewSupportEmailRouteRepository(db)
	senderRepo := repository.NewSupportEmailSenderRepository(db)
	emailLogRepo := repository.NewSupportEmailLogRepository(db)
	webhookRepo := repository.NewSupportEmailWebhookEventRepository(db)
	installRepo := repository.NewSupportInboxInstallationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := installRepo.Create(context.Background(), &model.SupportWidgetInstallation{
		WorkspaceID: workspaceID,
		WidgetKey:   "widget-key",
		SecretKey:   "secret-key",
		Settings:    string(settingsJSON),
		Active:      true,
	}); err != nil {
		t.Fatalf("create installation: %v", err)
	}

	emailClient := email.NewClient("postmark-token", "noreply@example.com")
	service := NewEmailFallbackService(
		redisClient,
		websocket.NewHub(),
		nil,
		emailClient,
		messageRepo,
		convRepo,
		emailLogRepo,
		webhookRepo,
		installRepo,
		sessionRepo,
		workspaceRepo,
		"replies.helpin.ai",
		"https://app.helpin.ai",
		"pod-test",
	)
	supportInboxService := NewSupportInboxService(convRepo, repository.NewSupportMailboxRepository(db), messageRepo, nil, nil, installRepo, sessionRepo, nil, nil, nil, repository.NewCRMContactRepository(db), nil, nil, nil, nil)
	supportInboxService.SetEmailRouteRepository(routeRepo)
	supportInboxService.SetEmailSenderRepository(senderRepo)
	supportInboxService.SetWorkspaceRepo(workspaceRepo)
	supportInboxService.SetEmailLogRepo(emailLogRepo)
	supportInboxService.SetRouteDomain("on.helpin.email")
	service.SetSupportInboxService(supportInboxService)

	return &emailFallbackTestEnv{
		redis:        redisClient,
		redisServer:  redisServer,
		service:      service,
		routeRepo:    routeRepo,
		senderRepo:   senderRepo,
		messageRepo:  messageRepo,
		convRepo:     convRepo,
		emailLogRepo: emailLogRepo,
		webhookRepo:  webhookRepo,
		installRepo:  installRepo,
		sessionRepo:  sessionRepo,
	}
}

func TestEmailFallbackOnAgentReplyEnqueuesAndDebounces(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 45
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	env.service.now = func() time.Time { return now }

	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            "33333333-3333-3333-3333-333333333333",
		WorkspaceID:   "11111111-1111-1111-1111-111111111111",
		Subject:       "Need help",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	msg1 := &model.SupportMessage{ID: "44444444-4444-4444-4444-444444444444"}
	msg2 := &model.SupportMessage{ID: "55555555-5555-5555-5555-555555555555"}

	if err := env.service.OnAgentReply(ctx, conv.WorkspaceID, msg1, conv); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	score, err := env.redis.ZScore(ctx, emailFallbackOutboxKey, conv.ID).Result()
	if err != nil {
		t.Fatalf("get zscore: %v", err)
	}
	if got, want := int64(score), now.Add(45*time.Second).Unix(); got != want {
		t.Fatalf("expected first fire time %d, got %d", want, got)
	}

	now = now.Add(30 * time.Second)
	if err := env.service.OnAgentReply(ctx, conv.WorkspaceID, msg2, conv); err != nil {
		t.Fatalf("second enqueue: %v", err)
	}

	score, err = env.redis.ZScore(ctx, emailFallbackOutboxKey, conv.ID).Result()
	if err != nil {
		t.Fatalf("get debounced zscore: %v", err)
	}
	if got, want := int64(score), now.Add(45*time.Second).Unix(); got != want {
		t.Fatalf("expected debounced fire time %d, got %d", want, got)
	}

	msgIDs, err := env.redis.LRange(ctx, env.service.msgListKey(conv.ID), 0, -1).Result()
	if err != nil {
		t.Fatalf("load queued message ids: %v", err)
	}
	if len(msgIDs) != 2 || msgIDs[0] != msg1.ID || msgIDs[1] != msg2.ID {
		t.Fatalf("unexpected queued message ids: %#v", msgIDs)
	}
}

func TestEmailFallbackOnAgentReplySetsCancellableUntil(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 45
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	env.service.now = func() time.Time { return now }

	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            "33333333-3333-3333-3333-333333333333",
		WorkspaceID:   "11111111-1111-1111-1111-111111111111",
		Subject:       "Need help",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	msg := &model.SupportMessage{
		ID:             "44444444-4444-4444-4444-444444444444",
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: conv.ID,
		SenderType:     "user",
		MessageType:    "reply",
		Content:        "Hello",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := env.service.OnAgentReply(ctx, conv.WorkspaceID, msg, conv); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	reloaded, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	want := now.Add(45 * time.Second)
	if reloaded == nil || reloaded.CancellableUntil == nil || !reloaded.CancellableUntil.UTC().Equal(want) {
		t.Fatalf("cancellable_until = %v, want %v", reloaded.CancellableUntil, want)
	}
}

func TestEmailFallbackOnAgentReplySkipsQueueWhenVisitorOnline(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 30
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	env.service.now = func() time.Time { return now }

	customerEmail := "customer@example.com"
	anonymousID := "anon-online-at-reply"
	conv := &model.SupportConversation{
		ID:            "33333333-3333-3333-3333-333333333334",
		WorkspaceID:   "11111111-1111-1111-1111-111111111111",
		Subject:       "Need help",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
	}
	msg := &model.SupportMessage{
		ID:             "44444444-4444-4444-4444-444444444445",
		WorkspaceID:    conv.WorkspaceID,
		ConversationID: conv.ID,
		SenderType:     "user",
		MessageType:    "reply",
		Content:        "Hello while you are here",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := env.service.hub.Presence.SetVisitorOnline(ctx, conv.WorkspaceID, anonymousID, "conn-1"); err != nil {
		t.Fatalf("set visitor online: %v", err)
	}

	if err := env.service.OnAgentReply(ctx, conv.WorkspaceID, msg, conv); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := env.redis.ZScore(ctx, emailFallbackOutboxKey, conv.ID).Result(); err != redis.Nil {
		t.Fatalf("expected no queued email fallback for online visitor, got err=%v", err)
	}
	reloaded, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if reloaded != nil && reloaded.CancellableUntil != nil {
		t.Fatalf("expected no cancellable_until for online visitor, got %v", reloaded.CancellableUntil)
	}
}

func TestEmailFallbackCancelForMessageRemovesOnlyTargetWhenOthersPending(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "33333333-3333-3333-3333-333333333333"
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: 100, Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), "msg-1", "msg-2", "msg-3").Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}

	alreadySent, err := env.service.CancelForMessage(ctx, workspaceID, conversationID, "msg-2")
	if err != nil {
		t.Fatalf("CancelForMessage: %v", err)
	}
	if alreadySent {
		t.Fatal("expected alreadySent=false")
	}

	msgIDs, err := env.redis.LRange(ctx, env.service.msgListKey(conversationID), 0, -1).Result()
	if err != nil {
		t.Fatalf("load msg list: %v", err)
	}
	if got, want := strings.Join(msgIDs, ","), "msg-1,msg-3"; got != want {
		t.Fatalf("queued message ids = %q, want %q", got, want)
	}
	if _, err := env.redis.ZScore(ctx, emailFallbackOutboxKey, conversationID).Result(); err != nil {
		t.Fatalf("conversation outbox entry should remain: %v", err)
	}
}

func TestEmailFallbackCancelForMessageRemovesConversationWhenLastMessageRemoved(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "33333333-3333-3333-3333-333333333333"
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: 100, Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), "msg-1").Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}

	alreadySent, err := env.service.CancelForMessage(ctx, workspaceID, conversationID, "msg-1")
	if err != nil {
		t.Fatalf("CancelForMessage: %v", err)
	}
	if alreadySent {
		t.Fatal("expected alreadySent=false")
	}
	if exists, err := env.redis.Exists(ctx, emailFallbackOutboxKey, env.service.msgListKey(conversationID)).Result(); err != nil {
		t.Fatalf("check redis keys: %v", err)
	} else if exists != 0 {
		t.Fatalf("expected outbox and msg list removed, found %d keys", exists)
	}
}

func TestEmailFallbackCancelForMessageReturnsAlreadySentWhenEmailLogExists(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "33333333-3333-3333-3333-333333333333"
	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "77777777-7777-7777-7777-777777777777",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		Direction:      "outbound",
		MessageIDs:     model.DocsStringArray{"msg-1"},
		RFCMessageID:   "<sent@example.com>",
		Status:         "bounced",
	}); err != nil {
		t.Fatalf("seed email log: %v", err)
	}

	alreadySent, err := env.service.CancelForMessage(ctx, workspaceID, conversationID, "msg-1")
	if err != nil {
		t.Fatalf("CancelForMessage: %v", err)
	}
	if !alreadySent {
		t.Fatal("expected alreadySent=true")
	}
}

func TestEmailFallbackFireEmailMarksMessagesAndLogs(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackFromName = "Acme Support"
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666666"
	anonymousID := "anon-123"
	customerEmail := "customer@example.com"
	customerName := "Taylor"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		DisplayID:     1234,
		Subject:       "Pricing question",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	pageURL := "https://example.com/pricing"
	if err := env.sessionRepo.Create(ctx, &model.SupportWidgetSession{
		WorkspaceID:    workspaceID,
		ConversationID: &conversationID,
		SessionToken:   "session-token",
		AnonymousID:    anonymousID,
		IsAnonymous:    false,
		CustomerName:   &customerName,
		CustomerEmail:  &customerEmail,
		LastPageURL:    &pageURL,
		ExpiresAt:      fixedNow.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	msg1 := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("Alex Agent"),
		Content:           "Thanks for reaching out.",
		MessageType:       "reply",
	}
	msg2 := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("Alex Agent"),
		Content:           "We can help with annual billing.",
		MessageType:       "reply",
	}
	if err := env.messageRepo.Create(ctx, msg1); err != nil {
		t.Fatalf("create msg1: %v", err)
	}
	if err := env.messageRepo.Create(ctx, msg2); err != nil {
		t.Fatalf("create msg2: %v", err)
	}

	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "77777777-7777-7777-7777-777777777777",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		Direction:      "outbound",
		MessageIDs:     model.DocsStringArray{msg1.ID},
		RFCMessageID:   "<prior@replies.helpin.ai>",
		Status:         "sent",
	}); err != nil {
		t.Fatalf("seed prior email log: %v", err)
	}

	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(fixedNow.Unix()), Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), msg1.ID, msg2.ID).Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}

	var captured capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-out-1",
					"SubmittedAt": "2026-03-20T12:30:00Z",
					"To": "customer@example.com"
				}`)),
			}, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg1.ID, msg2.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}

	if captured.To != customerEmail {
		t.Fatalf("expected recipient %q, got %q", customerEmail, captured.To)
	}
	expectedReplyTo := `"Acme Support" <conv-` + conversationID + `@replies.helpin.ai>`
	if captured.ReplyTo != expectedReplyTo {
		t.Fatalf("unexpected reply-to: %q", captured.ReplyTo)
	}
	if captured.Subject != "Re: Pricing question (#1)" {
		t.Fatalf("unexpected subject: %q", captured.Subject)
	}
	if !strings.Contains(captured.From, "Alex Agent - Acme Support <inbox@acme.on.helpin.email>") {
		t.Fatalf("unexpected from: %q", captured.From)
	}
	if !strings.Contains(captured.HtmlBody, "#helpin-conv="+conversationID) {
		t.Fatalf("expected html body deep link, got %q", captured.HtmlBody)
	}
	if !strings.Contains(captured.TextBody, "annual billing") {
		t.Fatalf("expected text body to include batched content, got %q", captured.TextBody)
	}
	if !captured.TrackOpens {
		t.Fatal("expected support fallback emails to enable TrackOpens")
	}

	headerMap := make(map[string]string, len(captured.Headers))
	for _, header := range captured.Headers {
		headerMap[header.Name] = header.Value
	}
	if headerMap["In-Reply-To"] != "<prior@replies.helpin.ai>" {
		t.Fatalf("unexpected In-Reply-To: %q", headerMap["In-Reply-To"])
	}
	if headerMap["References"] != "<prior@replies.helpin.ai>" {
		t.Fatalf("unexpected References: %q", headerMap["References"])
	}
	if headerMap["X-Conversation-ID"] != conversationID {
		t.Fatalf("unexpected X-Conversation-ID: %q", headerMap["X-Conversation-ID"])
	}
	if headerMap["Message-ID"] == "" {
		t.Fatal("expected generated Message-ID header")
	}
	if headerMap["List-Unsubscribe"] != "" {
		t.Fatalf("support reply emails should not include List-Unsubscribe, got %q", headerMap["List-Unsubscribe"])
	}

	savedMessages, err := env.messageRepo.GetByIDs(ctx, []string{msg1.ID, msg2.ID})
	if err != nil {
		t.Fatalf("reload messages: %v", err)
	}
	for _, msg := range savedMessages {
		if msg.EmailNotifiedAt == nil {
			t.Fatalf("expected email_notified_at to be set for message %s", msg.ID)
		}
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 email logs, got %d", len(logs))
	}
	lastLog := logs[len(logs)-1]
	if lastLog.Direction != "outbound" {
		t.Fatalf("expected outbound log, got %q", lastLog.Direction)
	}
	if lastLog.PostmarkMessageID == nil || *lastLog.PostmarkMessageID != "pm-out-1" {
		t.Fatalf("unexpected postmark message id: %#v", lastLog.PostmarkMessageID)
	}
	if len(lastLog.MessageIDs) != 2 {
		t.Fatalf("expected batched message ids, got %#v", lastLog.MessageIDs)
	}

	if exists, err := env.redis.Exists(ctx, emailFallbackOutboxKey, env.service.msgListKey(conversationID)).Result(); err != nil {
		t.Fatalf("check redis cleanup: %v", err)
	} else if exists != 0 {
		t.Fatalf("expected redis cleanup, found %d keys", exists)
	}
}

// TestReplyByEmailWhenVisitorOffline verifies the after-hours promise from
// Task 10: when a human teammate replies and the visitor is not currently
// connected to the widget, the reply goes out by email exactly once. This is
// the inverse of TestEmailFallbackFireEmailVisitorOnlineUnreadPostponesWithinGraceWindow,
// which proves an online visitor is NOT emailed.
func TestReplyByEmailWhenVisitorOffline(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666672"
	anonymousID := "anon-offline-visitor"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "After hours question",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("Sam Teammate"),
		Content:           "Sorry we missed you — here is the answer.",
		MessageType:       "reply",
		CreatedAt:         fixedNow.Add(-120 * time.Second),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(fixedNow.Unix()), Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), msg.ID).Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}

	// Intentionally do NOT mark the visitor online — they are offline.
	sendCount := 0
	var captured capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				return nil, err
			}
			sendCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-offline-1",
					"SubmittedAt": "2026-03-20T12:30:00Z",
					"To": "customer@example.com"
				}`)),
			}, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}

	if sendCount != 1 {
		t.Fatalf("expected exactly one email send to offline visitor, got %d", sendCount)
	}
	if captured.To != customerEmail {
		t.Fatalf("expected recipient %q, got %q", customerEmail, captured.To)
	}

	saved, err := env.messageRepo.GetByIDs(ctx, []string{msg.ID})
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if len(saved) != 1 || saved[0].EmailNotifiedAt == nil {
		t.Fatalf("expected email_notified_at to be set after offline send")
	}

	// The conversation should be removed from the outbox (sent, not postponed).
	if exists, err := env.redis.Exists(ctx, emailFallbackOutboxKey, env.service.msgListKey(conversationID)).Result(); err != nil {
		t.Fatalf("check redis cleanup: %v", err)
	} else if exists != 0 {
		t.Fatalf("expected redis cleanup after offline send, found %d keys", exists)
	}
}

func TestSupportRouteReplyToFormatsDisplayName(t *testing.T) {
	tests := []struct {
		name           string
		inboundAddress string
		displayName    string
		want           string
	}{
		{
			name:           "with display name",
			inboundAddress: "inbox@acme.on.helpin.email",
			displayName:    "Acme Support",
			want:           `"Acme Support" <inbox@acme.on.helpin.email>`,
		},
		{
			name:           "without display name returns bare address",
			inboundAddress: "inbox@acme.on.helpin.email",
			displayName:    "  ",
			want:           "inbox@acme.on.helpin.email",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportRouteReplyTo(tt.inboundAddress, tt.displayName); got != tt.want {
				t.Errorf("supportRouteReplyTo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveConversationReplyToPrefersRouteAddress(t *testing.T) {
	env := setupEmailFallbackInboundTestEnv(t, model.SupportInboxSettings{})

	conv := &model.SupportConversation{
		ID:          "44444444-4444-4444-4444-444444444444",
		WorkspaceID: "11111111-1111-1111-1111-111111111111",
		Status:      model.SupportConversationStatusOpen,
	}
	if err := env.convRepo.Create(context.Background(), conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	gotLegacy := env.service.resolveConversationReplyTo(context.Background(), conv, "Acme Support")
	wantLegacy := `"Acme Support" <conv-` + conv.ID + `@replies.helpin.ai>`
	if gotLegacy != wantLegacy {
		t.Fatalf("legacy reply-to = %q, want %q", gotLegacy, wantLegacy)
	}

	if err := env.routeRepo.Create(context.Background(), &model.SupportEmailRoute{
		ID:             "55555555-5555-5555-5555-555555555555",
		WorkspaceID:    conv.WorkspaceID,
		RouteKey:       "route-44444444",
		InboundAddress: "inbox@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
	}); err != nil {
		t.Fatalf("create route: %v", err)
	}

	got := env.service.resolveConversationReplyTo(context.Background(), conv, "Acme Support")
	want := `"Acme Support" <inbox@acme.on.helpin.email>`
	if got != want {
		t.Errorf("route reply-to = %q, want %q", got, want)
	}
}

func TestEmailFallbackFireEmailRetriesVerifiedSenderWhenBrandedSenderRejected(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackFromName = "Acme Support"
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666667"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Pricing question",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("Alex Agent"),
		Content:           "We can help with billing.",
		MessageType:       "reply",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	var attempts []capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			var captured capturedPostmarkRequest
			if err := json.Unmarshal(body, &captured); err != nil {
				return nil, err
			}
			attempts = append(attempts, captured)
			if len(attempts) == 1 {
				return &http.Response{
					StatusCode: http.StatusUnprocessableEntity,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"ErrorCode": 300,
						"Message": "The 'From' address you supplied (inbox@acme.on.helpin.email) is not a Sender Signature on your account."
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-fallback-1",
					"SubmittedAt": "2026-03-20T12:30:00Z",
					"To": "customer@example.com"
				}`)),
			}, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}

	if len(attempts) != 2 {
		t.Fatalf("expected branded sender attempt and verified fallback attempt, got %d", len(attempts))
	}
	if !strings.Contains(attempts[0].From, "Alex Agent - Acme Support <inbox@acme.on.helpin.email>") {
		t.Fatalf("unexpected first from: %q", attempts[0].From)
	}
	if !strings.Contains(attempts[1].From, "Alex Agent - Acme Support <noreply@example.com>") {
		t.Fatalf("unexpected fallback from: %q", attempts[1].From)
	}
	expectedReplyTo := `"Acme Support" <conv-` + conversationID + `@replies.helpin.ai>`
	if attempts[0].ReplyTo != expectedReplyTo || attempts[1].ReplyTo != expectedReplyTo {
		t.Fatalf("reply-to changed across retry: first=%q second=%q", attempts[0].ReplyTo, attempts[1].ReplyTo)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one outbound log, got %d", len(logs))
	}
	if logs[0].FromEmail != "noreply@example.com" {
		t.Fatalf("expected fallback from in email log, got %q", logs[0].FromEmail)
	}
	if logs[0].FromSource != "verified_fallback_sender" {
		t.Fatalf("expected verified fallback sender source in email log, got %q", logs[0].FromSource)
	}
	if logs[0].FromFallbackReason != "postmark_sender_signature_rejected" {
		t.Fatalf("expected postmark fallback reason in email log, got %q", logs[0].FromFallbackReason)
	}
	if logs[0].PostmarkMessageID == nil || *logs[0].PostmarkMessageID != "pm-fallback-1" {
		t.Fatalf("unexpected postmark message id: %#v", logs[0].PostmarkMessageID)
	}
}

func TestEmailFallbackFireEmailUsesMailboxDefaultSenderAndLogsReplyContract(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackFromName = "Acme Support"
	env := setupEmailFallbackTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "44444444-4444-4444-4444-444444444444"
	mailboxID := "33333333-3333-3333-3333-333333333333"

	mailboxRepo := repository.NewSupportMailboxRepository(env.convRepo.DB())
	if err := mailboxRepo.Create(ctx, &model.SupportMailbox{
		ID:          mailboxID,
		WorkspaceID: workspaceID,
		Name:        "Billing",
		Handle:      "billing",
		Icon:        "Inbox",
		Active:      true,
		Position:    1,
		CreatedByID: "22222222-2222-2222-2222-222222222222",
	}); err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	if err := env.senderRepo.Create(ctx, &model.SupportEmailSender{
		ID:                       "55555555-5555-5555-5555-555555555555",
		WorkspaceID:              workspaceID,
		MailboxID:                &mailboxID,
		Email:                    "billing@acme.test",
		LocalPart:                "billing",
		Domain:                   "acme.test",
		DisplayName:              "Acme Billing",
		ReturnPathDomainVerified: true,
		DKIMVerified:             true,
		DomainStatus:             "verified",
		ForwardingStatus:         "verified",
		VerificationStatus:       "verified",
		DefaultScope:             "mailbox",
		Active:                   true,
		CreatedByID:              "22222222-2222-2222-2222-222222222222",
	}); err != nil {
		t.Fatalf("create sender: %v", err)
	}
	if err := env.routeRepo.Create(ctx, &model.SupportEmailRoute{
		ID:             "77777777-7777-7777-7777-777777777777",
		WorkspaceID:    workspaceID,
		MailboxID:      &mailboxID,
		RouteKey:       "route-mailbox-billing",
		InboundAddress: "inbox@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
	}); err != nil {
		t.Fatalf("create route: %v", err)
	}

	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		MailboxID:     &mailboxID,
		Subject:       "Billing question",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	msg := &model.SupportMessage{
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("Arooj"),
		Content:           "I checked your invoice.",
		MessageType:       "reply",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	var captured capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-mailbox-1",
					"SubmittedAt": "2026-03-20T12:30:00Z",
					"To": "customer@example.com"
				}`)),
			}, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}

	expectedReplyTo := `"Acme Support" <inbox@acme.on.helpin.email>`
	if captured.From != "Arooj - Acme Support <billing@acme.test>" {
		t.Fatalf("from = %q, want mailbox sender with agent and workspace display", captured.From)
	}
	if captured.ReplyTo != expectedReplyTo {
		t.Fatalf("reply-to = %q, want %q", captured.ReplyTo, expectedReplyTo)
	}
	if strings.Contains(captured.TextBody, "app.helpin.ai#helpin-conv") || strings.Contains(captured.HtmlBody, "app.helpin.ai#helpin-conv") {
		t.Fatalf("email body should not link customers to the Helpin app fallback, text=%q html=%q", captured.TextBody, captured.HtmlBody)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one outbound log, got %d", len(logs))
	}
	if logs[0].FromEmail != "billing@acme.test" {
		t.Fatalf("log from email = %q, want billing@acme.test", logs[0].FromEmail)
	}
	if logs[0].FromDisplayName != "Arooj - Acme Support" {
		t.Fatalf("log from display name = %q", logs[0].FromDisplayName)
	}
	if logs[0].ReplyTo != expectedReplyTo {
		t.Fatalf("log reply-to = %q, want %q", logs[0].ReplyTo, expectedReplyTo)
	}
	if logs[0].FromSource != "mailbox_default_sender" {
		t.Fatalf("log from source = %q, want mailbox_default_sender", logs[0].FromSource)
	}
}

func TestEmailFallbackRenderBodiesIncludesMessageAttachments(t *testing.T) {
	svc := &EmailFallbackService{}

	htmlBody, textBody := svc.renderBodies(
		[]model.SupportMessage{
			{
				Content: "Here is the report.",
				Attachments: []model.SupportAttachmentPayload{
					{
						FileName: "billing report.pdf",
						URL:      "https://cdn.example.com/billing-report.pdf",
					},
				},
			},
		},
		"Alex Agent",
		"Acme Support",
		"",
		"",
	)

	for _, body := range []string{htmlBody, textBody} {
		if !strings.Contains(body, "billing report.pdf") {
			t.Fatalf("expected rendered email body to include attachment name, got %q", body)
		}
		if !strings.Contains(body, "https://cdn.example.com/billing-report.pdf") {
			t.Fatalf("expected rendered email body to include attachment URL, got %q", body)
		}
	}
}

func TestEmailFallbackRenderBodiesAddsTrackedPoweredByFooter(t *testing.T) {
	svc := &EmailFallbackService{}

	htmlBody, textBody := svc.renderBodies(
		[]model.SupportMessage{{WorkspaceID: "7d314f04-d25e-461e-8ce4-49de0527ae32", Content: "Thanks."}},
		"Alex Agent",
		"Replug",
		"",
		"",
	)
	wantURL := "https://helpin.ai/?utm_source=replug-7d314f04&utm_medium=email&utm_campaign=powered_by_helpin&utm_content=support_email_footer"
	wantHTMLURL := html.EscapeString(wantURL)

	if !strings.Contains(htmlBody, "<strong>Helpin AI</strong>") {
		t.Fatalf("html footer should bold Helpin AI, got %q", htmlBody)
	}
	if !strings.Contains(htmlBody, wantHTMLURL) {
		t.Fatalf("html footer missing tracked URL, got %q", htmlBody)
	}
	if strings.Contains(htmlBody, "&amp;amp;") {
		t.Fatalf("html footer URL should be escaped exactly once, got %q", htmlBody)
	}
	if !strings.Contains(textBody, wantURL) || strings.Contains(textBody, "&amp;") {
		t.Fatalf("text footer missing tracked URL, got %q", textBody)
	}
}

func TestEmailFallbackRenderBodiesAddsPreviewBeforeReplyDelimiter(t *testing.T) {
	svc := &EmailFallbackService{}

	htmlBody, textBody := svc.renderBodies(
		[]model.SupportMessage{{Content: "Thanks for reaching out about billing.\nWe can help."}},
		"Alex Agent",
		"Acme Support",
		"",
		"",
	)

	preheaderIndex := strings.Index(htmlBody, "Thanks for reaching out about billing. We can help.")
	delimiterIndex := strings.Index(htmlBody, supportEmailReplyDelimiter)
	messageIndex := strings.Index(htmlBody, "<p>Thanks for reaching out about billing.")
	if preheaderIndex < 0 {
		t.Fatalf("expected hidden preheader with message preview, got %q", htmlBody)
	}
	if delimiterIndex < 0 {
		t.Fatalf("expected reply delimiter, got %q", htmlBody)
	}
	if !(preheaderIndex < delimiterIndex && delimiterIndex < messageIndex) {
		t.Fatalf("expected preheader before delimiter before message, got %q", htmlBody)
	}
	if !strings.HasPrefix(textBody, supportEmailReplyDelimiter+"\n\nThanks for reaching out about billing.") {
		t.Fatalf("expected plaintext body to start with delimiter then message, got %q", textBody)
	}
	if strings.Contains(htmlBody, "Reply directly to this email") || strings.Contains(textBody, "Reply directly to this email") {
		t.Fatalf("reply-directly footer text should be removed, html=%q text=%q", htmlBody, textBody)
	}
}

func TestEmailFallbackRenderBodiesUsesAttachmentPreviewWhenMessageHasNoText(t *testing.T) {
	svc := &EmailFallbackService{}

	htmlBody, _ := svc.renderBodies(
		[]model.SupportMessage{
			{
				Attachments: []model.SupportAttachmentPayload{{FileName: "report.pdf"}},
			},
		},
		"Alex Agent",
		"Acme Support",
		"",
		"",
	)

	if !strings.Contains(htmlBody, "Attachment from Acme Support") {
		t.Fatalf("expected attachment-only preheader fallback, got %q", htmlBody)
	}
}

type fakeSupportEmailAttachmentDownloader struct {
	data map[string][]byte
	err  error
}

func (f fakeSupportEmailAttachmentDownloader) DownloadContent(ctx context.Context, attachment model.SupportAttachmentPayload) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.data[attachment.ID], nil
}

type fakeSupportInboundEmailAttachmentStore struct {
	requests []supportInboundEmailAttachmentRequest
	urls     map[string]string
}

func (f *fakeSupportInboundEmailAttachmentStore) StoreInboundEmailAttachment(ctx context.Context, req supportInboundEmailAttachmentRequest) (*model.SupportAttachmentPayload, error) {
	f.requests = append(f.requests, req)
	id := req.ContentID
	if id == "" {
		id = req.FileName
	}
	return &model.SupportAttachmentPayload{
		ID:       id,
		FileKey:  "support/" + req.FileName,
		FileName: req.FileName,
		FileType: req.ContentType,
		FileSize: req.ContentLength,
		URL:      f.urls[req.ContentID],
	}, nil
}

func TestEmailFallbackPrepareEmailAttachmentsEmbedsSmallFilesAndLeavesLargeFilesLinked(t *testing.T) {
	svc := &EmailFallbackService{
		attachmentDownloader: fakeSupportEmailAttachmentDownloader{
			data: map[string][]byte{
				"small-file": []byte("pdf-bytes"),
			},
		},
	}

	messages, attachments := svc.prepareEmailAttachments(context.Background(), []model.SupportMessage{
		{
			Content: "Here is the report.",
			Attachments: []model.SupportAttachmentPayload{
				{
					ID:       "small-file",
					FileKey:  "support/small-file",
					FileName: "report.pdf",
					FileType: "application/pdf",
					FileSize: 1024,
					URL:      "https://cdn.example.com/report.pdf",
				},
				{
					ID:       "large-file",
					FileKey:  "support/large-file",
					FileName: "recording.mov",
					FileType: "video/quicktime",
					FileSize: supportEmailAttachmentMaxFileBytes + 1,
					URL:      "https://cdn.example.com/recording.mov",
				},
			},
		},
	})

	if len(attachments) != 1 {
		t.Fatalf("attachments len = %d, want 1", len(attachments))
	}
	if attachments[0].Name != "report.pdf" {
		t.Fatalf("attachment name = %q", attachments[0].Name)
	}
	if attachments[0].ContentType != "application/pdf" {
		t.Fatalf("attachment content type = %q", attachments[0].ContentType)
	}
	if attachments[0].Content != base64.StdEncoding.EncodeToString([]byte("pdf-bytes")) {
		t.Fatalf("attachment content = %q", attachments[0].Content)
	}
	if got := messages[0].Attachments[0].URL; got != "" {
		t.Fatalf("attached file URL should be cleared from body, got %q", got)
	}
	if got := messages[0].Attachments[1].URL; got != "https://cdn.example.com/recording.mov" {
		t.Fatalf("large file URL = %q", got)
	}
}

func TestEmailFallbackRenderBodiesOmitsRegularEmailAttachmentsFromBody(t *testing.T) {
	svc := &EmailFallbackService{}

	htmlBody, textBody := svc.renderBodies(
		[]model.SupportMessage{
			{
				Content: "Here is the report.",
				Attachments: []model.SupportAttachmentPayload{
					{
						FileName: "report.pdf",
						URL:      "",
					},
				},
			},
		},
		"Alex Agent",
		"Acme Support",
		"",
		"",
	)

	for _, body := range []string{htmlBody, textBody} {
		if strings.Contains(body, "Attachments:") || strings.Contains(body, "report.pdf") {
			t.Fatalf("regular email attachment should not be duplicated in body, got %q", body)
		}
	}
	if !strings.Contains(htmlBody, "<p>Here is the report.</p>") {
		t.Fatalf("expected message text to remain visible, got %q", htmlBody)
	}
}

func TestEmailFallbackReconcileMissedOutboundReplySendsOnce(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666667"
	anonymousID := "anon-reconcile"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Missed queue reply",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	msg := &model.SupportMessage{
		ID:                "55555555-5555-5555-5555-555555555555",
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("Alex Agent"),
		Content:           "This reply missed the redis queue.",
		MessageType:       "reply",
		CreatedAt:         fixedNow.Add(-3 * time.Minute),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	sendCount := 0
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sendCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-reconcile-1",
					"SubmittedAt": "2026-03-20T12:30:00Z",
					"To": "customer@example.com"
				}`)),
			}, nil
		}),
	})

	sent, err := env.service.ReconcileMissedOutboundEmails(ctx, 10)
	if err != nil {
		t.Fatalf("reconcile missed outbound emails: %v", err)
	}
	if sent != 1 {
		t.Fatalf("expected one reconciled send, got %d", sent)
	}
	if sendCount != 1 {
		t.Fatalf("expected one postmark send, got %d", sendCount)
	}

	reloaded, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if reloaded.EmailNotifiedAt == nil {
		t.Fatal("expected email_notified_at to be set")
	}

	sent, err = env.service.ReconcileMissedOutboundEmails(ctx, 10)
	if err != nil {
		t.Fatalf("second reconcile missed outbound emails: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected second reconcile to send nothing, got %d", sent)
	}
	if sendCount != 1 {
		t.Fatalf("expected no duplicate postmark send, got %d", sendCount)
	}
}

func TestEmailFallbackReconcileSkipsOnlineVisitor(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666668"
	anonymousID := "anon-reconcile-online"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Online missed queue reply",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := env.service.hub.Presence.SetVisitorOnline(ctx, workspaceID, anonymousID, "conn-1"); err != nil {
		t.Fatalf("set visitor online: %v", err)
	}

	msg := &model.SupportMessage{
		ID:                "55555555-5555-5555-5555-555555555556",
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("Alex Agent"),
		Content:           "This reply missed the redis queue but visitor is online.",
		MessageType:       "reply",
		CreatedAt:         fixedNow.Add(-3 * time.Minute),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	sendCount := 0
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sendCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK","MessageID":"pm-online","SubmittedAt":"2026-03-20T12:30:00Z","To":"customer@example.com"}`)),
			}, nil
		}),
	})

	sent, err := env.service.ReconcileMissedOutboundEmails(ctx, 10)
	if err != nil {
		t.Fatalf("reconcile missed outbound emails: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected online visitor reconcile to send nothing, got %d", sent)
	}
	if sendCount != 0 {
		t.Fatalf("expected no postmark send while visitor is online, got %d", sendCount)
	}
}

func TestEmailFallbackReconcileSkipsStaleMissedOutboundReply(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666677"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Stale missed queue reply",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	msg := &model.SupportMessage{
		ID:             "55555555-5555-5555-5555-555555555557",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "This reply is too old to reconcile.",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-20 * time.Minute),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected postmark send for stale reconcile candidate")
		return nil, nil
	})})

	sent, err := env.service.ReconcileMissedOutboundEmails(ctx, 10)
	if err != nil {
		t.Fatalf("reconcile missed outbound emails: %v", err)
	}
	if sent != 0 {
		t.Fatalf("expected stale reconcile candidate to send nothing, got %d", sent)
	}

	reloaded, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if reloaded.EmailNotifiedAt != nil {
		t.Fatal("did not expect email_notified_at to be set")
	}
}

func TestEmailFallbackBackfillSendsStaleMissedOutboundReplyInWindow(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666678"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Bug-window missed reply",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	msg := &model.SupportMessage{
		ID:             "55555555-5555-5555-5555-555555555558",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "This stale reply should be recovered by explicit backfill.",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-20 * time.Minute),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	sendCount := 0
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sendCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"ErrorCode": 0,
					"Message": "OK",
					"MessageID": "pm-backfill-1",
					"SubmittedAt": "2026-03-20T12:30:00Z",
					"To": "customer@example.com"
				}`)),
			}, nil
		}),
	})

	result, err := env.service.BackfillMissedOutboundEmails(ctx, EmailFallbackBackfillOptions{
		From:   fixedNow.Add(-30 * time.Minute),
		To:     fixedNow,
		Limit:  10,
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("backfill missed outbound emails: %v", err)
	}
	if result.SentMessages != 1 || result.SentConversations != 1 {
		t.Fatalf("unexpected backfill result: %+v", result)
	}
	if sendCount != 1 {
		t.Fatalf("expected one postmark send, got %d", sendCount)
	}

	reloaded, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if reloaded.EmailNotifiedAt == nil {
		t.Fatal("expected email_notified_at to be set")
	}
}

func TestEmailFallbackFireEmailSendsOnlyUnreadEligibleMessages(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666668"
	customerEmail := "customer@example.com"
	seenAt := fixedNow.Add(-2 * time.Minute)
	notifiedAt := fixedNow.Add(-30 * time.Second)
	conv := &model.SupportConversation{
		ID:                conversationID,
		WorkspaceID:       workspaceID,
		Subject:           "Mixed unread batch",
		Status:            "open",
		CustomerEmail:     &customerEmail,
		ContactLastSeenAt: &seenAt,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	messages := []*model.SupportMessage{
		{
			ID:             "10000000-0000-0000-0000-000000000001",
			WorkspaceID:    workspaceID,
			ConversationID: conversationID,
			SenderType:     "user",
			Content:        "Read in chat already.",
			MessageType:    "reply",
			CreatedAt:      fixedNow.Add(-3 * time.Minute),
		},
		{
			ID:              "10000000-0000-0000-0000-000000000002",
			WorkspaceID:     workspaceID,
			ConversationID:  conversationID,
			SenderType:      "user",
			Content:         "Already emailed.",
			MessageType:     "reply",
			CreatedAt:       fixedNow.Add(-30 * time.Second),
			EmailNotifiedAt: &notifiedAt,
		},
		{
			ID:             "10000000-0000-0000-0000-000000000003",
			WorkspaceID:    workspaceID,
			ConversationID: conversationID,
			SenderType:     "user",
			Content:        "Internal note should stay internal.",
			MessageType:    "reply",
			IsInternal:     true,
			CreatedAt:      fixedNow.Add(-20 * time.Second),
		},
		{
			ID:             "10000000-0000-0000-0000-000000000004",
			WorkspaceID:    workspaceID,
			ConversationID: conversationID,
			SenderType:     "customer",
			Content:        "Customer text should not be emailed back.",
			MessageType:    "reply",
			CreatedAt:      fixedNow.Add(-10 * time.Second),
		},
		{
			ID:                "10000000-0000-0000-0000-000000000005",
			WorkspaceID:       workspaceID,
			ConversationID:    conversationID,
			SenderType:        "user",
			SenderDisplayName: strPtr("Alex Agent"),
			Content:           "Fresh agent reply.",
			MessageType:       "reply",
			CreatedAt:         fixedNow.Add(-time.Minute),
		},
		{
			ID:                "10000000-0000-0000-0000-000000000006",
			WorkspaceID:       workspaceID,
			ConversationID:    conversationID,
			SenderType:        "ai",
			SenderDisplayName: strPtr("Helpin AI"),
			Content:           "Fresh AI reply.",
			MessageType:       "",
			CreatedAt:         fixedNow.Add(-15 * time.Second),
		},
	}
	messageIDs := make([]string, 0, len(messages))
	for _, msg := range messages {
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatalf("create message %s: %v", msg.ID, err)
		}
		messageIDs = append(messageIDs, msg.ID)
	}
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(fixedNow.Unix()), Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	queuedMessageIDs := make([]interface{}, 0, len(messageIDs))
	for _, id := range messageIDs {
		queuedMessageIDs = append(queuedMessageIDs, id)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), queuedMessageIDs...).Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}

	var captured capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK","MessageID":"pm-mixed-1","SubmittedAt":"2026-03-20T12:30:00Z","To":"customer@example.com"}`)),
			}, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, messageIDs); err != nil {
		t.Fatalf("fire email: %v", err)
	}

	for _, unexpected := range []string{"Read in chat already", "Already emailed", "Internal note", "Customer text"} {
		if strings.Contains(captured.TextBody, unexpected) {
			t.Fatalf("email body included ineligible content %q: %q", unexpected, captured.TextBody)
		}
	}
	for _, expected := range []string{"Fresh agent reply", "Fresh AI reply"} {
		if !strings.Contains(captured.TextBody, expected) {
			t.Fatalf("email body missing eligible content %q: %q", expected, captured.TextBody)
		}
	}

	savedMessages, err := env.messageRepo.GetByIDs(ctx, messageIDs)
	if err != nil {
		t.Fatalf("reload messages: %v", err)
	}
	notified := map[string]bool{}
	for _, msg := range savedMessages {
		notified[msg.ID] = msg.EmailNotifiedAt != nil
	}
	for _, id := range []string{"10000000-0000-0000-0000-000000000005", "10000000-0000-0000-0000-000000000006"} {
		if !notified[id] {
			t.Fatalf("expected eligible message %s to be marked email-notified", id)
		}
	}
	for _, id := range []string{"10000000-0000-0000-0000-000000000001", "10000000-0000-0000-0000-000000000003", "10000000-0000-0000-0000-000000000004"} {
		if notified[id] {
			t.Fatalf("expected ineligible message %s to remain unnotified", id)
		}
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one outbound log, got %d", len(logs))
	}
	if got, want := []string(logs[0].MessageIDs), []string{"10000000-0000-0000-0000-000000000005", "10000000-0000-0000-0000-000000000006"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected logged message ids: got %#v want %#v", got, want)
	}
}

func TestUnreadFallbackMessagesFiltersReadAndIneligibleReplies(t *testing.T) {
	seenAt := time.Date(2026, 3, 20, 12, 5, 0, 0, time.UTC)
	notifiedAt := seenAt.Add(-time.Minute)
	messages := []model.SupportMessage{
		{ID: "customer", SenderType: "customer", MessageType: "reply", CreatedAt: seenAt.Add(time.Minute)},
		{ID: "internal", SenderType: "user", MessageType: "reply", IsInternal: true, CreatedAt: seenAt.Add(time.Minute)},
		{ID: "system", SenderType: "user", MessageType: "system", CreatedAt: seenAt.Add(time.Minute)},
		{ID: "notified", SenderType: "user", MessageType: "reply", CreatedAt: seenAt.Add(time.Minute), EmailNotifiedAt: &notifiedAt},
		{ID: "read", SenderType: "user", MessageType: "reply", CreatedAt: seenAt},
		{ID: "unread-1", SenderType: "user", MessageType: "reply", CreatedAt: seenAt.Add(time.Minute)},
		{ID: "unread-2", SenderType: "ai", MessageType: "", CreatedAt: seenAt.Add(2 * time.Minute)},
	}

	pending := unreadFallbackMessages(&model.SupportConversation{ContactLastSeenAt: &seenAt}, messages)
	if len(pending) != 2 || pending[0].ID != "unread-1" || pending[1].ID != "unread-2" {
		t.Fatalf("unexpected pending messages: %#v", pending)
	}
}

func TestUnreadFallbackMessagesTreatsNilLastSeenAsUnread(t *testing.T) {
	createdAt := time.Date(2026, 3, 20, 12, 5, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{ID: "agent", SenderType: "agent", MessageType: "reply", CreatedAt: createdAt},
		{ID: "user", SenderType: "user", MessageType: "reply", CreatedAt: createdAt.Add(time.Second)},
		{ID: "ai", SenderType: "ai", MessageType: "reply", CreatedAt: createdAt.Add(2 * time.Second)},
	}

	pending := unreadFallbackMessages(&model.SupportConversation{}, messages)
	if len(pending) != 3 {
		t.Fatalf("expected all agent-side replies to be unread without contact_last_seen_at, got %#v", pending)
	}
}

func TestFreshEmailFallbackMessagesDropsOnlyStaleMessages(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{ID: "stale", CreatedAt: now.Add(-10*time.Minute - time.Nanosecond)},
		{ID: "boundary", CreatedAt: now.Add(-10 * time.Minute)},
		{ID: "fresh", CreatedAt: now.Add(-9*time.Minute - 59*time.Second)},
		{ID: "zero"},
	}

	fresh := freshEmailFallbackMessages(messages, now, 10*time.Minute)
	if len(fresh) != 3 || fresh[0].ID != "boundary" || fresh[1].ID != "fresh" || fresh[2].ID != "zero" {
		t.Fatalf("unexpected fresh messages: %#v", fresh)
	}
}

func TestEmailFallbackFireEmailVisitorOnlineUnreadPostponesWithinGraceWindow(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666671"
	anonymousID := "anon-online-unread"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Unread online",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Unread while online",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-120 * time.Second),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(fixedNow.Unix()), Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), msg.ID).Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}
	if err := env.service.hub.Presence.SetVisitorOnline(ctx, workspaceID, anonymousID, "conn-1"); err != nil {
		t.Fatalf("set visitor online: %v", err)
	}
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected postmark send while visitor is online")
		return nil, nil
	})})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}
	score, err := env.redis.ZScore(ctx, emailFallbackOutboxKey, conversationID).Result()
	if err != nil {
		t.Fatalf("get postponed score: %v", err)
	}
	if got, want := int64(score), fixedNow.Add(emailFallbackOnlineRetry).Unix(); got != want {
		t.Fatalf("postponed score = %d, want %d", got, want)
	}
	if exists, err := env.redis.Exists(ctx, env.service.msgListKey(conversationID)).Result(); err != nil {
		t.Fatalf("check msg list: %v", err)
	} else if exists != 1 {
		t.Fatalf("expected msg list to remain, exists=%d", exists)
	}
}

func TestEmailFallbackFireEmailVisitorOnlineUnreadKeepsPostponingAfterGraceWindow(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 120
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666672"
	anonymousID := "anon-online-unread-after-grace"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Unread online after grace",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Unread after online grace",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-151 * time.Second),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(fixedNow.Unix()), Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), msg.ID).Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}
	if err := env.service.hub.Presence.SetVisitorOnline(ctx, workspaceID, anonymousID, "conn-1"); err != nil {
		t.Fatalf("set visitor online: %v", err)
	}

	sendCount := 0
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sendCount++
			t.Fatalf("unexpected postmark send while visitor is still online")
			return nil, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}
	if sendCount != 0 {
		t.Fatalf("expected no postmark send while visitor is online, got %d", sendCount)
	}
	score, err := env.redis.ZScore(ctx, emailFallbackOutboxKey, conversationID).Result()
	if err != nil {
		t.Fatalf("get postponed score: %v", err)
	}
	if got, want := int64(score), fixedNow.Add(emailFallbackOnlineRetry).Unix(); got != want {
		t.Fatalf("postponed score = %d, want %d", got, want)
	}
	if exists, err := env.redis.Exists(ctx, emailFallbackOutboxKey, env.service.msgListKey(conversationID)).Result(); err != nil {
		t.Fatalf("check redis keys: %v", err)
	} else if exists != 2 {
		t.Fatalf("expected outbox and msg list to remain, found %d keys", exists)
	}
}

func TestEmailFallbackFireEmailReadMessagesCleanUp(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666672"
	customerEmail := "customer@example.com"
	seenAt := fixedNow.Add(-30 * time.Second)
	conv := &model.SupportConversation{
		ID:                conversationID,
		WorkspaceID:       workspaceID,
		Subject:           "Read already",
		Status:            "open",
		ContactLastSeenAt: &seenAt,
		CustomerEmail:     &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Already read",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-time.Minute),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(fixedNow.Unix()), Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), msg.ID).Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}
	if exists, err := env.redis.Exists(ctx, emailFallbackOutboxKey, env.service.msgListKey(conversationID)).Result(); err != nil {
		t.Fatalf("check redis cleanup: %v", err)
	} else if exists != 0 {
		t.Fatalf("expected redis cleanup, found %d keys", exists)
	}
}

func TestEmailFallbackFireEmailStaleUnreadCleanUp(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666673"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Stale unread",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Too old to email",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-11 * time.Minute),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if err := env.redis.ZAdd(ctx, emailFallbackOutboxKey, redis.Z{Score: float64(fixedNow.Unix()), Member: conversationID}).Err(); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	if err := env.redis.RPush(ctx, env.service.msgListKey(conversationID), msg.ID).Err(); err != nil {
		t.Fatalf("seed msg list: %v", err)
	}
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected postmark send for stale fallback")
		return nil, nil
	})})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}
	if exists, err := env.redis.Exists(ctx, emailFallbackOutboxKey, env.service.msgListKey(conversationID)).Result(); err != nil {
		t.Fatalf("check redis cleanup: %v", err)
	} else if exists != 0 {
		t.Fatalf("expected redis cleanup, found %d keys", exists)
	}
}

func TestEmailFallbackFireEmailDropsStaleMessagesFromMixedBatch(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666674"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Mixed stale unread",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	staleMsg := &model.SupportMessage{
		ID:             "20000000-0000-0000-0000-000000000001",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "This old reply should not be emailed.",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-11 * time.Minute),
	}
	freshMsg := &model.SupportMessage{
		ID:             "20000000-0000-0000-0000-000000000002",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "This fresh reply should be emailed.",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-2 * time.Minute),
	}
	for _, msg := range []*model.SupportMessage{staleMsg, freshMsg} {
		if err := env.messageRepo.Create(ctx, msg); err != nil {
			t.Fatalf("create message %s: %v", msg.ID, err)
		}
	}

	var captured capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK","MessageID":"pm-mixed-stale-1","SubmittedAt":"2026-03-20T12:30:00Z","To":"customer@example.com"}`)),
			}, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, []string{staleMsg.ID, freshMsg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}
	if strings.Contains(captured.TextBody, staleMsg.Content) {
		t.Fatalf("email body included stale content: %q", captured.TextBody)
	}
	if !strings.Contains(captured.TextBody, freshMsg.Content) {
		t.Fatalf("email body missing fresh content: %q", captured.TextBody)
	}

	savedMessages, err := env.messageRepo.GetByIDs(ctx, []string{staleMsg.ID, freshMsg.ID})
	if err != nil {
		t.Fatalf("reload messages: %v", err)
	}
	for _, msg := range savedMessages {
		switch msg.ID {
		case staleMsg.ID:
			if msg.EmailNotifiedAt != nil {
				t.Fatal("expected stale message to remain unnotified")
			}
		case freshMsg.ID:
			if msg.EmailNotifiedAt == nil {
				t.Fatal("expected fresh message to be marked email-notified")
			}
		}
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 || len(logs[0].MessageIDs) != 1 || logs[0].MessageIDs[0] != freshMsg.ID {
		t.Fatalf("expected only fresh message in outbound log, got %#v", logs)
	}
}

func TestEmailFallbackFireEmailSendsAtFreshnessBoundary(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

	fixedNow := time.Date(2026, 3, 20, 12, 30, 0, 0, time.UTC)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666666675"
	customerEmail := "customer@example.com"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Boundary unread",
		Status:        "open",
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Exactly ten minutes old.",
		MessageType:    "reply",
		CreatedAt:      fixedNow.Add(-10 * time.Minute),
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	sendCount := 0
	env.service.emailClient.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sendCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK","MessageID":"pm-boundary-1","SubmittedAt":"2026-03-20T12:30:00Z","To":"customer@example.com"}`)),
			}, nil
		}),
	})

	if err := env.service.fireEmail(ctx, conversationID, []string{msg.ID}); err != nil {
		t.Fatalf("fire email: %v", err)
	}
	if sendCount != 1 {
		t.Fatalf("expected one email at freshness boundary, got %d", sendCount)
	}
	savedMsg, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if savedMsg.EmailNotifiedAt == nil {
		t.Fatal("expected boundary message to be marked email-notified")
	}
}

func TestEmailFallbackProcessOpenEventMarksMessagesRead(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "99999999-9999-9999-9999-999999999999"
	readAt := time.Date(2026, 3, 20, 13, 5, 0, 0, time.UTC)
	env.service.now = func() time.Time { return readAt }

	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Following up via email.",
		MessageType:    "reply",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create msg: %v", err)
	}

	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:                "aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa1",
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		Direction:         "outbound",
		MessageIDs:        model.DocsStringArray{msg.ID},
		PostmarkMessageID: strPtr("pm-open-1"),
		Status:            "sent",
	}); err != nil {
		t.Fatalf("seed email log: %v", err)
	}

	if err := env.service.ProcessOpenEvent(ctx, model.PostmarkOpenPayload{
		RecordType: "Open",
		MessageID:  "pm-open-1",
		FirstOpen:  true,
		ReceivedAt: "2026-03-20T13:05:00Z",
	}, `{"RecordType":"Open","MessageID":"pm-open-1","FirstOpen":true,"ReceivedAt":"2026-03-20T13:05:00Z"}`); err != nil {
		t.Fatalf("process open event: %v", err)
	}

	savedMsg, err := env.messageRepo.GetByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if savedMsg == nil || savedMsg.EmailReadAt == nil || !savedMsg.EmailReadAt.Equal(readAt) {
		t.Fatalf("expected email_read_at=%v, got %#v", readAt, savedMsg)
	}

	logRow, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, "pm-open-1")
	if err != nil {
		t.Fatalf("reload email log: %v", err)
	}
	if logRow == nil || logRow.Status != "opened" {
		t.Fatalf("expected opened email log, got %#v", logRow)
	}
	if logRow.OpenedAt == nil || !logRow.OpenedAt.Equal(readAt) {
		t.Fatalf("expected opened_at=%v, got %#v", readAt, logRow.OpenedAt)
	}

	events, err := env.webhookRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "open" {
		t.Fatalf("expected one open webhook event, got %#v", events)
	}
}

func TestEmailFallbackProcessDeliveryEventMarksDelivered(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "99999999-9999-9999-9999-999999999998"
	deliveredAt := time.Date(2026, 3, 20, 13, 15, 0, 0, time.UTC)
	env.service.now = func() time.Time { return deliveredAt }

	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Following up via email.",
		MessageType:    "reply",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create msg: %v", err)
	}

	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:                "aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa2",
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		Direction:         "outbound",
		MessageIDs:        model.DocsStringArray{msg.ID},
		PostmarkMessageID: strPtr("pm-delivery-1"),
		Status:            "sent",
	}); err != nil {
		t.Fatalf("seed email log: %v", err)
	}

	rawPayload := `{"RecordType":"Delivery","MessageID":"pm-delivery-1","DeliveredAt":"2026-03-20T13:15:00Z"}`
	if err := env.service.ProcessDeliveryEvent(ctx, model.PostmarkDeliveryPayload{
		RecordType:  "Delivery",
		MessageID:   "pm-delivery-1",
		DeliveredAt: "2026-03-20T13:15:00Z",
	}, rawPayload); err != nil {
		t.Fatalf("process delivery event: %v", err)
	}
	if err := env.service.ProcessDeliveryEvent(ctx, model.PostmarkDeliveryPayload{
		RecordType:  "Delivery",
		MessageID:   "pm-delivery-1",
		DeliveredAt: "2026-03-20T13:15:00Z",
	}, rawPayload); err != nil {
		t.Fatalf("process duplicate delivery event: %v", err)
	}

	logRow, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, "pm-delivery-1")
	if err != nil {
		t.Fatalf("reload email log: %v", err)
	}
	if logRow == nil || logRow.Status != "delivered" {
		t.Fatalf("expected delivered email log, got %#v", logRow)
	}
	if logRow.DeliveredAt == nil || !logRow.DeliveredAt.Equal(deliveredAt) {
		t.Fatalf("expected delivered_at=%v, got %#v", deliveredAt, logRow.DeliveredAt)
	}

	events, err := env.webhookRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two delivery webhook events, got %#v", events)
	}
	if events[0].EventType != "delivery" || events[1].EventType != "delivery" {
		t.Fatalf("expected delivery events, got %#v", events)
	}
}

func TestEmailFallbackProcessBounceEventMarksBounced(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "99999999-9999-9999-9999-999999999997"
	bouncedAt := time.Date(2026, 3, 20, 13, 25, 0, 0, time.UTC)
	env.service.now = func() time.Time { return bouncedAt }

	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Following up via email.",
		MessageType:    "reply",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create msg: %v", err)
	}

	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:                "aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa3",
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		Direction:         "outbound",
		MessageIDs:        model.DocsStringArray{msg.ID},
		PostmarkMessageID: strPtr("pm-bounce-1"),
		Status:            "sent",
	}); err != nil {
		t.Fatalf("seed email log: %v", err)
	}

	rawPayload := `{"RecordType":"Bounce","MessageID":"pm-bounce-1","BouncedAt":"2026-03-20T13:25:00Z","Description":"Hard bounce"}`
	if err := env.service.ProcessBounceEvent(ctx, model.PostmarkBouncePayload{
		RecordType:  "Bounce",
		MessageID:   "pm-bounce-1",
		BouncedAt:   "2026-03-20T13:25:00Z",
		Description: "Hard bounce",
	}, rawPayload); err != nil {
		t.Fatalf("process bounce event: %v", err)
	}
	if err := env.service.ProcessBounceEvent(ctx, model.PostmarkBouncePayload{
		RecordType:  "Bounce",
		MessageID:   "pm-bounce-1",
		BouncedAt:   "2026-03-20T13:25:00Z",
		Description: "Hard bounce",
	}, rawPayload); err != nil {
		t.Fatalf("process duplicate bounce event: %v", err)
	}

	logRow, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, "pm-bounce-1")
	if err != nil {
		t.Fatalf("reload email log: %v", err)
	}
	if logRow == nil || logRow.Status != "bounced" {
		t.Fatalf("expected bounced email log, got %#v", logRow)
	}
	if logRow.BouncedAt == nil || !logRow.BouncedAt.Equal(bouncedAt) {
		t.Fatalf("expected bounced_at=%v, got %#v", bouncedAt, logRow.BouncedAt)
	}
	if logRow.ErrorMessage != "Hard bounce" {
		t.Fatalf("expected error_message=Hard bounce, got %q", logRow.ErrorMessage)
	}

	events, err := env.webhookRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two bounce webhook events, got %#v", events)
	}
	if events[0].EventType != "bounce" || events[1].EventType != "bounce" {
		t.Fatalf("expected bounce events, got %#v", events)
	}
}

func TestEmailFallbackProcessSpamComplaintEventMarksSpamComplaint(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "99999999-9999-9999-9999-999999999996"
	complaintAt := time.Date(2026, 3, 20, 13, 35, 0, 0, time.UTC)
	env.service.now = func() time.Time { return complaintAt }

	msg := &model.SupportMessage{
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		SenderType:     "user",
		Content:        "Following up via email.",
		MessageType:    "reply",
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create msg: %v", err)
	}

	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:                "aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa4",
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		Direction:         "outbound",
		MessageIDs:        model.DocsStringArray{msg.ID},
		PostmarkMessageID: strPtr("pm-complaint-1"),
		Status:            "delivered",
	}); err != nil {
		t.Fatalf("seed email log: %v", err)
	}

	rawPayload := `{"RecordType":"SpamComplaint","MessageID":"pm-complaint-1","BouncedAt":"2026-03-20T13:35:00Z","Description":"Marked as spam"}`
	if err := env.service.ProcessSpamComplaintEvent(ctx, model.PostmarkSpamComplaintPayload{
		RecordType:  "SpamComplaint",
		MessageID:   "pm-complaint-1",
		BouncedAt:   "2026-03-20T13:35:00Z",
		Description: "Marked as spam",
	}, rawPayload); err != nil {
		t.Fatalf("process spam complaint event: %v", err)
	}
	if err := env.service.ProcessSpamComplaintEvent(ctx, model.PostmarkSpamComplaintPayload{
		RecordType:  "SpamComplaint",
		MessageID:   "pm-complaint-1",
		BouncedAt:   "2026-03-20T13:35:00Z",
		Description: "Marked as spam",
	}, rawPayload); err != nil {
		t.Fatalf("process duplicate spam complaint event: %v", err)
	}

	logRow, err := env.emailLogRepo.GetByPostmarkMessageID(ctx, "pm-complaint-1")
	if err != nil {
		t.Fatalf("reload email log: %v", err)
	}
	if logRow == nil || logRow.Status != "spam_complaint" {
		t.Fatalf("expected spam complaint email log, got %#v", logRow)
	}
	if logRow.BouncedAt == nil || !logRow.BouncedAt.Equal(complaintAt) {
		t.Fatalf("expected complaint timestamp=%v, got %#v", complaintAt, logRow.BouncedAt)
	}
	if logRow.ErrorMessage != "Marked as spam" {
		t.Fatalf("expected error_message=Marked as spam, got %q", logRow.ErrorMessage)
	}

	events, err := env.webhookRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two spam complaint webhook events, got %#v", events)
	}
	if events[0].EventType != "spam_complaint" || events[1].EventType != "spam_complaint" {
		t.Fatalf("expected spam complaint events, got %#v", events)
	}
}

func TestEmailFallbackProcessPostmarkStatusEventsRecordAuditWithoutMatchingEmailLog(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		eventType string
		run       func(*emailFallbackTestEnv) error
	}{
		{
			name:      "delivery missing message id",
			eventType: "delivery",
			run: func(env *emailFallbackTestEnv) error {
				return env.service.ProcessDeliveryEvent(ctx, model.PostmarkDeliveryPayload{
					RecordType:  "Delivery",
					DeliveredAt: "2026-03-20T13:15:00Z",
				}, "")
			},
		},
		{
			name:      "delivery without matching email log",
			eventType: "delivery",
			run: func(env *emailFallbackTestEnv) error {
				return env.service.ProcessDeliveryEvent(ctx, model.PostmarkDeliveryPayload{
					RecordType:  "Delivery",
					MessageID:   "pm-delivery-missing",
					DeliveredAt: "2026-03-20T13:15:00Z",
				}, "")
			},
		},
		{
			name:      "bounce missing message id",
			eventType: "bounce",
			run: func(env *emailFallbackTestEnv) error {
				return env.service.ProcessBounceEvent(ctx, model.PostmarkBouncePayload{
					RecordType:  "Bounce",
					BouncedAt:   "2026-03-20T13:25:00Z",
					Description: "Mailbox unavailable",
				}, "")
			},
		},
		{
			name:      "bounce without matching email log",
			eventType: "bounce",
			run: func(env *emailFallbackTestEnv) error {
				return env.service.ProcessBounceEvent(ctx, model.PostmarkBouncePayload{
					RecordType:  "Bounce",
					MessageID:   "pm-bounce-missing",
					BouncedAt:   "2026-03-20T13:25:00Z",
					Description: "Mailbox unavailable",
				}, "")
			},
		},
		{
			name:      "spam complaint missing message id",
			eventType: "spam_complaint",
			run: func(env *emailFallbackTestEnv) error {
				return env.service.ProcessSpamComplaintEvent(ctx, model.PostmarkSpamComplaintPayload{
					RecordType:  "SpamComplaint",
					BouncedAt:   "2026-03-20T13:35:00Z",
					Description: "Marked as spam",
				}, "")
			},
		},
		{
			name:      "spam complaint without matching email log",
			eventType: "spam_complaint",
			run: func(env *emailFallbackTestEnv) error {
				return env.service.ProcessSpamComplaintEvent(ctx, model.PostmarkSpamComplaintPayload{
					RecordType:  "SpamComplaint",
					MessageID:   "pm-complaint-missing",
					BouncedAt:   "2026-03-20T13:35:00Z",
					Description: "Marked as spam",
				}, "")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settings := model.DefaultSupportInboxSettings()
			env := setupEmailFallbackInboundTestEnv(t, settings)

			if err := tc.run(env); err != nil {
				t.Fatalf("run event: %v", err)
			}

			resp, err := env.webhookRepo.ListPaginated(ctx, 1, 10, tc.eventType, "postmark")
			if err != nil {
				t.Fatalf("list webhook events: %v", err)
			}
			if len(resp.Data) != 1 {
				t.Fatalf("expected one %s webhook event, got %#v", tc.eventType, resp.Data)
			}
		})
	}
}

func TestEmailFallbackProcessInboundEmailCreatesMessageAndDedupes(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "88888888-8888-8888-8888-888888888888"
	customerEmail := "customer@example.com"
	customerName := "Taylor"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Inbound test",
		Status:        "open",
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: customerEmail, Name: customerName},
		To:                "conv-" + conversationID + "@replies.helpin.ai",
		OriginalRecipient: "conv-" + conversationID + "@replies.helpin.ai",
		Subject:           "Re: Inbound test",
		MessageID:         "pm-in-1",
		StrippedTextReply: "Thanks, that helps. https://example.com",
		HtmlBody:          "<p>Thanks, that helps. https://example.com</p>",
	}

	rawInboundPayload := `{"MessageStream":"inbound","MessageID":"pm-in-1","OriginalRecipient":"conv-` + conversationID + `@replies.helpin.ai","To":"conv-` + conversationID + `@replies.helpin.ai","StrippedTextReply":"Thanks, that helps. https://example.com"}`
	if err := env.service.ProcessInboundEmail(ctx, payload, rawInboundPayload); err != nil {
		t.Fatalf("process inbound email: %v", err)
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, rawInboundPayload); err != nil {
		t.Fatalf("process duplicate inbound email: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 inbound message, got %d", len(messages))
	}
	if messages[0].ViaChannel == nil || *messages[0].ViaChannel != "email" {
		t.Fatalf("expected via_channel=email, got %#v", messages[0].ViaChannel)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 inbound email log, got %d", len(logs))
	}
	if logs[0].Direction != "inbound" {
		t.Fatalf("expected inbound direction, got %q", logs[0].Direction)
	}
	if logs[0].PostmarkMessageID == nil || *logs[0].PostmarkMessageID != "pm-in-1" {
		t.Fatalf("unexpected inbound postmark message id: %#v", logs[0].PostmarkMessageID)
	}
	if !strings.Contains(logs[0].RawBody, `"MessageID":"pm-in-1"`) || !strings.Contains(logs[0].RawBody, `"StrippedTextReply":"Thanks, that helps. https://example.com"`) {
		t.Fatalf("expected raw payload json to be stored, got %q", logs[0].RawBody)
	}

	events, err := env.webhookRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 webhook event rows for duplicate posts, got %d", len(events))
	}
	if events[0].EventType != "inbound" {
		t.Fatalf("expected inbound webhook event, got %#v", events[0])
	}
	if !strings.Contains(events[0].RawPayload, `"OriginalRecipient":"conv-`+conversationID+`@replies.helpin.ai"`) {
		t.Fatalf("expected raw webhook payload to be stored, got %q", events[0].RawPayload)
	}
}

func TestEmailFallbackProcessInboundEmailRewritesInlineCIDImages(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	store := &fakeSupportInboundEmailAttachmentStore{
		urls: map[string]string{
			"image001.png@01DCF421.90A2D1C0": "https://assets.example.com/image001.png",
		},
	}
	env.service.inboundAttachmentStore = store

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "88888888-8888-8888-8888-888888888889"
	customerEmail := "customer@example.com"
	customerName := "Taylor"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Inline image",
		Status:        "open",
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: customerEmail, Name: customerName},
		To:                "conv-" + conversationID + "@replies.helpin.ai",
		OriginalRecipient: "conv-" + conversationID + "@replies.helpin.ai",
		Subject:           "Re: Inline image",
		MessageID:         "pm-in-inline-image",
		TextBody:          "[cid:image001.png@01DCF421.90A2D1C0]\n\nThanks,\nViktoria",
		HtmlBody:          `<div><img width="538" height="554" src="cid:image001.png@01DCF421.90A2D1C0"><p>Thanks,<br>Viktoria</p></div>`,
		Attachments: []model.PostmarkInboundAttachment{
			{
				Name:          "image001.png",
				Content:       base64.StdEncoding.EncodeToString([]byte("png-bytes")),
				ContentType:   "image/png",
				ContentLength: int64(len("png-bytes")),
				ContentID:     "image001.png@01DCF421.90A2D1C0",
			},
		},
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
		t.Fatalf("process inbound email: %v", err)
	}

	if len(store.requests) != 0 {
		t.Fatal("upload ran before message commit")
	}
	if err := env.service.processNextInboundJob(ctx, "attachment"); err != nil {
		t.Fatal(err)
	}
	if len(store.requests) != 1 {
		t.Fatalf("stored attachment requests = %d, want 1", len(store.requests))
	}
	if store.requests[0].MessageID == "" {
		t.Fatalf("expected inbound attachment to be linked to generated message id")
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 inbound email log, got %d", len(logs))
	}
	var messages []model.SupportMessage
	if err := env.convRepo.DB().Where("conversation_id = ?", conversationID).Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	attachmentService := NewSupportAttachmentService(repository.NewSupportAttachmentRepository(env.convRepo.DB()), nil)
	if err := attachmentService.HydrateMessages(ctx, messages); err != nil {
		t.Fatal(err)
	}
	hydrateEmailBodiesFromLogs(messages, logs)
	if strings.Contains(messages[0].HTMLBody, "cid:image001") {
		t.Fatalf("expected cid src to be rewritten, got %q", logs[0].HTMLBody)
	}
	if !strings.Contains(messages[0].HTMLBody, `src="https://assets.example.com/image001.png"`) {
		t.Fatalf("expected stored asset URL in HTML body, got %q", logs[0].HTMLBody)
	}
	if strings.Contains(logs[0].StrippedText, "[cid:") {
		t.Fatalf("expected cid placeholder stripped from text, got %q", logs[0].StrippedText)
	}
}

func TestEmailFallbackProcessInboundEmailRouteCreatesConversation(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{
		ID:             "a1111111-1111-1111-1111-111111111111",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-shared123",
		InboundAddress: "inbox@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "customer@example.com", Name: "Taylor"},
		To:                route.InboundAddress,
		OriginalRecipient: route.InboundAddress,
		Subject:           "Need billing help",
		MessageID:         "pm-route-1",
		StrippedTextReply: "I need help with my invoice",
		Headers: []model.PostmarkHeader{
			{Name: "Message-ID", Value: "<customer-thread-1@example.com>"},
		},
	}

	rawPayload := `{"MessageID":"pm-route-1","OriginalRecipient":"` + route.InboundAddress + `","Subject":"Need billing help"}`
	if err := env.service.ProcessInboundEmail(ctx, payload, rawPayload); err != nil {
		t.Fatalf("process routed inbound email: %v", err)
	}

	resp, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(resp) != 1 {
		t.Fatalf("expected 1 conversation, got total=%d len=%d", total, len(resp))
	}
	conv := resp[0]
	if conv.Channel != "email" || conv.Source != "email" {
		t.Fatalf("expected email conversation, got channel=%q source=%q", conv.Channel, conv.Source)
	}
	if conv.CustomerEmail == nil || *conv.CustomerEmail != "customer@example.com" {
		t.Fatalf("unexpected customer email: %#v", conv.CustomerEmail)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conv.ID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 || messages[0].ViaChannel == nil || *messages[0].ViaChannel != "email" {
		t.Fatalf("expected 1 email message, got %#v", messages)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conv.ID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 email log, got %d", len(logs))
	}
	if logs[0].EmailRouteID == nil || *logs[0].EmailRouteID != route.ID {
		t.Fatalf("expected route id on log, got %#v", logs[0].EmailRouteID)
	}
	if logs[0].RFCMessageID != "<customer-thread-1@example.com>" {
		t.Fatalf("unexpected rfc message id: %q", logs[0].RFCMessageID)
	}
}

func TestEmailFallbackProcessInboundEmailRouteFromCcRequiresPrimaryRecipientConfirmation(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{
		ID:             "a2111111-1111-1111-1111-111111111115",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-copied",
		InboundAddress: "support@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "teammate@company.com", Name: "Alex Teammate"},
		To:                "Jane Persona <jane@example.com>",
		ToFull:            []model.PostmarkAddress{{Email: "jane@example.com", Name: "Jane Persona"}},
		Cc:                route.InboundAddress,
		CcFull:            []model.PostmarkAddress{{Email: route.InboundAddress, Name: "Support"}},
		OriginalRecipient: "",
		Subject:           "Can you handle this?",
		MessageID:         "pm-route-copied-1",
		StrippedTextReply: "Looping support in.",
		Headers: []model.PostmarkHeader{
			{Name: "Message-ID", Value: "<copied-thread-1@example.com>"},
		},
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-copied-1"}`); err != nil {
		t.Fatalf("process copied inbound email: %v", err)
	}

	conversations, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(conversations) != 1 {
		t.Fatalf("expected 1 conversation, got total=%d len=%d", total, len(conversations))
	}
	conv := conversations[0]
	if conv.CustomerEmail == nil || *conv.CustomerEmail != "teammate@company.com" {
		t.Fatalf("customer_email = %#v, want teammate sender", conv.CustomerEmail)
	}
	if conv.PrimaryRecipientState != model.SupportPrimaryRecipientStateUnconfirmed {
		t.Fatalf("primary_recipient_state = %q", conv.PrimaryRecipientState)
	}
	if conv.SuggestedPrimaryRecipientEmail == nil || *conv.SuggestedPrimaryRecipientEmail != "jane@example.com" {
		t.Fatalf("suggested_primary_recipient_email = %#v", conv.SuggestedPrimaryRecipientEmail)
	}
	if len(conv.EmailThreadParticipants) != 1 || conv.EmailThreadParticipants[0] != "jane@example.com" {
		t.Fatalf("email_thread_participants = %#v", conv.EmailThreadParticipants)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conv.ID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 email log, got %d", len(logs))
	}
	if len(logs[0].CCEmails) != 1 || logs[0].CCEmails[0] != route.InboundAddress {
		t.Fatalf("cc_emails = %#v", logs[0].CCEmails)
	}
}

func TestEmailFallbackProcessInboundEmailRouteFromToPersistsCustomerCCForReplies(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{
		ID:             "a2111111-1111-1111-1111-111111111116",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-direct-cc",
		InboundAddress: "support@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "customer@company.com", Name: "Casey Customer"},
		To:                route.InboundAddress,
		ToFull:            []model.PostmarkAddress{{Email: route.InboundAddress, Name: "Support"}},
		Cc:                "teammate1@company.com, Teammate Two <teammate2@company.com>",
		CcFull:            []model.PostmarkAddress{{Email: "teammate1@company.com", Name: "Teammate One"}, {Email: "teammate2@company.com", Name: "Teammate Two"}},
		OriginalRecipient: route.InboundAddress,
		Subject:           "Need help with billing",
		MessageID:         "pm-route-direct-cc-1",
		StrippedTextReply: "Can you help us?",
		Headers: []model.PostmarkHeader{
			{Name: "Message-ID", Value: "<direct-cc-thread-1@example.com>"},
		},
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-direct-cc-1"}`); err != nil {
		t.Fatalf("process direct inbound email with cc: %v", err)
	}

	conversations, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(conversations) != 1 {
		t.Fatalf("expected 1 conversation, got total=%d len=%d", total, len(conversations))
	}
	conv := conversations[0]
	if conv.CustomerEmail == nil || *conv.CustomerEmail != "customer@company.com" {
		t.Fatalf("customer_email = %#v, want customer sender", conv.CustomerEmail)
	}
	if conv.PrimaryRecipientState != model.SupportPrimaryRecipientStateConfirmed {
		t.Fatalf("primary_recipient_state = %q", conv.PrimaryRecipientState)
	}
	if len(conv.EmailCC) != 2 || conv.EmailCC[0] != "teammate1@company.com" || conv.EmailCC[1] != "teammate2@company.com" {
		t.Fatalf("email_cc = %#v, want customer cc teammates", conv.EmailCC)
	}
	if len(conv.EmailThreadParticipants) != 2 || conv.EmailThreadParticipants[0] != "teammate1@company.com" || conv.EmailThreadParticipants[1] != "teammate2@company.com" {
		t.Fatalf("email_thread_participants = %#v", conv.EmailThreadParticipants)
	}
}

func TestEmailFallbackProcessInboundEmailRouteDerivesCustomerNameFromEmail(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{
		ID:             "a2111111-1111-1111-1111-111111111111",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-derived-name",
		InboundAddress: "support@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "matta.trisha@gmail.com"},
		To:                route.InboundAddress,
		OriginalRecipient: route.InboundAddress,
		Subject:           "Question",
		MessageID:         "pm-route-derived-name",
		StrippedTextReply: "Can you help?",
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-derived-name"}`); err != nil {
		t.Fatalf("process routed inbound email: %v", err)
	}

	conversations, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(conversations) != 1 {
		t.Fatalf("expected 1 conversation, got total=%d len=%d", total, len(conversations))
	}
	if conversations[0].CustomerName == nil || *conversations[0].CustomerName != "Matta Trisha" {
		t.Fatalf("customer_name = %#v, want Matta Trisha", conversations[0].CustomerName)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversations[0].ID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 || messages[0].SenderDisplayName == nil || *messages[0].SenderDisplayName != "Matta Trisha" {
		t.Fatalf("sender_display_name = %#v, want Matta Trisha", messages)
	}
}

func TestEmailFallbackProcessInboundEmailRouteUsesReplyToForContactFormCustomer(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{
		ID:             "a1111111-1111-1111-1111-111111111112",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-contactform",
		InboundAddress: "inbox@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "website@acme.com", Name: "Acme Contact Form"},
		To:                route.InboundAddress,
		OriginalRecipient: route.InboundAddress,
		Subject:           "Website inquiry",
		MessageID:         "pm-route-contact-form-1",
		StrippedTextReply: "Can someone contact me about pricing?",
		Headers: []model.PostmarkHeader{
			{Name: "Reply-To", Value: "Taylor Visitor <taylor.visitor@example.com>"},
			{Name: "Message-ID", Value: "<contact-form-1@acme.com>"},
		},
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-contact-form-1"}`); err != nil {
		t.Fatalf("process routed inbound email: %v", err)
	}

	resp, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(resp) != 1 {
		t.Fatalf("expected 1 conversation, got total=%d len=%d", total, len(resp))
	}
	conv := resp[0]
	if conv.CustomerEmail == nil || *conv.CustomerEmail != "taylor.visitor@example.com" {
		t.Fatalf("customer email = %#v, want reply-to visitor", conv.CustomerEmail)
	}
	if conv.CustomerName == nil || *conv.CustomerName != "Taylor Visitor" {
		t.Fatalf("customer name = %#v, want reply-to display name", conv.CustomerName)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conv.ID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 email log, got %d", len(logs))
	}
	if logs[0].FromEmail != "website@acme.com" {
		t.Fatalf("from email = %q, want original contact form sender", logs[0].FromEmail)
	}
	if logs[0].ReplyTo != "Taylor Visitor <taylor.visitor@example.com>" {
		t.Fatalf("reply-to = %q, want visitor header", logs[0].ReplyTo)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conv.ID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	detail, err := env.service.supportInboxService.GetMessageEmailDetail(ctx, workspaceID, messages[0].ID)
	if err != nil {
		t.Fatalf("get message email detail: %v", err)
	}
	if detail.ReplyTo != "Taylor Visitor <taylor.visitor@example.com>" {
		t.Fatalf("detail reply-to = %q, want visitor header", detail.ReplyTo)
	}
}

func TestEmailFallbackProcessInboundEmailRouteAcceptsExistingContactFormReplyByReplyTo(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "99999999-9999-9999-9999-999999999991"
	customerEmail := "taylor.visitor@example.com"
	customerName := "Taylor Visitor"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Website inquiry",
		Status:        model.SupportConversationStatusOpen,
		Channel:       "email",
		Source:        "email",
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	outboundMsgID := "<helpin-contact-form-thread@on.helpin.email>"
	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "dddddddd-dddd-dddd-dddd-dddddddddd91",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		Direction:      "outbound",
		RFCMessageID:   outboundMsgID,
		ToEmail:        customerEmail,
		Status:         "sent",
	}); err != nil {
		t.Fatalf("create email log: %v", err)
	}

	route := &model.SupportEmailRoute{
		ID:             "eeeeeeee-eeee-eeee-eeee-eeeeeeeeee91",
		WorkspaceID:    workspaceID,
		InboundAddress: "inbox@acme.on.helpin.email",
		Active:         true,
	}
	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "website@acme.com", Name: "Acme Contact Form"},
		OriginalRecipient: route.InboundAddress,
		To:                route.InboundAddress,
		Subject:           "Re: Website inquiry",
		MessageID:         "pm-route-contact-form-reply",
		TextBody:          "I can meet tomorrow.",
		Headers: []model.PostmarkHeader{
			{Name: "Reply-To", Value: "Taylor Visitor <taylor.visitor@example.com>"},
			{Name: "In-Reply-To", Value: outboundMsgID},
			{Name: "Message-ID", Value: "<contact-form-reply-1@acme.com>"},
		},
	}

	if err := env.service.processInboundRoute(ctx, route, payload, `{"MessageID":"pm-route-contact-form-reply"}`); err != nil {
		t.Fatalf("process routed contact form reply: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected contact form reply to be accepted on existing conversation, got %d messages", len(messages))
	}
	if messages[0].SenderDisplayName == nil || *messages[0].SenderDisplayName != "Taylor Visitor" {
		t.Fatalf("sender display name = %#v, want Taylor Visitor", messages[0].SenderDisplayName)
	}

	detail, err := env.service.supportInboxService.GetMessageEmailDetail(ctx, workspaceID, messages[0].ID)
	if err != nil {
		t.Fatalf("get message email detail: %v", err)
	}
	if detail.FromEmail != "website@acme.com" {
		t.Fatalf("detail from = %q, want raw contact form sender", detail.FromEmail)
	}
	if detail.ReplyTo != "Taylor Visitor <taylor.visitor@example.com>" {
		t.Fatalf("detail reply-to = %q, want visitor header", detail.ReplyTo)
	}
}

func TestEmailFallbackProcessInboundEmailRouteUsesForwardedOriginalSender(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{
		ID:             "a1111111-1111-1111-1111-111111111118",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-forwarded123",
		InboundAddress: "support@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "founder@company.com", Name: "Founder"},
		To:                route.InboundAddress,
		OriginalRecipient: route.InboundAddress,
		Subject:           "Fwd: Billing question",
		MessageID:         "pm-route-forwarded-1",
		TextBody: `Can someone handle this?

---------- Forwarded message ---------
From: Jane Customer <jane@customer.example>
Date: Tue, Jun 2, 2026 at 10:14 AM
Subject: Billing question
To: Founder <founder@company.com>

I need help with my invoice.

--
Jane Customer

--
Founder
Company`,
		HtmlBody: `<div dir="ltr">Can someone handle this?<br><br><div class="gmail_quote gmail_quote_container"><div class="gmail_attr">---------- Forwarded message ---------<br>From: <strong>Jane Customer</strong> &lt;<a href="mailto:jane@customer.example">jane@customer.example</a>&gt;<br>Date: Tue, Jun 2, 2026 at 10:14 AM<br>Subject: Billing question<br>To: Founder &lt;<a href="mailto:founder@company.com">founder@company.com</a>&gt;<br></div><br><div>I need help with my invoice.</div><div class="gmail_signature">Jane Customer</div></div><span class="gmail_signature_prefix">-- </span><br><div class="gmail_signature">Founder<br>Company</div></div>`,
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-forwarded-1"}`); err != nil {
		t.Fatalf("process routed inbound email: %v", err)
	}

	resp, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(resp) != 1 {
		t.Fatalf("expected 1 conversation, got total=%d len=%d", total, len(resp))
	}
	conv := resp[0]
	if conv.CustomerEmail == nil || *conv.CustomerEmail != "jane@customer.example" {
		t.Fatalf("customer email = %#v, want jane@customer.example", conv.CustomerEmail)
	}
	if conv.CustomerName == nil || *conv.CustomerName != "Jane Customer" {
		t.Fatalf("customer name = %#v, want Jane Customer", conv.CustomerName)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conv.ID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %#v", messages)
	}
	if messages[0].SenderDisplayName == nil || *messages[0].SenderDisplayName != "Jane Customer" {
		t.Fatalf("sender display name = %#v, want Jane Customer", messages[0].SenderDisplayName)
	}
	if !strings.Contains(messages[0].Content, "I need help with my invoice.") {
		t.Fatalf("message content did not include forwarded customer body: %q", messages[0].Content)
	}
	if strings.TrimSpace(messages[0].Content) == "Founder\nCompany" || !strings.Contains(messages[0].Content, "Forwarded message") {
		t.Fatalf("message content should be based on forwarded text body, got %q", messages[0].Content)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(messages[0].Metadata), &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["forwarded_by_email"] != "founder@company.com" {
		t.Fatalf("forwarded_by_email metadata = %#v", metadata["forwarded_by_email"])
	}
	if metadata["original_sender_email"] != "jane@customer.example" {
		t.Fatalf("original_sender_email metadata = %#v", metadata["original_sender_email"])
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conv.ID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 email log, got %d", len(logs))
	}
	if logs[0].FromEmail != "founder@company.com" {
		t.Fatalf("log from email = %q, want founder@company.com", logs[0].FromEmail)
	}
	if !strings.Contains(logs[0].EmailVisibleText, "I need help with my invoice.") {
		t.Fatalf("forwarded visible projection lost customer body: %q", logs[0].EmailVisibleText)
	}
	if strings.Contains(logs[0].EmailVisibleText, "Forwarded message") || strings.Contains(logs[0].EmailVisibleText, "From: Jane Customer") {
		t.Fatalf("forwarded visible projection retained attribution headers: %q", logs[0].EmailVisibleText)
	}
	if logs[0].EmailHasQuotedContent || logs[0].EmailQuotedText != "" {
		t.Fatalf("forwarded customer body was incorrectly hidden as history: %#v", logs[0])
	}
	if logs[0].EmailProjectionVersion != inboundhtml.CurrentProjectionVersion {
		t.Fatalf("forwarded projection version = %d, want current", logs[0].EmailProjectionVersion)
	}

	detail, err := env.service.supportInboxService.GetMessageEmailDetail(ctx, workspaceID, messages[0].ID)
	if err != nil {
		t.Fatalf("get message email detail: %v", err)
	}
	if detail == nil || detail.ForwardedAttribution == nil {
		t.Fatalf("expected forwarded attribution in email detail, got %#v", detail)
	}
	if detail.ForwardedAttribution.OriginalSenderEmail != "jane@customer.example" {
		t.Fatalf("email detail original sender = %#v", detail.ForwardedAttribution)
	}
}

func TestEmailFallbackProcessInboundEmailRouteMarksHighSpamScoreAsSpam(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{
		ID:             "a1111111-1111-1111-1111-111111111112",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-spam123",
		InboundAddress: "spam@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "promo@example.com", Name: "Promo"},
		To:                route.InboundAddress,
		OriginalRecipient: route.InboundAddress,
		Subject:           "Special offer",
		MessageID:         "pm-route-spam-1",
		StrippedTextReply: "Buy now",
		Headers: []model.PostmarkHeader{
			{Name: "X-Spam-Status", Value: "No"},
			{Name: "X-Spam-Score", Value: "5.3"},
			{Name: "X-Spam-Tests", Value: "HTML_MESSAGE,LOTS_OF_MONEY"},
		},
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-spam-1"}`); err != nil {
		t.Fatalf("process routed inbound email: %v", err)
	}

	resp, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, model.SupportConversationStatusSpam, "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list spam conversations: %v", err)
	}
	if total != 1 || len(resp) != 1 {
		t.Fatalf("expected 1 spam conversation, got total=%d len=%d", total, len(resp))
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, resp[0].ID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %#v", messages)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(messages[0].Metadata), &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["postmark_spam_status"] != "No" {
		t.Fatalf("expected postmark_spam_status metadata, got %#v", metadata)
	}
	if metadata["postmark_spam_tests"] != "HTML_MESSAGE,LOTS_OF_MONEY" {
		t.Fatalf("expected postmark_spam_tests metadata, got %#v", metadata)
	}
	if score, ok := metadata["postmark_spam_score"].(float64); !ok || score != 5.3 {
		t.Fatalf("expected postmark_spam_score=5.3, got %#v", metadata["postmark_spam_score"])
	}
}

func TestEmailFallbackProcessInboundEmailVerifiesSenderForwarding(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	sender := verifiedSender(workspaceID, nil, "support@example.com", "none", "22222222-2222-2222-2222-222222222222")
	sender.ForwardingVerificationToken = "abc123"
	sender.ForwardingAddress = "verify-abc123@on.helpin.email"
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	if err := env.senderRepo.Create(ctx, sender); err != nil {
		t.Fatalf("create sender: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MailboxHash:       "verify-abc123",
		OriginalRecipient: sender.ForwardingAddress,
		To:                sender.Email,
		FromFull:          model.PostmarkAddress{Email: "customer@example.com", Name: "Taylor"},
		Subject:           "Forwarding test",
		MessageID:         "pm-sender-verify-1",
		StrippedTextReply: "Testing forwarding",
		Headers: []model.PostmarkHeader{
			{Name: "To", Value: sender.Email},
		},
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-sender-verify-1"}`); err != nil {
		t.Fatalf("process sender verification email: %v", err)
	}

	updated, err := env.senderRepo.GetByID(ctx, workspaceID, sender.ID)
	if err != nil {
		t.Fatalf("reload sender: %v", err)
	}
	if updated == nil || updated.ForwardingStatus != supportEmailSenderForwardingVerified || updated.ForwardingVerifiedAt == nil {
		t.Fatalf("expected sender forwarding verified, got %#v", updated)
	}

	_, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 0 {
		t.Fatalf("verification email should not create conversation, got total=%d", total)
	}
}

func TestEmailFallbackProcessInboundEmailVerifiesSenderForwardingFromRecipientAddress(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	sender := verifiedSender(workspaceID, nil, "support@example.com", "none", "22222222-2222-2222-2222-222222222222")
	sender.ForwardingVerificationToken = "abc123"
	sender.ForwardingAddress = "verify-abc123@on.helpin.email"
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	if err := env.senderRepo.Create(ctx, sender); err != nil {
		t.Fatalf("create sender: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		OriginalRecipient: "Forward Verify <" + sender.ForwardingAddress + ">",
		ToFull: []model.PostmarkAddress{
			{Email: sender.Email, Name: "Support"},
		},
		FromFull:  model.PostmarkAddress{Email: "customer@example.com", Name: "Taylor"},
		Subject:   "Forwarding test",
		MessageID: "pm-sender-verify-from-recipient-1",
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-sender-verify-from-recipient-1"}`); err != nil {
		t.Fatalf("process sender verification email: %v", err)
	}

	updated, err := env.senderRepo.GetByID(ctx, workspaceID, sender.ID)
	if err != nil {
		t.Fatalf("reload sender: %v", err)
	}
	if updated == nil || updated.ForwardingStatus != supportEmailSenderForwardingVerified || updated.ForwardingVerifiedAt == nil {
		t.Fatalf("expected sender forwarding verified from recipient address, got %#v", updated)
	}
}

func TestEmailFallbackProcessInboundEmailMarksSenderForwardingFailedWithoutSenderEvidence(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	sender := verifiedSender(workspaceID, nil, "support@example.com", "none", "22222222-2222-2222-2222-222222222222")
	sender.ForwardingVerificationToken = "abc123"
	sender.ForwardingAddress = "verify-abc123@on.helpin.email"
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	if err := env.senderRepo.Create(ctx, sender); err != nil {
		t.Fatalf("create sender: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		OriginalRecipient: sender.ForwardingAddress,
		To:                "not-support@example.com",
		FromFull:          model.PostmarkAddress{Email: "customer@example.com", Name: "Taylor"},
		Subject:           "Forwarding test",
		MessageID:         "pm-sender-verify-failed-1",
		Headers: []model.PostmarkHeader{
			{Name: "X-Forwarded-To", Value: "not-support@example.com"},
		},
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-sender-verify-failed-1"}`); err != nil {
		t.Fatalf("process sender verification email: %v", err)
	}

	updated, err := env.senderRepo.GetByID(ctx, workspaceID, sender.ID)
	if err != nil {
		t.Fatalf("reload sender: %v", err)
	}
	if updated == nil || updated.ForwardingStatus != supportEmailSenderForwardingFailed || updated.ForwardingLastError == nil {
		t.Fatalf("expected sender forwarding failed, got %#v", updated)
	}

	_, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 0 {
		t.Fatalf("failed verification email should not create conversation, got total=%d", total)
	}
}

func TestEmailFallbackConsumesReturnedEmailRouteVerificationTest(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	sourceAddress := "support@example.com"
	sentAt := time.Now().UTC().Add(-time.Minute)
	route := &model.SupportEmailRoute{
		WorkspaceID:                 workspaceID,
		RouteKey:                    "route-forward-test",
		InboundAddress:              "inbox@acme.on.helpin.email",
		SourceAddress:               &sourceAddress,
		ProviderType:                "forwarding",
		Active:                      true,
		VerificationSentAt:          &sentAt,
		ForwardingVerificationToken: "test-token",
		CreatedByID:                 actorID,
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MailboxHash:       route.RouteKey,
		OriginalRecipient: route.InboundAddress,
		To:                sourceAddress,
		FromFull:          model.PostmarkAddress{Email: "noreply@example.com", Name: "Helpin"},
		Subject:           "Helpin forwarding test [test-token]",
		MessageID:         "pm-route-test-returned",
		TextBody:          "No action is required.",
		Headers:           []model.PostmarkHeader{{Name: "To", Value: sourceAddress}},
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-test-returned"}`); err != nil {
		t.Fatalf("process returned forwarding test: %v", err)
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-test-returned"}`); err != nil {
		t.Fatalf("process duplicate returned forwarding test: %v", err)
	}

	updated, err := env.routeRepo.GetByID(ctx, workspaceID, route.ID)
	if err != nil {
		t.Fatalf("reload route: %v", err)
	}
	if updated.ForwardingVerifiedAt == nil || updated.ForwardingVerificationToken != "" {
		t.Fatalf("expected verified route with consumed token, got %#v", updated)
	}
	_, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 0 {
		t.Fatalf("forwarding test should not create a conversation, got total=%d", total)
	}
}

func TestEmailFallbackKeepsProviderConfirmationPendingAndVisible(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())

	workspaceID := "11111111-1111-1111-1111-111111111111"
	sourceAddress := "support@example.com"
	route := &model.SupportEmailRoute{
		WorkspaceID:    workspaceID,
		RouteKey:       "route-provider-confirmation",
		InboundAddress: "inbox@acme.on.helpin.email",
		SourceAddress:  &sourceAddress,
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MailboxHash:       route.RouteKey,
		OriginalRecipient: route.InboundAddress,
		To:                route.InboundAddress,
		FromFull:          model.PostmarkAddress{Email: "forwarding-noreply@google.com", Name: "Gmail Team"},
		Subject:           "Gmail Forwarding Confirmation - Receive Mail from support@example.com",
		MessageID:         "pm-provider-confirmation",
		TextBody:          "support@example.com requested forwarding. Confirm at https://mail-settings.google.com/mail/vf-token",
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-provider-confirmation"}`); err != nil {
		t.Fatalf("process provider confirmation: %v", err)
	}

	updated, err := env.routeRepo.GetByID(ctx, workspaceID, route.ID)
	if err != nil {
		t.Fatalf("reload route: %v", err)
	}
	if updated.ConfirmationReceivedAt == nil || updated.ForwardingVerifiedAt != nil {
		t.Fatalf("expected confirmation received but forwarding pending, got %#v", updated)
	}
	if updated.ConfirmationConversationID == nil || strings.TrimSpace(*updated.ConfirmationConversationID) == "" {
		t.Fatalf("expected confirmation conversation to be retained, got %#v", updated)
	}
	_, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 {
		t.Fatalf("provider confirmation should remain visible as a conversation, got total=%d", total)
	}
	conversation, err := env.service.findConversationByID(ctx, *updated.ConfirmationConversationID)
	if err != nil || conversation == nil {
		t.Fatalf("find retained confirmation conversation = %#v, %v", conversation, err)
	}
}

func TestEmailFallbackQualifyingCustomerMailVerifiesRoute(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())

	workspaceID := "11111111-1111-1111-1111-111111111111"
	sourceAddress := "support@example.com"
	route := &model.SupportEmailRoute{
		WorkspaceID:    workspaceID,
		RouteKey:       "route-customer-proof",
		InboundAddress: "inbox@acme.on.helpin.email",
		SourceAddress:  &sourceAddress,
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MailboxHash:       route.RouteKey,
		OriginalRecipient: route.InboundAddress,
		To:                sourceAddress,
		FromFull:          model.PostmarkAddress{Email: "customer@example.net", Name: "Customer"},
		Subject:           "Need help",
		MessageID:         "pm-customer-proof",
		TextBody:          "Please help with my account.",
		Headers:           []model.PostmarkHeader{{Name: "To", Value: sourceAddress}},
	}
	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-customer-proof"}`); err != nil {
		t.Fatalf("process customer mail: %v", err)
	}

	updated, err := env.routeRepo.GetByID(ctx, workspaceID, route.ID)
	if err != nil {
		t.Fatalf("reload route: %v", err)
	}
	if updated.ForwardingVerifiedAt == nil {
		t.Fatalf("expected qualifying customer mail to verify route, got %#v", updated)
	}
	_, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 {
		t.Fatalf("customer mail should create a conversation, got total=%d", total)
	}
}

func TestEmailFallbackProcessInboundEmailRouteThreadsReply(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "99999999-9999-9999-9999-999999999999"
	customerEmail := "customer@example.com"
	customerName := "Taylor"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Existing email thread",
		Status:        "open",
		Channel:       "email",
		Source:        "email",
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	outboundLog := &model.SupportEmailLog{
		ID:             "b2222222-2222-2222-2222-222222222222",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		Direction:      "outbound",
		FromEmail:      "support@example.com",
		ToEmail:        customerEmail,
		Subject:        "Existing email thread",
		RFCMessageID:   "<helpin-thread-123@replies.helpin.ai>",
		Status:         "sent",
	}
	if err := env.emailLogRepo.Create(ctx, outboundLog); err != nil {
		t.Fatalf("create outbound log: %v", err)
	}

	route := &model.SupportEmailRoute{
		ID:             "c3333333-3333-3333-3333-333333333333",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-thread123",
		InboundAddress: "billing@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: customerEmail, Name: customerName},
		To:                route.InboundAddress,
		OriginalRecipient: route.InboundAddress,
		Subject:           "Re: Existing email thread",
		MessageID:         "pm-route-reply-1",
		StrippedTextReply: "Following up on this thread.",
		Headers: []model.PostmarkHeader{
			{Name: "Message-ID", Value: "<customer-reply-2@example.com>"},
			{Name: "In-Reply-To", Value: "<helpin-thread-123@replies.helpin.ai>"},
			{Name: "X-Spam-Status", Value: "Yes"},
			{Name: "X-Spam-Score", Value: "9.1"},
		},
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-reply-1"}`); err != nil {
		t.Fatalf("process routed reply: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected threaded reply to stay in existing conversation, got %d messages", len(messages))
	}

	resp, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(resp) != 1 || resp[0].ID != conversationID {
		t.Fatalf("expected reply to stay on original conversation, got %#v total=%d", resp, total)
	}
	if resp[0].Status != model.SupportConversationStatusOpen {
		t.Fatalf("expected high-score reply not to mark existing conversation spam, got %q", resp[0].Status)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(messages[0].Metadata), &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["postmark_spam_status"] != "Yes" {
		t.Fatalf("expected postmark spam metadata on reply, got %#v", metadata)
	}
}

func TestProcessInboundRouteThreadsByRFCHeaders(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.SupportInboxSettings{})
	workspaceID := "11111111-1111-1111-1111-111111111111"
	customerEmail := "buyer@example.com"
	conversationID := "cccccccc-cccc-cccc-cccc-cccccccccccc"

	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Status:        model.SupportConversationStatusOpen,
		CustomerEmail: &customerEmail,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	outboundMsgID := "<helpin-rfc-route-thread@on.helpin.email>"
	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "dddddddd-dddd-dddd-dddd-dddddddddddd",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		Direction:      "outbound",
		RFCMessageID:   outboundMsgID,
		ToEmail:        customerEmail,
		Status:         "sent",
	}); err != nil {
		t.Fatalf("create email log: %v", err)
	}

	route := &model.SupportEmailRoute{
		ID:             "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
		WorkspaceID:    workspaceID,
		InboundAddress: "inbox@acme.on.helpin.email",
		Active:         true,
	}
	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: customerEmail},
		OriginalRecipient: route.InboundAddress,
		To:                route.InboundAddress,
		Subject:           "Re: Support conversation",
		MessageID:         "pm-route-rfc-thread",
		TextBody:          "Thanks, that worked!",
		Headers: []model.PostmarkHeader{
			{Name: "In-Reply-To", Value: outboundMsgID},
		},
	}

	if err := env.service.processInboundRoute(ctx, route, payload, `{"MessageID":"pm-route-rfc-thread"}`); err != nil {
		t.Fatalf("processInboundRoute: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected inbound message on original conversation, got %d", len(messages))
	}

	_, total, err := env.convRepo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "", ""))
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected no new conversation, got total=%d", total)
	}
}

func TestProcessInboundRouteThreadsActiveTeammatePersonalInboxReply(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.SupportInboxSettings{})
	workspaceID := "11111111-1111-1111-1111-111111111111"
	ownerID := "22222222-2222-2222-2222-222222222222"
	customerEmail := "buyer@example.com"
	conversationID := "abababab-abab-abab-abab-abababababab"

	if _, err := env.service.workspaceRepo.AddMember(ctx, workspaceID, ownerID, model.RoleOwner); err != nil {
		t.Fatalf("add active workspace member: %v", err)
	}
	resolvedAt := time.Now().UTC().Add(-time.Hour)
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Status:        model.SupportConversationStatusResolved,
		CustomerEmail: &customerEmail,
		ResolvedAt:    &resolvedAt,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	previousRFCMessageID := "<customer-thread-message@example.com>"
	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID:             "cdcdcdcd-cdcd-cdcd-cdcd-cdcdcdcdcdcd",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		Direction:      "inbound",
		RFCMessageID:   previousRFCMessageID,
		FromEmail:      customerEmail,
		Status:         "sent",
	}); err != nil {
		t.Fatalf("create prior email log: %v", err)
	}

	route := &model.SupportEmailRoute{
		ID:             "dededede-dede-dede-dede-dededededede",
		WorkspaceID:    workspaceID,
		InboundAddress: "inbox@acme.on.helpin.email",
		Active:         true,
	}
	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "owner@example.com", Name: "Owner"},
		To:                "Buyer <buyer@example.com>",
		ToFull:            []model.PostmarkAddress{{Email: customerEmail, Name: "Buyer"}},
		Cc:                "Support <support@example.com>",
		CcFull:            []model.PostmarkAddress{{Email: "support@example.com", Name: "Support"}},
		OriginalRecipient: route.InboundAddress,
		MailboxHash:       route.ID,
		Subject:           "Re: Support conversation",
		MessageID:         "pm-teammate-personal-reply",
		Date:              "Thu, 03 Sep 2026 07:35:00 +0000",
		TextBody:          "I sent this directly from my personal inbox.",
		Headers: []model.PostmarkHeader{
			{Name: "Message-ID", Value: "<teammate-personal-reply@example.com>"},
			{Name: "In-Reply-To", Value: previousRFCMessageID},
			{Name: "References", Value: previousRFCMessageID},
		},
	}

	if err := env.service.processInboundRoute(ctx, route, payload, `{"MessageID":"pm-teammate-personal-reply"}`); err != nil {
		t.Fatalf("process teammate personal inbox reply: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one teammate reply on the original conversation, got %d", len(messages))
	}
	if messages[0].SenderType != "user" || messages[0].SenderUserID == nil || *messages[0].SenderUserID != ownerID {
		t.Fatalf("expected reply attributed to teammate %s, got sender_type=%q sender_user_id=%v", ownerID, messages[0].SenderType, messages[0].SenderUserID)
	}
	if messages[0].SenderDisplayName == nil || *messages[0].SenderDisplayName != "Owner" {
		t.Fatalf("expected teammate display name, got %v", messages[0].SenderDisplayName)
	}
	if messages[0].ViaChannel == nil || *messages[0].ViaChannel != "email" {
		t.Fatalf("expected email channel, got %v", messages[0].ViaChannel)
	}
	var messageMetadata map[string]any
	if err := json.Unmarshal([]byte(messages[0].Metadata), &messageMetadata); err != nil {
		t.Fatalf("unmarshal teammate reply metadata: %v", err)
	}
	if messageMetadata["external_email_reply"] != true || messageMetadata["external_email_capture"] != "support_email_copy" {
		t.Fatalf("expected external email provenance metadata, got %#v", messageMetadata)
	}
	wantSentAt := time.Date(2026, 9, 3, 7, 35, 0, 0, time.UTC)
	if !messages[0].CreatedAt.Equal(wantSentAt) {
		t.Fatalf("created_at = %v, want external sent time %v", messages[0].CreatedAt, wantSentAt)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 2 || logs[1].Direction != "outbound" || logs[1].Status != "external" {
		t.Fatalf("expected copied teammate reply recorded as outbound, got %#v", logs)
	}

	updated, err := env.convRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.Status != model.SupportConversationStatusWaitingOnCustomer {
		t.Fatalf("expected resolved conversation to wait on customer, got %q", updated.Status)
	}
	if updated.OpenedByUserID == nil || *updated.OpenedByUserID != ownerID || updated.HumanTakeover == nil || !*updated.HumanTakeover {
		t.Fatalf("expected teammate ownership and human takeover, got opened_by=%v human_takeover=%v", updated.OpenedByUserID, updated.HumanTakeover)
	}

	forwardOnlyPayload := payload
	forwardOnlyPayload.MessageID = "pm-teammate-forward-to-support"
	forwardOnlyPayload.To = route.InboundAddress
	forwardOnlyPayload.ToFull = []model.PostmarkAddress{{Email: route.InboundAddress}}
	forwardOnlyPayload.Cc = ""
	forwardOnlyPayload.CcFull = nil
	forwardOnlyPayload.TextBody = "Forwarding this internally without the customer copied."
	forwardOnlyPayload.Headers = []model.PostmarkHeader{
		{Name: "Message-ID", Value: "<teammate-forward-to-support@example.com>"},
		{Name: "In-Reply-To", Value: previousRFCMessageID},
	}
	if err := env.service.processInboundRoute(ctx, route, forwardOnlyPayload, `{"MessageID":"pm-teammate-forward-to-support"}`); err != nil {
		t.Fatalf("process teammate forward without customer recipient: %v", err)
	}
	messages, err = env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		t.Fatalf("list messages after teammate forward: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected internal forward without customer recipient to be ignored, got %d messages", len(messages))
	}
}

func TestInboundUnrelatedEmailCreatesNewConversation(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	ws := "11111111-1111-1111-1111-111111111111"
	route := &model.SupportEmailRoute{ID: uuid.NewString(), WorkspaceID: ws, RouteKey: "route-unrelated", InboundAddress: "inbox@acme.on.helpin.email", ProviderType: "forwarding", Active: true, CreatedByID: "22222222-2222-2222-2222-222222222222"}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatal(err)
	}
	old := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: ws, Subject: "Old issue", Status: "open", CustomerEmail: strPtr("customer@example.com")}
	if err := env.convRepo.Create(ctx, old); err != nil {
		t.Fatal(err)
	}
	payload := model.PostmarkInboundPayload{MessageID: "new-issue", FromFull: model.PostmarkAddress{Email: "customer@example.com"}, To: route.InboundAddress, Subject: "Different issue", TextBody: "A new question"}
	if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
		t.Fatal(err)
	}
	var n int64
	env.convRepo.DB().Model(&model.SupportConversation{}).Where("workspace_id = ?", ws).Count(&n)
	if n != 2 {
		t.Fatalf("unrelated email grouped into old conversation: %d", n)
	}
}

func TestEmailFallbackProcessInboundEmailRouteThreadsForwardedReplyFromOriginalSender(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "99999999-9999-9999-9999-999999999998"
	customerEmail := "jason@the-web-dev.com"
	customerName := "Jason Smith"
	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Data Export",
		Status:        "open",
		Channel:       "email",
		Source:        "email",
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	threadLog := &model.SupportEmailLog{
		ID:             "b2222222-2222-2222-2222-222222222229",
		WorkspaceID:    workspaceID,
		ConversationID: conversationID,
		Direction:      "inbound",
		FromEmail:      customerEmail,
		ToEmail:        "waqar@usermaven.com",
		Subject:        "Data Export",
		RFCMessageID:   "<CAGW4uJr8a1GgA=7UEudEYTZj-xL85o1kyA1gPboMQyrw3=Z2iQ@mail.gmail.com>",
		Status:         "sent",
	}
	if err := env.emailLogRepo.Create(ctx, threadLog); err != nil {
		t.Fatalf("create thread log: %v", err)
	}

	route := &model.SupportEmailRoute{
		ID:             "c3333333-3333-3333-3333-333333333338",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-threadfwd",
		InboundAddress: "support@usermaven.com",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    "22222222-2222-2222-2222-222222222222",
	}
	if err := env.routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		FromFull:          model.PostmarkAddress{Email: "waqar@usermaven.com", Name: "Waqar Azeem"},
		To:                route.InboundAddress,
		OriginalRecipient: route.InboundAddress,
		Subject:           "Fwd: Data Export",
		MessageID:         "pm-route-forwarded-thread-1",
		StrippedTextReply: "---------- Forwarded message ---------\n\nData Export",
		TextBody: `---------- Forwarded message ---------
From: Jason Smith <jason@the-web-dev.com>
Date: Sun, May 31, 2026 at 2:38 PM
Subject: Data Export
To: Waqar from Usermaven <waqar@usermaven.com>

Can I export my data?`,
		Headers: []model.PostmarkHeader{
			{Name: "Message-ID", Value: "<CAKs2i=G6cwV+L7jpWNuFsbJrYQwTRPryJcmTmp32HK+2JKCy-A@mail.gmail.com>"},
			{Name: "In-Reply-To", Value: "<CAGW4uJr8a1GgA=7UEudEYTZj-xL85o1kyA1gPboMQyrw3=Z2iQ@mail.gmail.com>"},
			{Name: "References", Value: "<CAGW4uJr8a1GgA=7UEudEYTZj-xL85o1kyA1gPboMQyrw3=Z2iQ@mail.gmail.com>"},
		},
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-route-forwarded-thread-1"}`); err != nil {
		t.Fatalf("process routed forwarded reply: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, false)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected forwarded reply to stay in existing conversation, got %d messages", len(messages))
	}
	if messages[0].SenderDisplayName == nil || *messages[0].SenderDisplayName != customerName {
		t.Fatalf("sender display name = %#v, want %q", messages[0].SenderDisplayName, customerName)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(messages[0].Metadata), &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["forwarded_by_email"] != "waqar@usermaven.com" {
		t.Fatalf("forwarded_by_email metadata = %#v", metadata["forwarded_by_email"])
	}
	if metadata["original_sender_email"] != customerEmail {
		t.Fatalf("original_sender_email metadata = %#v", metadata["original_sender_email"])
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected original thread log and forwarded inbound log, got %d", len(logs))
	}
	if logs[1].FromEmail != "waqar@usermaven.com" {
		t.Fatalf("log from email = %q, want waqar@usermaven.com", logs[1].FromEmail)
	}
}

func TestEmailFallbackRenderBodiesOmitsUnsubscribeLink(t *testing.T) {
	svc := &EmailFallbackService{}
	htmlBody, textBody := svc.renderBodies(
		[]model.SupportMessage{{Content: "Thanks for reaching out."}},
		"Alex Agent",
		"Acme Support",
		"https://example.com/#helpin-conv=conv-1",
		"unsubscribe-conv-1@replies.helpin.ai",
	)

	if strings.Contains(htmlBody, "unsubscribe-conv-1@replies.helpin.ai") || strings.Contains(strings.ToLower(htmlBody), "unsubscribe") {
		t.Fatalf("expected html body to omit unsubscribe link, got %q", htmlBody)
	}
	if strings.Contains(htmlBody, "max-width:600px") {
		t.Fatalf("expected plain html email body without template wrapper, got %q", htmlBody)
	}
	if strings.Contains(textBody, "unsubscribe-conv-1@replies.helpin.ai") || strings.Contains(strings.ToLower(textBody), "unsubscribe") {
		t.Fatalf("expected text body to omit unsubscribe link, got %q", textBody)
	}
}

func TestEmailFallbackRenderBodiesUsesLinkedChatAndHelpinAttribution(t *testing.T) {
	svc := &EmailFallbackService{}
	htmlBody, textBody := svc.renderBodies(
		[]model.SupportMessage{{Content: "Thanks for reaching out."}},
		"Alex Agent",
		"Acme Support",
		"https://example.com/#helpin-conv=conv-1",
		"",
	)

	if !strings.Contains(htmlBody, `<a href="https://example.com/#helpin-conv=conv-1">View conversation in browser</a>`) {
		t.Fatalf("expected html body to link open the chat text, got %q", htmlBody)
	}
	if strings.Contains(htmlBody, `>https://example.com/#helpin-conv=conv-1</a>`) {
		t.Fatalf("expected html body not to expose raw chat URL as anchor text, got %q", htmlBody)
	}
	if !strings.Contains(htmlBody, `border-top:1px solid #e5e7eb`) {
		t.Fatalf("expected subtle bordered attribution footer, got %q", htmlBody)
	}
	wantURL := "https://helpin.ai/?utm_source=acme-support&utm_medium=email&utm_campaign=powered_by_helpin&utm_content=support_email_footer"
	if !strings.Contains(htmlBody, `<a href="`+html.EscapeString(wantURL)+`"`) || !strings.Contains(htmlBody, `<strong>Helpin AI</strong></a>`) {
		t.Fatalf("expected Helpin AI attribution link, got %q", htmlBody)
	}
	if !strings.Contains(textBody, "Powered by Helpin AI: "+wantURL) {
		t.Fatalf("expected plaintext Helpin AI attribution URL, got %q", textBody)
	}
}

func TestEmailFallbackInboundPayloadBodiesStripReplyDelimiterHistory(t *testing.T) {
	content, _ := inboundPayloadBodies(model.PostmarkInboundPayload{
		StrippedTextReply: "Fresh customer reply.\n\n" + supportEmailReplyDelimiter + "\n\nOld quoted body",
	})

	if content != "Fresh customer reply." {
		t.Fatalf("content = %q, want fresh reply only", content)
	}
}

func TestEmailFallbackInboundPayloadBodiesStripReplyDelimiterFromTextFallback(t *testing.T) {
	content, _ := inboundPayloadBodies(model.PostmarkInboundPayload{
		TextBody: "Fresh fallback reply.\n\n" + supportEmailReplyDelimiter + "\n\nOld quoted body",
	})

	if content != "Fresh fallback reply." {
		t.Fatalf("content = %q, want fresh reply only", content)
	}
}

func TestEmailFallbackInboundPayloadBodiesStripReplyDelimiterFromHTMLMarkdown(t *testing.T) {
	content, _ := inboundPayloadBodies(model.PostmarkInboundPayload{
		HtmlBody: "<p>Fresh HTML reply.</p><p>" + supportEmailReplyDelimiter + "</p><p>Old quoted body</p>",
	})

	if strings.Contains(content, supportEmailReplyDelimiter) || strings.Contains(content, "Old quoted body") {
		t.Fatalf("expected delimiter history stripped, got %q", content)
	}
	if !strings.Contains(content, "Fresh HTML reply.") {
		t.Fatalf("expected fresh reply, got %q", content)
	}
}

func TestInboundPayloadProjectionSeparatesQuotedHistory(t *testing.T) {
	projection := inboundPayloadProjection(model.PostmarkInboundPayload{
		HtmlBody: `<p>Fresh customer reply.</p><div class="gmail_quote"><p>Old quoted body.</p></div>`,
		TextBody: "Fresh customer reply.\n\nOld quoted body.",
	})

	if projection.VisibleText != "Fresh customer reply." {
		t.Fatalf("visible text = %q, want fresh reply", projection.VisibleText)
	}
	if !projection.HasQuotedContent || !strings.Contains(projection.QuotedText, "Old quoted body.") {
		t.Fatalf("expected retained quoted history, got %#v", projection)
	}
	if projection.Version != inboundhtml.CurrentProjectionVersion {
		t.Fatalf("projection version = %d, want %d", projection.Version, inboundhtml.CurrentProjectionVersion)
	}
}

func TestSupportEmailNotificationPreviewPrefersFreshProviderReply(t *testing.T) {
	payload := model.PostmarkInboundPayload{
		HtmlBody:          `<p>This worked - thanks!</p><table><tr><td>Old quoted reply</td></tr></table>`,
		StrippedTextReply: "This worked - thanks!",
	}
	projection := inboundPayloadProjection(payload)
	if got := supportEmailNotificationPreview(payload, projection); got != "This worked - thanks!" {
		t.Fatalf("notification preview = %q, want fresh reply only", got)
	}
}

func TestSupportEmailNotificationPreviewOmitsAmbiguousHistory(t *testing.T) {
	payload := model.PostmarkInboundPayload{
		HtmlBody: `<p>This worked - thanks!</p><table><tr><td>Old quoted reply</td></tr></table>`,
	}
	projection := inboundPayloadProjection(payload)
	if got := supportEmailNotificationPreview(payload, projection); got != "" {
		t.Fatalf("notification preview = %q, want no ambiguous excerpt", got)
	}
}

func TestIsEmailFallbackTerminalStatus(t *testing.T) {
	if !isEmailFallbackTerminalStatus("closed") {
		t.Fatal("legacy closed alias should still be terminal")
	}
	if !isEmailFallbackTerminalStatus("resolved") {
		t.Fatal("resolved should be terminal")
	}
	if !isEmailFallbackTerminalStatus("spam") {
		t.Fatal("spam should be terminal")
	}
	if isEmailFallbackTerminalStatus("open") {
		t.Fatal("open should not be terminal")
	}
}

func TestIsEmailFallbackInboundTerminalStatus(t *testing.T) {
	if isEmailFallbackInboundTerminalStatus("resolved") {
		t.Fatal("resolved should accept inbound replies and reopen")
	}
	if !isEmailFallbackInboundTerminalStatus("spam") {
		t.Fatal("spam should be terminal")
	}
	if isEmailFallbackInboundTerminalStatus("open") {
		t.Fatal("open should not be terminal")
	}
}

func TestEmailFallbackProcessInboundEmailReopensResolvedConversation(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)
	ctx := context.Background()

	workspaceID := "11111111-1111-1111-1111-111111111111"
	customerEmail := "customer@example.com"
	customerName := "Customer"
	resolvedAt := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)
	flowState := model.SupportConversationFlowStateResolvedByHuman
	conversationID := "33333333-3333-3333-3333-333333333333"
	conversation := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Billing help",
		Status:        model.SupportConversationStatusResolved,
		FlowState:     &flowState,
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
		Source:        "email",
		ResolvedAt:    &resolvedAt,
	}
	if err := env.convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create resolved conversation: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MessageID:         "pm-reopen-1",
		MessageStream:     "inbound",
		OriginalRecipient: "conv-" + conversationID + "@replies.helpin.ai",
		To:                "conv-" + conversationID + "@replies.helpin.ai",
		From:              customerEmail,
		FromFull:          model.PostmarkAddress{Name: customerName, Email: customerEmail},
		Subject:           "Re: Billing help",
		StrippedTextReply: "I still need help with this invoice.",
	}
	rawPayload := `{"MessageStream":"inbound","MessageID":"pm-reopen-1"}`

	if err := env.service.ProcessInboundEmail(ctx, payload, rawPayload); err != nil {
		t.Fatalf("process inbound email: %v", err)
	}

	updated, err := env.convRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load updated conversation: %v", err)
	}
	if updated.Status != model.SupportConversationStatusOpen {
		t.Fatalf("status = %q, want open", updated.Status)
	}
	if updated.ResolvedAt != nil {
		t.Fatalf("resolved_at = %v, want nil", updated.ResolvedAt)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("closed_at = %v, want nil", updated.ClosedAt)
	}
	if updated.FlowState == nil || *updated.FlowState == model.SupportConversationFlowStateResolvedByHuman {
		t.Fatalf("flow_state was not restored: %#v", updated.FlowState)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	var customerReplies, reopenedEvents int
	for _, msg := range messages {
		if msg.SenderType == "customer" && msg.MessageType == "reply" && strings.Contains(msg.Content, "invoice") {
			customerReplies++
		}
		if msg.MessageType == "system" && msg.SystemEventType != nil && *msg.SystemEventType == string(model.SystemEventReopened) {
			reopenedEvents++
			if !msg.IsInternal {
				t.Fatal("reopened system message should be internal")
			}
		}
	}
	if customerReplies != 1 {
		t.Fatalf("customer replies = %d, want 1", customerReplies)
	}
	if reopenedEvents != 1 {
		t.Fatalf("reopened system events = %d, want 1", reopenedEvents)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Direction != "inbound" {
		t.Fatalf("expected one inbound email log, got %+v", logs)
	}
}

func TestEmailFallbackProcessInboundEmailReopensWaitingConversation(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)
	ctx := context.Background()

	workspaceID := "11111111-1111-1111-1111-111111111111"
	assignedUserID := "22222222-2222-2222-2222-222222222222"
	customerEmail := "customer@example.com"
	customerName := "Customer"
	flowState := model.SupportConversationFlowStateWaitingForHuman
	conversationID := "55555555-5555-5555-5555-555555555555"
	conversation := &model.SupportConversation{
		ID:             conversationID,
		WorkspaceID:    workspaceID,
		Subject:        "Plan question",
		Status:         model.SupportConversationStatusWaitingOnCustomer,
		FlowState:      &flowState,
		AssignedUserID: &assignedUserID,
		CustomerEmail:  &customerEmail,
		CustomerName:   &customerName,
		Source:         "email",
	}
	if err := env.convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create waiting conversation: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MessageID:         "pm-waiting-reopen-1",
		MessageStream:     "inbound",
		OriginalRecipient: "conv-" + conversationID + "@replies.helpin.ai",
		To:                "conv-" + conversationID + "@replies.helpin.ai",
		From:              customerEmail,
		FromFull:          model.PostmarkAddress{Name: customerName, Email: customerEmail},
		Subject:           "Re: Plan question",
		StrippedTextReply: "Here is the info you asked for.",
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-waiting-reopen-1"}`); err != nil {
		t.Fatalf("process inbound email: %v", err)
	}

	updated, err := env.convRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load updated conversation: %v", err)
	}
	if updated.Status != model.SupportConversationStatusOpen {
		t.Fatalf("status = %q, want open", updated.Status)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("closed_at = %v, want nil", updated.ClosedAt)
	}
	if updated.FlowState == nil || *updated.FlowState != model.SupportConversationFlowStateAssignedToHuman {
		t.Fatalf("flow_state = %#v, want %q", updated.FlowState, model.SupportConversationFlowStateAssignedToHuman)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	for _, msg := range messages {
		if msg.MessageType == "system" && msg.SystemEventType != nil && *msg.SystemEventType == string(model.SystemEventReopened) {
			t.Fatal("waiting customer reply should not add a reopened system event")
		}
	}
}

func TestEmailFallbackProcessInboundEmailIgnoresSpamConversation(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)
	ctx := context.Background()

	workspaceID := "11111111-1111-1111-1111-111111111111"
	customerEmail := "customer@example.com"
	conversationID := "44444444-4444-4444-4444-444444444444"
	conversation := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Spam",
		Status:        model.SupportConversationStatusSpam,
		CustomerEmail: &customerEmail,
		Source:        "email",
	}
	if err := env.convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create spam conversation: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MessageID:         "pm-spam-1",
		MessageStream:     "inbound",
		OriginalRecipient: "conv-" + conversationID + "@replies.helpin.ai",
		To:                "conv-" + conversationID + "@replies.helpin.ai",
		From:              customerEmail,
		FromFull:          model.PostmarkAddress{Email: customerEmail},
		Subject:           "Re: Spam",
		StrippedTextReply: "Why was this marked spam?",
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-spam-1"}`); err != nil {
		t.Fatalf("process inbound spam reply: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages = %d, want 0", len(messages))
	}
}

func TestCleanForwardedEmailProjectionTextWrappedHeaders(t *testing.T) {
	input := `---------- Forwarded message ---------
From: Justin Staples <justin@js-interactive.com>
Date: Thu, Sep 24, 2026 at 4:01 AM
Subject: Re: Monthly Dashboard Report — Google Search Console (
coloradoelectricalengineering.com)
To: Waqar from Usermaven <waqar@usermaven.com>, Cassie Christman <
cassie@js-interactive.com>

Hi Waqar,
This dashboard hasn't shown data for some time now.`
	want := "Hi Waqar,\nThis dashboard hasn't shown data for some time now."
	if got := cleanForwardedEmailProjectionText(input); got != want {
		t.Fatalf("cleanForwardedEmailProjectionText() = %q, want %q", got, want)
	}
}
