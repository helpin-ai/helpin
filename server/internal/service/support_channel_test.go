package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/channels/telegram"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const testBotToken = "424242:AAE-test-token"

// fakeTelegram is a Bot API stand-in that records webhook registrations.
type fakeTelegram struct {
	mu        sync.Mutex
	botID     int64
	reject    bool // getMe answers 401
	failHook  bool // setWebhook answers 500
	webhooks  []map[string]any
	deletions int
}

func (f *fakeTelegram) serve(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if f.reject {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": f.botID, "is_bot": true, "first_name": "Acme Support", "username": "acme_support_bot"}})
		case strings.HasSuffix(r.URL.Path, "/setWebhook"):
			if f.failHook {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ok":false,"error_code":500,"description":"boom"}`))
				return
			}
			var params map[string]any
			_ = json.NewDecoder(r.Body).Decode(&params)
			f.webhooks = append(f.webhooks, params)
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		case strings.HasSuffix(r.URL.Path, "/deleteWebhook"):
			f.deletions++
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func (f *fakeTelegram) lastSecret() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.webhooks) == 0 {
		return ""
	}
	secret, _ := f.webhooks[len(f.webhooks)-1]["secret_token"].(string)
	return secret
}

type channelFixture struct {
	db        *gorm.DB
	svc       *SupportChannelService
	tg        *fakeTelegram
	workspace string
	other     string
	mailbox   string
}

func newChannelFixture(t *testing.T) *channelFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SupportChannelAccount{}, &model.SupportInboundJob{}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE UNIQUE INDEX idx_support_channel_accounts_live ON support_channel_accounts (provider, external_account_id) WHERE disabled_at IS NULL`,
		`CREATE TABLE support_mailboxes (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL)`,
		`CREATE TABLE support_channel_outbox (id TEXT PRIMARY KEY, workspace_id TEXT, channel_account_id TEXT, dispatch_status TEXT,
			last_error_code TEXT, claim_token TEXT, lease_expires_at DATETIME, updated_at DATETIME)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	f := &channelFixture{db: db, tg: &fakeTelegram{botID: 424242}, workspace: uuid.NewString(), other: uuid.NewString(), mailbox: uuid.NewString()}
	if err := db.Exec(`INSERT INTO support_mailboxes (id, workspace_id, name) VALUES (?, ?, 'Billing')`, f.mailbox, f.workspace).Error; err != nil {
		t.Fatal(err)
	}
	server := f.tg.serve(t)
	f.svc = NewSupportChannelService(repository.NewSupportChannelAccountRepository(db), repository.NewSupportInboundJobRepository(db),
		channels.NewRegistry(telegram.NewAdapter(server.URL)),
		SupportChannelConfig{EncryptionKey: []byte(strings.Repeat("k", 32)), PublicAPIBaseURL: "https://helpin.example/", RequireHTTPS: true})
	return f
}

func (f *channelFixture) connect(t *testing.T, workspace string) *model.SupportChannelAccount {
	t.Helper()
	account, err := f.svc.Connect(context.Background(), workspace, uuid.NewString(), model.ConnectSupportChannelRequest{Provider: "telegram", Token: testBotToken})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return account
}

func (f *channelFixture) countAccounts(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&model.SupportChannelAccount{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConnectStoresEncryptedTokenAndRegistersWebhook(t *testing.T) {
	f := newChannelFixture(t)
	account, err := f.svc.Connect(context.Background(), f.workspace, uuid.NewString(),
		model.ConnectSupportChannelRequest{Provider: "telegram", Token: " " + testBotToken + " ", MailboxID: &f.mailbox})
	if err != nil {
		t.Fatal(err)
	}
	if account.ExternalAccountID != "424242" || account.Handle != "acme_support_bot" || account.Status != model.SupportChannelAccountActive {
		t.Errorf("account = %+v", account)
	}
	if account.MailboxID == nil || *account.MailboxID != f.mailbox {
		t.Errorf("mailbox = %v", account.MailboxID)
	}
	var stored model.SupportChannelAccount
	if err := f.db.First(&stored, "id = ?", account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.TokenEncrypted, testBotToken) || stored.TokenEncrypted == "" {
		t.Fatal("token stored in clear")
	}
	if token, err := f.svc.decryptToken(&stored); err != nil || token != testBotToken {
		t.Fatalf("decrypt = %q %v", token, err)
	}
	hook := f.tg.webhooks[0]
	if hook["url"] != "https://helpin.example/api/webhooks/channels/telegram/"+account.ID {
		t.Errorf("webhook url = %v", hook["url"])
	}
	sum := sha256.Sum256([]byte(f.tg.lastSecret()))
	if stored.WebhookSecretHash != hex.EncodeToString(sum[:]) || len(f.tg.lastSecret()) < 40 {
		t.Error("stored hash does not match the registered secret")
	}
}

func TestConnectRefusalsStoreNothing(t *testing.T) {
	foreignMailbox := uuid.NewString()
	for _, tc := range []struct {
		name  string
		setup func(f *channelFixture)
		req   func(f *channelFixture) model.ConnectSupportChannelRequest
		want  error
	}{
		{"rejected token", func(f *channelFixture) { f.tg.reject = true }, nil, ErrChannelTokenRejected},
		{"webhook registration fails", func(f *channelFixture) { f.tg.failHook = true }, nil, ErrChannelProviderUnavailable},
		{"no encryption key", func(f *channelFixture) { f.svc.cfg.EncryptionKey = nil }, nil, ErrChannelEncryptionUnavailable},
		{"plain http public url", func(f *channelFixture) { f.svc.cfg.PublicAPIBaseURL = "http://helpin.example" }, nil, ErrChannelPublicURLUnavailable},
		{"no public url", func(f *channelFixture) { f.svc.cfg.PublicAPIBaseURL = "" }, nil, ErrChannelPublicURLUnavailable},
		{"unknown provider", nil, func(f *channelFixture) model.ConnectSupportChannelRequest {
			return model.ConnectSupportChannelRequest{Provider: "carrier-pigeon", Token: testBotToken}
		}, ErrChannelInvalid},
		{"blank token", nil, func(f *channelFixture) model.ConnectSupportChannelRequest {
			return model.ConnectSupportChannelRequest{Provider: "telegram", Token: "  "}
		}, ErrChannelInvalid},
		{"inbox from another workspace", nil, func(f *channelFixture) model.ConnectSupportChannelRequest {
			return model.ConnectSupportChannelRequest{Provider: "telegram", Token: testBotToken, MailboxID: &foreignMailbox}
		}, ErrChannelInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newChannelFixture(t)
			if tc.setup != nil {
				tc.setup(f)
			}
			req := model.ConnectSupportChannelRequest{Provider: "telegram", Token: testBotToken}
			if tc.req != nil {
				req = tc.req(f)
			}
			if _, err := f.svc.Connect(context.Background(), f.workspace, "", req); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if n := f.countAccounts(t); n != 0 {
				t.Fatalf("%d accounts stored", n)
			}
		})
	}
}

func TestConnectRefusesBotLiveElsewhereWithoutTouchingItsWebhook(t *testing.T) {
	f := newChannelFixture(t)
	f.connect(t, f.other)
	registrations := len(f.tg.webhooks)
	if _, err := f.svc.Connect(context.Background(), f.workspace, "", model.ConnectSupportChannelRequest{Provider: "telegram", Token: testBotToken}); !errors.Is(err, ErrChannelAlreadyConnected) {
		t.Fatalf("err = %v", err)
	}
	if len(f.tg.webhooks) != registrations {
		t.Fatal("setWebhook was called, which would steal the other workspace's webhook")
	}
}

func telegramUpdate(updateID int, chatType, text string) []byte {
	return []byte(`{"update_id":` + itoaTest(updateID) + `,"message":{"message_id":5,"from":{"id":9001,"is_bot":false,"first_name":"Ada"},"chat":{"id":9001,"type":"` + chatType + `"},"date":1763020800,"text":"` + text + `"}}`)
}

func itoaTest(n int) string { b, _ := json.Marshal(n); return string(b) }

func secretHeader(secret string) http.Header {
	h := http.Header{}
	h.Set(telegram.SecretHeader, secret)
	return h
}

func (f *channelFixture) jobs(t *testing.T) []model.SupportInboundJob {
	t.Helper()
	var jobs []model.SupportInboundJob
	if err := f.db.Order("created_at").Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	return jobs
}

func TestAcceptWebhookStoresEachUpdateOnce(t *testing.T) {
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	secret := f.tg.lastSecret()
	ctx := context.Background()
	for i := 0; i < 2; i++ { // the provider retried the same update
		if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, secretHeader(secret), telegramUpdate(77, "private", "hello")); err != nil {
			t.Fatal(err)
		}
	}
	jobs := f.jobs(t)
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d", len(jobs))
	}
	job := jobs[0]
	if job.Kind != SupportInboundKindChannelEvent || job.Status != "pending" || *job.WorkspaceID != f.workspace || *job.ChannelAccountID != account.ID {
		t.Errorf("job = %+v", job)
	}
	if *job.SubjectKey != model.SupportChannelSubjectKey(account.ID, "9001") || strings.Contains(*job.SubjectKey, "9001") {
		t.Errorf("subject key = %q", *job.SubjectKey)
	}
	var payload channelEventPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.AccountID != account.ID || payload.Event.Text != "hello" || payload.Event.ExternalMessageKey != "9001:5" {
		t.Errorf("payload = %+v", payload)
	}
	var stored model.SupportChannelAccount
	f.db.First(&stored, "id = ?", account.ID)
	if stored.LastEventAt == nil {
		t.Error("last event time not recorded")
	}
}

func TestAcceptWebhookRejectsAndDrops(t *testing.T) {
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	secret := f.tg.lastSecret()
	ctx := context.Background()

	if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, secretHeader("wrong"), telegramUpdate(1, "private", "x")); !errors.Is(err, ErrChannelWebhookUnauthorized) {
		t.Errorf("wrong secret: %v", err)
	}
	if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, http.Header{}, telegramUpdate(1, "private", "x")); !errors.Is(err, ErrChannelWebhookUnauthorized) {
		t.Errorf("missing secret: %v", err)
	}
	if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, secretHeader(secret), []byte("{not json")); !errors.Is(err, ErrChannelWebhookInvalid) {
		t.Errorf("malformed: %v", err)
	}
	// Acknowledged and dropped: an unknown account, a malformed id, an
	// unknown provider, and a group message.
	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		if err := f.svc.AcceptWebhook(ctx, "telegram", id, secretHeader(secret), telegramUpdate(2, "private", "x")); err != nil {
			t.Errorf("account %q: %v", id, err)
		}
	}
	if err := f.svc.AcceptWebhook(ctx, "whatsapp", account.ID, secretHeader(secret), telegramUpdate(3, "private", "x")); err != nil {
		t.Errorf("unknown provider: %v", err)
	}
	if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, secretHeader(secret), telegramUpdate(4, "group", "x")); err != nil {
		t.Errorf("group: %v", err)
	}
	if jobs := f.jobs(t); len(jobs) != 0 {
		t.Fatalf("jobs stored: %d", len(jobs))
	}
}

func TestReconnectRotatesSecretAndChecksTheBot(t *testing.T) {
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	oldSecret := f.tg.lastSecret()
	ctx := context.Background()
	if _, err := f.svc.Reconnect(ctx, f.workspace, account.ID, model.ReconnectSupportChannelRequest{}); err != nil {
		t.Fatal(err)
	}
	newSecret := f.tg.lastSecret()
	if newSecret == oldSecret {
		t.Fatal("secret not rotated")
	}
	if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, secretHeader(oldSecret), telegramUpdate(5, "private", "x")); !errors.Is(err, ErrChannelWebhookUnauthorized) {
		t.Errorf("old secret still accepted: %v", err)
	}
	if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, secretHeader(newSecret), telegramUpdate(5, "private", "x")); err != nil {
		t.Errorf("new secret rejected: %v", err)
	}

	f.tg.botID = 515151 // a token for a different bot
	if _, err := f.svc.Reconnect(ctx, f.workspace, account.ID, model.ReconnectSupportChannelRequest{Token: "515151:other"}); !errors.Is(err, ErrChannelInvalid) {
		t.Errorf("other bot's token: %v", err)
	}
	if _, err := f.svc.Reconnect(ctx, f.other, account.ID, model.ReconnectSupportChannelRequest{}); !errors.Is(err, repository.ErrSupportChannelAccountNotFound) {
		t.Errorf("another workspace reconnected it: %v", err)
	}
}

func TestDisconnectCancelsPendingRepliesAndAllowsReconnectElsewhere(t *testing.T) {
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	for id, status := range map[string]string{"r1": "pending", "r2": "dispatching", "r3": "accepted"} {
		if err := f.db.Exec(`INSERT INTO support_channel_outbox (id, workspace_id, channel_account_id, dispatch_status) VALUES (?, ?, ?, ?)`,
			id, f.workspace, account.ID, status).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if err := f.svc.Disconnect(ctx, f.workspace, account.ID); err != nil {
		t.Fatal(err)
	}
	if f.tg.deletions != 1 {
		t.Errorf("deleteWebhook calls = %d", f.tg.deletions)
	}
	var statuses []string
	f.db.Raw(`SELECT dispatch_status FROM support_channel_outbox ORDER BY id`).Scan(&statuses)
	if strings.Join(statuses, ",") != "cancelled,cancelled,accepted" {
		t.Errorf("outbox = %v", statuses)
	}
	if list, _ := f.svc.List(ctx, f.workspace); len(list) != 0 {
		t.Errorf("disconnected account still listed: %d", len(list))
	}
	if err := f.svc.AcceptWebhook(ctx, "telegram", account.ID, secretHeader(f.tg.lastSecret()), telegramUpdate(6, "private", "x")); err != nil {
		t.Errorf("webhook after disconnect: %v", err)
	}
	if jobs := f.jobs(t); len(jobs) != 0 {
		t.Error("events stored for a disconnected account")
	}
	if err := f.svc.Disconnect(ctx, f.workspace, account.ID); !errors.Is(err, repository.ErrSupportChannelAccountNotFound) {
		t.Errorf("second disconnect: %v", err)
	}
	f.connect(t, f.other) // the bot is free again
}

func TestUpdateChangesInboxAndAIReplies(t *testing.T) {
	f := newChannelFixture(t)
	account := f.connect(t, f.workspace)
	ctx := context.Background()
	disabled := true
	updated, err := f.svc.Update(ctx, f.workspace, account.ID, model.UpdateSupportChannelRequest{MailboxID: &f.mailbox, AIRepliesDisabled: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	if updated.MailboxID == nil || *updated.MailboxID != f.mailbox || !updated.AIRepliesDisabled {
		t.Fatalf("updated = %+v", updated)
	}
	empty := ""
	if updated, err = f.svc.Update(ctx, f.workspace, account.ID, model.UpdateSupportChannelRequest{MailboxID: &empty}); err != nil || updated.MailboxID != nil {
		t.Fatalf("clear mailbox = %+v %v", updated, err)
	}
	if _, err := f.svc.Update(ctx, f.other, account.ID, model.UpdateSupportChannelRequest{AIRepliesDisabled: &disabled}); !errors.Is(err, repository.ErrSupportChannelAccountNotFound) {
		t.Errorf("another workspace updated it: %v", err)
	}
}
