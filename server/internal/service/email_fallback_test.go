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
	supportInboxService.SetWorkspaceRepo(workspaceRepo)
	service.SetSupportInboxService(supportInboxService)

	return &emailFallbackTestEnv{
		redis:        redisClient,
		redisServer:  redisServer,
		service:      service,
		routeRepo:    routeRepo,
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
	if !strings.Contains(messages[0].Metadata, `"link_previews"`) {
		t.Fatalf("expected link preview metadata, got %q", messages[0].Metadata)
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

	resp, total, err := env.convRepo.List(ctx, workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "")
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

	resp, total, err := env.convRepo.List(ctx, workspaceID, "", "", model.PMPagination{Page: 1, PerPage: 10}, "", model.RoleOwner, nil, "")
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total != 1 || len(resp) != 1 || resp[0].ID != conversationID {
		t.Fatalf("expected reply to stay on original conversation, got %#v total=%d", resp, total)
	}
}

func TestEmailFallbackRenderBodiesIncludesUnsubscribeLink(t *testing.T) {
	svc := &EmailFallbackService{}
	htmlBody, textBody := svc.renderBodies(
		[]model.SupportMessage{{Content: "Thanks for reaching out."}},
		"Alex Agent",
		"Acme Support",
		"https://example.com/#helpin-conv=conv-1",
		"unsubscribe-conv-1@replies.helpin.ai",
	)

	if !strings.Contains(htmlBody, "mailto:unsubscribe-conv-1@replies.helpin.ai") {
		t.Fatalf("expected html unsubscribe mailto link, got %q", htmlBody)
	}
	if strings.Contains(htmlBody, "max-width:600px") {
		t.Fatalf("expected plain html email body without template wrapper, got %q", htmlBody)
	}
	if !strings.Contains(textBody, "Unsubscribe: mailto:unsubscribe-conv-1@replies.helpin.ai") {
		t.Fatalf("expected text unsubscribe mailto link, got %q", textBody)
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
