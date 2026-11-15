//go:build integration

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/channels/telegram"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var outboxTestKey = []byte(strings.Repeat("k", 32))

// scriptedTelegram answers sends from a queue of canned responses; an empty
// queue accepts.
type scriptedTelegram struct {
	mu        sync.Mutex
	responses []func(w http.ResponseWriter)
	sent      []map[string]any
}

func (f *scriptedTelegram) serve(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params map[string]any
		_ = json.NewDecoder(r.Body).Decode(&params)
		f.mu.Lock()
		var respond func(http.ResponseWriter)
		if len(f.responses) > 0 {
			respond, f.responses = f.responses[0], f.responses[1:]
		}
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if respond != nil {
			respond(w)
			return
		}
		f.mu.Lock()
		f.sent = append(f.sent, params)
		id := len(f.sent)
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":` + itoaTest(100+id) + `,"chat":{"id":7001}}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func (f *scriptedTelegram) queue(responses ...func(http.ResponseWriter)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses = append(f.responses, responses...)
}

func telegramError(status int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

type outboxFixture struct {
	db        *gorm.DB
	svc       *SupportChannelService
	inbox     *SupportInboxService
	tg        *scriptedTelegram
	workspace string
	userID    string
	account   *model.SupportChannelAccount
	conv      string
}

func newOutboxFixture(t *testing.T) *outboxFixture {
	t.Helper()
	db := openSupportOpsPostgres(t)
	f := &outboxFixture{db: db, tg: &scriptedTelegram{}}
	f.workspace, f.userID = seedSupportOpsWorkspace(t, db)
	f.account = &model.SupportChannelAccount{ID: uuid.NewString(), WorkspaceID: f.workspace, Provider: "telegram",
		ExternalAccountID: "bot-" + uuid.NewString()[:8], Handle: "acme_bot", WebhookSecretHash: "hash", Status: "active"}
	token, err := crypto.EncryptStringWithAAD("424242:token", outboxTestKey, channelTokenAAD(f.account.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.account.TokenEncrypted = token
	if err := db.Create(f.account).Error; err != nil {
		t.Fatal(err)
	}
	f.inbox = newChannelInbox(db)
	receipt, err := f.inbox.receiveChannelMessage(context.Background(), f.account, textEvent("7001", "7001:1", "Help please"), 20<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.conv = receipt.message.ConversationID
	server := f.tg.serve(t)
	f.svc = NewSupportChannelService(repository.NewSupportChannelAccountRepository(db), repository.NewSupportInboundJobRepository(db),
		channels.NewRegistry(telegram.NewAdapter(server.URL)), SupportChannelConfig{EncryptionKey: outboxTestKey}).
		SetInboxService(f.inbox).SetOutboxRepository(repository.NewSupportChannelOutboxRepository(db))
	f.inbox.SetChannelOutbox(repository.NewSupportChannelOutboxRepository(db))
	return f
}

func (f *outboxFixture) reply(t *testing.T, conversationID, senderType, content string, mutate ...func(*model.SupportMessage)) *model.SupportMessage {
	t.Helper()
	msg := &model.SupportMessage{WorkspaceID: f.workspace, ConversationID: conversationID, SenderType: senderType, MessageType: "reply", Content: content}
	if senderType == "user" {
		msg.SenderUserID = &f.userID
	}
	for _, m := range mutate {
		m(msg)
	}
	if err := f.inbox.messageRepo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

func (f *outboxFixture) row(t *testing.T, messageID string) *model.SupportChannelOutbox {
	t.Helper()
	var row model.SupportChannelOutbox
	if err := f.db.Where("message_id = ?", messageID).Limit(1).Find(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ID == "" {
		return nil
	}
	return &row
}

// due makes every queued row due now, skipping backoff and undo windows.
func (f *outboxFixture) due(t *testing.T) {
	t.Helper()
	mustExec(t, f.db, `UPDATE support_channel_outbox SET next_attempt_at = now() - interval '1 second' WHERE workspace_id = ? AND dispatch_status = 'pending'`, f.workspace)
}

func TestChannelOutboxTriggerQueuesOnlyCustomerVisibleReplies(t *testing.T) {
	f := newOutboxFixture(t)
	queued := map[string]bool{
		f.reply(t, f.conv, "user", "Teammate reply").ID:   true,
		f.reply(t, f.conv, "ai", "AI reply").ID:           true,
		f.reply(t, f.conv, "agent", "Agent reply").ID:     true,
		f.reply(t, f.conv, "user", "Internal note", func(m *model.SupportMessage) { m.IsInternal, m.MessageType = true, "note" }).ID: false,
		f.reply(t, f.conv, "customer", "Customer").ID: false,
		f.reply(t, f.conv, "user", "Email only", func(m *model.SupportMessage) { m.Metadata = `{"delivery_mode":"email_only"}` }).ID: false,
		f.reply(t, f.conv, "user", "Imported", func(m *model.SupportMessage) { m.Metadata = `{"imported_from":"intercom"}` }).ID: false,
	}
	for id, want := range queued {
		row := f.row(t, id)
		if (row != nil) != want {
			t.Errorf("message %s queued=%v want %v", id, row != nil, want)
		}
		if row != nil && (row.DispatchStatus != "pending" || row.ChannelAccountID != f.account.ID) {
			t.Errorf("row = %+v", row)
		}
	}
	// A widget conversation has no channel identity: nothing is queued.
	widget := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: f.workspace, DisplayID: 900, Subject: "Widget", Status: "open", Priority: "medium", Channel: "widget"}
	if err := f.db.Create(widget).Error; err != nil {
		t.Fatal(err)
	}
	if row := f.row(t, f.reply(t, widget.ID, "user", "Chat reply").ID); row != nil {
		t.Errorf("widget reply queued: %+v", row)
	}
}

func TestChannelOutboxUndoAndDisconnect(t *testing.T) {
	f := newOutboxFixture(t)
	until := time.Now().UTC().Add(10 * time.Second)
	undone := f.reply(t, f.conv, "user", "Oops", func(m *model.SupportMessage) { m.CancellableUntil = &until })
	row := f.row(t, undone.ID)
	if row == nil || row.NextAttemptAt.Before(until.Add(-time.Second)) {
		t.Fatalf("undo-window row = %+v", row)
	}
	mustExec(t, f.db, `UPDATE support_messages SET deleted_at = now() WHERE id = ?`, undone.ID)
	if row := f.row(t, undone.ID); row.DispatchStatus != "cancelled" || derefString(row.LastErrorCode) != "deleted" {
		t.Fatalf("undone row = %+v", row)
	}

	mustExec(t, f.db, `UPDATE support_channel_accounts SET disabled_at = now(), status = 'disabled' WHERE id = ?`, f.account.ID)
	if row := f.row(t, f.reply(t, f.conv, "user", "After disconnect").ID); row.DispatchStatus != "cancelled" || derefString(row.LastErrorCode) != "disconnected" {
		t.Fatalf("disconnected row = %+v", row)
	}
}

func TestChannelOutboxDeliversAndRecordsTheProviderKey(t *testing.T) {
	f := newOutboxFixture(t)
	msg := f.reply(t, f.conv, "user", "Your **refund** is on its way")
	f.due(t)
	if n, err := f.svc.ProcessOutbox(context.Background()); err != nil || n != 1 {
		t.Fatalf("processed %d, %v", n, err)
	}
	row := f.row(t, msg.ID)
	if row.DispatchStatus != "accepted" || row.ReceiptStatus != "sent" || derefString(row.ExternalMessageKey) != "7001:101" {
		t.Fatalf("row = %+v", row)
	}
	if len(f.tg.sent) != 1 || f.tg.sent[0]["text"] != "Your <b>refund</b> is on its way" || f.tg.sent[0]["parse_mode"] != "HTML" || f.tg.sent[0]["chat_id"] != "7001" {
		t.Fatalf("sent = %+v", f.tg.sent)
	}
	var stored model.SupportMessage
	f.db.First(&stored, "id = ?", msg.ID)
	if derefString(stored.ExternalMessageKey) != "7001:101" {
		t.Errorf("message key = %v", stored.ExternalMessageKey)
	}
	got, err := f.inbox.ListConversationMessages(context.Background(), f.workspace, f.conv, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got {
		if m.ID == msg.ID && (m.ChannelDelivery == nil || m.ChannelDelivery.Status != "sent") {
			t.Errorf("hydrated delivery = %+v", m.ChannelDelivery)
		}
	}
	// The inbox loads messages page by page.
	page, err := f.inbox.ListConversationMessagePage(context.Background(), f.workspace, f.conv, true, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range page.Data {
		if m.ID == msg.ID {
			found = m.ChannelDelivery != nil && m.ChannelDelivery.Status == "sent"
		}
	}
	if !found {
		t.Error("the message page has no delivery state")
	}
}

func TestChannelOutboxOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response func(http.ResponseWriter)
		dispatch string
		code     string
		retryable bool
	}{
		{"rate limited retries later", telegramError(429, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":30}}`), "pending", "rate_limited", false},
		{"blocked is final", telegramError(403, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`), "rejected", channels.CodeBlocked, true},
		{"server error is unconfirmed", telegramError(500, `{"ok":false,"error_code":500,"description":"Internal Server Error"}`), "uncertain", "provider_error_500", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOutboxFixture(t)
			msg := f.reply(t, f.conv, "user", "Hello")
			f.tg.queue(tc.response)
			f.due(t)
			if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
				t.Fatal(err)
			}
			row := f.row(t, msg.ID)
			if row.DispatchStatus != tc.dispatch || derefString(row.LastErrorCode) != tc.code {
				t.Fatalf("row = %s %v", row.DispatchStatus, derefString(row.LastErrorCode))
			}
			if tc.dispatch == "pending" && row.NextAttemptAt.Before(time.Now().Add(25*time.Second)) {
				t.Errorf("retry_after not honoured: next attempt %v", row.NextAttemptAt)
			}
			if view := channelDeliveryView(*row); view.Retryable != tc.retryable {
				t.Errorf("retryable = %v", view.Retryable)
			}
			if len(f.tg.sent) != 0 {
				t.Errorf("unconfirmed or failed reply was sent: %v", f.tg.sent)
			}
			if tc.retryable {
				if err := f.inbox.RetryChannelDelivery(context.Background(), f.workspace, msg.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
					t.Fatal(err)
				}
				if row := f.row(t, msg.ID); row.DispatchStatus != "accepted" {
					t.Errorf("after retry = %s", row.DispatchStatus)
				}
			}
		})
	}
}

func TestChannelOutboxFormattingFallbackAndSplitting(t *testing.T) {
	f := newOutboxFixture(t)
	f.tg.queue(telegramError(400, `{"ok":false,"error_code":400,"description":"Bad Request: can't parse entities"}`))
	long := strings.Repeat("A paragraph of text that is long enough. ", 60) + "\n\n" + strings.Repeat("Second paragraph text. ", 120)
	msg := f.reply(t, f.conv, "user", "**Hi**\n\n"+long)
	f.due(t)
	if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	row := f.row(t, msg.ID)
	if row.DispatchStatus != "accepted" || row.PartsSent < 2 || len(f.tg.sent) != row.PartsSent {
		t.Fatalf("row = %s parts=%d sent=%d", row.DispatchStatus, row.PartsSent, len(f.tg.sent))
	}
	if f.tg.sent[0]["parse_mode"] != nil || !strings.HasPrefix(f.tg.sent[0]["text"].(string), "Hi") {
		t.Errorf("first part was not resent as plain text: %v", f.tg.sent[0])
	}
	for _, sent := range f.tg.sent {
		if len([]rune(sent["text"].(string))) > 4096 {
			t.Errorf("part over the limit: %d", len([]rune(sent["text"].(string))))
		}
	}
}

func TestChannelOutboxKeepsOrderPerCustomer(t *testing.T) {
	f := newOutboxFixture(t)
	first := f.reply(t, f.conv, "user", "First")
	second := f.reply(t, f.conv, "user", "Second")
	f.tg.queue(telegramError(429, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`))
	f.due(t)
	if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := f.row(t, second.ID); row.DispatchStatus != "pending" || row.Attempts != 0 {
		t.Fatalf("second reply went out before the first: %+v", row)
	}
	f.due(t)
	if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.due(t)
	if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.tg.sent) != 2 || f.tg.sent[0]["text"] != "First" || f.tg.sent[1]["text"] != "Second" {
		t.Fatalf("order = %v", f.tg.sent)
	}
	_ = first
}

func TestChannelOutboxRechecksBeforeSending(t *testing.T) {
	f := newOutboxFixture(t)
	msg := f.reply(t, f.conv, "user", "Queued")
	mustExec(t, f.db, `UPDATE support_channel_accounts SET disabled_at = now(), status = 'disabled' WHERE id = ?`, f.account.ID)
	f.due(t)
	if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
		t.Fatal(err)
	}
	if row := f.row(t, msg.ID); row.DispatchStatus != "cancelled" || derefString(row.LastErrorCode) != "disconnected" || len(f.tg.sent) != 0 {
		t.Fatalf("row = %+v sent=%d", row, len(f.tg.sent))
	}
}

func TestChannelOutboxRevokedTokenNeedsReconnect(t *testing.T) {
	f := newOutboxFixture(t)
	unauthorized := telegramError(401, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
	for i := 0; i < 2; i++ {
		f.tg.queue(unauthorized)
		f.reply(t, f.conv, "user", "Reply "+itoaTest(i))
		f.due(t)
		if _, err := f.svc.ProcessOutbox(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	f.db.Raw(`SELECT status FROM support_channel_accounts WHERE id = ?`, f.account.ID).Scan(&status)
	if status != model.SupportChannelAccountReauthRequired {
		t.Fatalf("account status = %s", status)
	}
}

func TestChannelOutboxRepliesFromAMergedConversationReachTelegram(t *testing.T) {
	f := newOutboxFixture(t)
	root := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: f.workspace, DisplayID: 901, Subject: "Root", Status: "open", Priority: "medium", Channel: "email"}
	if err := f.db.Create(root).Error; err != nil {
		t.Fatal(err)
	}
	// A Telegram message routed into the root keeps its origin conversation.
	mustExec(t, f.db, `INSERT INTO support_messages (id, workspace_id, conversation_id, origin_conversation_id, sender_type, message_type, content, channel_account_id, external_message_key)
		VALUES (?, ?, ?, ?, 'customer', 'reply', 'Routed', ?, '7001:99')`, uuid.NewString(), f.workspace, root.ID, f.conv, f.account.ID)
	row := f.row(t, f.reply(t, root.ID, "user", "Reply in the merged thread").ID)
	if row == nil || row.ChannelAccountID != f.account.ID || row.DispatchStatus != "pending" {
		t.Fatalf("merged reply row = %+v", row)
	}
}
