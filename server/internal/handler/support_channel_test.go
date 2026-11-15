package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/channels/telegram"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func newChannelWebhookRouter(t *testing.T) (http.Handler, *gorm.DB, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SupportChannelAccount{}, &model.SupportInboundJob{}); err != nil {
		t.Fatal(err)
	}
	accountID := uuid.NewString()
	sum := sha256.Sum256([]byte("the-secret"))
	if err := db.Create(&model.SupportChannelAccount{ID: accountID, WorkspaceID: uuid.NewString(), Provider: "telegram",
		ExternalAccountID: "1", TokenEncrypted: "enc", WebhookSecretHash: hex.EncodeToString(sum[:]), Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.NewSupportChannelService(repository.NewSupportChannelAccountRepository(db), repository.NewSupportInboundJobRepository(db),
		channels.NewRegistry(telegram.NewAdapter("http://127.0.0.1:1")), service.SupportChannelConfig{})
	r := chi.NewRouter()
	r.Post("/api/webhooks/channels/{provider}/{accountId}", NewSupportChannelHandler(svc).Webhook)
	return r, db, accountID
}

func postChannelWebhook(t *testing.T, router http.Handler, path, secret string, body []byte) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)).WithContext(context.Background())
	if secret != "" {
		req.Header.Set(telegram.SecretHeader, secret)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

func TestChannelWebhookStatusCodes(t *testing.T) {
	router, db, accountID := newChannelWebhookRouter(t)
	path := "/api/webhooks/channels/telegram/" + accountID
	update := []byte(`{"update_id":1,"message":{"message_id":2,"from":{"id":3,"is_bot":false,"first_name":"A"},"chat":{"id":3,"type":"private"},"date":1,"text":"hi"}}`)

	for _, tc := range []struct {
		name, path, secret string
		body               []byte
		want               int
	}{
		{"stored", path, "the-secret", update, http.StatusOK},
		{"wrong secret", path, "nope", update, http.StatusUnauthorized},
		{"no secret", path, "", update, http.StatusUnauthorized},
		{"malformed", path, "the-secret", []byte("{"), http.StatusBadRequest},
		{"too large", path, "the-secret", bytes.Repeat([]byte("x"), _maxChannelWebhookBody+1), http.StatusRequestEntityTooLarge},
		{"unknown account acknowledged", "/api/webhooks/channels/telegram/" + uuid.NewString(), "x", update, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := postChannelWebhook(t, router, tc.path, tc.secret, tc.body); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
	var jobs int64
	db.Model(&model.SupportInboundJob{}).Count(&jobs)
	if jobs != 1 {
		t.Errorf("jobs = %d", jobs)
	}

	// A storage failure answers 503 so Telegram retries the update.
	if err := db.Exec("DROP TABLE support_inbound_jobs").Error; err != nil {
		t.Fatal(err)
	}
	if got := postChannelWebhook(t, router, path, "the-secret", bytes.Replace(update, []byte(`"update_id":1`), []byte(`"update_id":9`), 1)); got != http.StatusServiceUnavailable {
		t.Errorf("storage failure status = %d", got)
	}
}

func TestChannelErrorMessagesAreActionable(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		text   string
	}{
		{service.ErrChannelTokenRejected, http.StatusBadRequest, "@BotFather"},
		{service.ErrChannelAlreadyConnected, http.StatusConflict, "Disconnect it there"},
		{service.ErrChannelEncryptionUnavailable, http.StatusServiceUnavailable, "CHANNEL_ENCRYPTION_KEY"},
		{service.ErrChannelPublicURLUnavailable, http.StatusServiceUnavailable, "PUBLIC_API_BASE_URL"},
		{repository.ErrSupportChannelAccountNotFound, http.StatusNotFound, "not found"},
	} {
		rec := httptest.NewRecorder()
		writeChannelError(rec, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.text) {
			t.Errorf("%v: %d %s", tc.err, rec.Code, rec.Body.String())
		}
	}
}
