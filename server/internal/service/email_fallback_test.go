package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type capturedPostmarkRequest struct {
	From     string `json:"From"`
	To       string `json:"To"`
	Subject  string `json:"Subject"`
	HtmlBody string `json:"HtmlBody"`
	TextBody string `json:"TextBody"`
	ReplyTo  string `json:"ReplyTo"`
	Headers  []struct {
		Name  string `json:"Name"`
		Value string `json:"Value"`
	} `json:"Headers"`
}

type emailFallbackTestEnv struct {
	redis        *redis.Client
	redisServer  *miniredis.Miniredis
	service      *EmailFallbackService
	messageRepo  *repository.SupportMessageRepository
	convRepo     *repository.SupportConversationRepository
	emailLogRepo *repository.SupportEmailLogRepository
	installRepo  *repository.SupportInboxInstallationRepository
	sessionRepo  *repository.SupportInboxSessionRepository
}

func setupEmailFallbackTestEnv(t *testing.T, settings model.SupportInboxSettings) *emailFallbackTestEnv {
	t.Helper()

	db := newTestDB(t)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})

	workspaceID := "11111111-1111-1111-1111-111111111111"
	ownerID := "22222222-2222-2222-2222-222222222222"
	seedWorkspace(t, db, workspaceID, "Acme", "acme", ownerID)

	messageRepo := repository.NewSupportMessageRepository(db)
	convRepo := repository.NewSupportConversationRepository(db)
	emailLogRepo := repository.NewSupportEmailLogRepository(db)
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
		installRepo,
		sessionRepo,
		workspaceRepo,
		"replies.helpin.ai",
		"https://app.helpin.ai",
		"pod-test",
	)

	return &emailFallbackTestEnv{
		redis:        redisClient,
		redisServer:  redisServer,
		service:      service,
		messageRepo:  messageRepo,
		convRepo:     convRepo,
		emailLogRepo: emailLogRepo,
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
	if captured.ReplyTo != "conv-"+conversationID+"@replies.helpin.ai" {
		t.Fatalf("unexpected reply-to: %q", captured.ReplyTo)
	}
	if !strings.Contains(captured.From, "Alex Agent - Acme Support <noreply@example.com>") {
		t.Fatalf("unexpected from: %q", captured.From)
	}
	if !strings.Contains(captured.HtmlBody, "#helpin-conv="+conversationID) {
		t.Fatalf("expected html body deep link, got %q", captured.HtmlBody)
	}
	if !strings.Contains(captured.TextBody, "annual billing") {
		t.Fatalf("expected text body to include batched content, got %q", captured.TextBody)
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

func TestEmailFallbackProcessInboundEmailCreatesMessageAndDedupes(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()

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
		Subject:           "Re: Inbound test",
		MessageID:         "pm-in-1",
		MailboxHash:       "conv-" + conversationID,
		StrippedTextReply: "Thanks, that helps.",
		HtmlBody:          "<p>Thanks, that helps.</p>",
	}

	if err := env.service.ProcessInboundEmail(ctx, payload); err != nil {
		t.Fatalf("process inbound email: %v", err)
	}
	if err := env.service.ProcessInboundEmail(ctx, payload); err != nil {
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
}
