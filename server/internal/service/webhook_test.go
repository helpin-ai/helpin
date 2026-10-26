package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type webhookReceiver struct {
	mu       sync.Mutex
	status   int
	requests []*http.Request
	bodies   [][]byte
}

func (r *webhookReceiver) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.requests = append(r.requests, req)
		r.bodies = append(r.bodies, body)
		status := r.status
		r.mu.Unlock()
		if status == 0 {
			status = http.StatusNoContent
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte("receiver says hi"))
	}
}

func (r *webhookReceiver) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

type webhookFixture struct {
	db          *gorm.DB
	service     *WebhookService
	receiver    *webhookReceiver
	server      *httptest.Server
	workspaceID string
	userID      string
	now         time.Time
}

func newWebhookFixture(t *testing.T) *webhookFixture {
	t.Helper()
	db := newTestDB(t)
	f := &webhookFixture{db: db, workspaceID: uuid.NewString(), userID: uuid.NewString(), receiver: &webhookReceiver{}}
	seedUser(t, db, f.userID, "admin@example.com", "Ada Admin", "hash")
	seedWorkspace(t, db, f.workspaceID, "Hooks", "hooks-"+f.workspaceID[:8], f.userID)
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil,
		repository.NewUserRepository(db), nil, nil, nil)
	inbox.workspaceRepo = repository.NewWorkspaceRepository(db)
	inbox.tagRepo = repository.NewSupportTagRepository(db)
	f.server = httptest.NewServer(f.receiver.handler())
	t.Cleanup(f.server.Close)
	f.service = NewWebhookService(repository.NewWebhookRepository(db), inbox, WebhookConfig{
		EncryptionKey: []byte(strings.Repeat("k", 32)), AllowPrivateNetworks: true, AppBaseURL: "https://app.example.com",
	})
	f.now = time.Now().UTC().Truncate(time.Second)
	f.service.now = func() time.Time { return f.now }
	return f
}

func (f *webhookFixture) endpoint(t *testing.T, req model.WebhookEndpointRequest) (*model.WebhookEndpoint, string) {
	t.Helper()
	if req.URL == "" {
		req.URL = f.server.URL + "/hook"
	}
	created, err := f.service.CreateEndpoint(context.Background(), f.workspaceID, f.userID, req)
	if err != nil {
		t.Fatal(err)
	}
	return created.Endpoint, created.Secret
}

func (f *webhookFixture) conversation(t *testing.T, mailboxID *string) string {
	t.Helper()
	conversation := &model.SupportConversation{ID: uuid.NewString(), WorkspaceID: f.workspaceID, DisplayID: 7, Subject: "Invoice",
		Status: model.SupportConversationStatusOpen, Priority: "high", Channel: "email", MailboxID: mailboxID,
		CustomerEmail: strRef("ada@customer.test"), CreatedAt: f.now, UpdatedAt: f.now}
	if err := f.db.Create(conversation).Error; err != nil {
		t.Fatal(err)
	}
	return conversation.ID
}

func (f *webhookFixture) deliveries(t *testing.T) []model.WebhookDelivery {
	t.Helper()
	var deliveries []model.WebhookDelivery
	if err := f.db.Order("created_at").Find(&deliveries).Error; err != nil {
		t.Fatal(err)
	}
	return deliveries
}

func (f *webhookFixture) process(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.service.ProcessEvents(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ProcessDeliveries(ctx, 100); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWebhookURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://hooks.example.com/helpin?token=abc": true,
		"http://hooks.example.com/helpin":           false,
		"https://localhost/hook":                    false,
		"https://127.0.0.1/hook":                    false,
		"https://10.1.2.3/hook":                     false,
		"https://169.254.169.254/latest":            false,
		"https://user:pass@hooks.example.com/":      false,
		"ftp://hooks.example.com/":                  false,
		"not a url":                                 false,
	} {
		if _, err := ValidateWebhookURL(raw, false); (err == nil) != ok {
			t.Errorf("ValidateWebhookURL(%q) = %v", raw, err)
		}
	}
	if _, err := ValidateWebhookURL("http://10.1.2.3/hook", true); err != nil {
		t.Errorf("private networks allowed: %v", err)
	}
}

func TestWebhookEndpointManagement(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()
	if _, err := f.service.CreateEndpoint(ctx, f.workspaceID, f.userID, model.WebhookEndpointRequest{URL: f.server.URL, Events: []string{"conversation.deleted"}}); !errors.Is(err, ErrWebhookInvalid) {
		t.Fatalf("unknown event accepted: %v", err)
	}
	if _, err := f.service.CreateEndpoint(ctx, f.workspaceID, f.userID, model.WebhookEndpointRequest{URL: f.server.URL, MailboxIDs: []string{uuid.NewString()}}); !errors.Is(err, ErrWebhookInvalid) {
		t.Fatalf("unknown inbox accepted: %v", err)
	}
	disabled := false
	endpoint, secret := f.endpoint(t, model.WebhookEndpointRequest{Description: "  CRM   sync ", Events: []string{"sla.breached", "sla.breached"}, Enabled: &disabled})
	if !strings.HasPrefix(secret, "whsec_") || endpoint.SecretHint != secret[len(secret)-4:] || endpoint.Description != "CRM sync" ||
		len(endpoint.Events) != 1 || endpoint.Enabled {
		t.Fatalf("created endpoint: %+v", endpoint)
	}
	var stored model.WebhookEndpoint
	f.db.Where("id = ?", endpoint.ID).Take(&stored)
	if stored.Enabled || strings.Contains(stored.EncryptedSecret, secret) {
		t.Fatalf("stored endpoint: enabled=%v secret stored in plain text=%v", stored.Enabled, strings.Contains(stored.EncryptedSecret, secret))
	}
	rotated, err := f.service.RotateSecret(ctx, f.workspaceID, endpoint.ID)
	if err != nil || rotated.Secret == secret {
		t.Fatalf("rotate: %v", err)
	}
	if got, err := f.service.secret(*mustWebhookEndpoint(t, f, endpoint.ID)); err != nil || got != rotated.Secret {
		t.Fatalf("rotated secret not stored: %v", err)
	}
	noKey := NewWebhookService(f.service.repo, f.service.inbox, WebhookConfig{AllowPrivateNetworks: true})
	if _, err := noKey.CreateEndpoint(ctx, f.workspaceID, f.userID, model.WebhookEndpointRequest{URL: f.server.URL}); !errors.Is(err, ErrWebhookSecretsUnavailable) {
		t.Fatalf("created without an encryption key: %v", err)
	}
	for i := 1; i < model.WebhookEndpointLimit; i++ {
		f.endpoint(t, model.WebhookEndpointRequest{})
	}
	if _, err := f.service.CreateEndpoint(ctx, f.workspaceID, f.userID, model.WebhookEndpointRequest{URL: f.server.URL}); !errors.Is(err, ErrWebhookInvalid) {
		t.Fatalf("endpoint limit not enforced: %v", err)
	}
	if err := f.service.DeleteEndpoint(ctx, f.workspaceID, endpoint.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.service.DeleteEndpoint(ctx, f.workspaceID, endpoint.ID); !errors.Is(err, repository.ErrWebhookEndpointNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func mustWebhookEndpoint(t *testing.T, f *webhookFixture, id string) *model.WebhookEndpoint {
	t.Helper()
	endpoint, err := f.service.repo.GetEndpoint(context.Background(), f.workspaceID, id)
	if err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func TestWebhookDeliverySignedPayload(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()
	endpoint, secret := f.endpoint(t, model.WebhookEndpointRequest{Events: []string{model.WebhookEventSLABreached}})
	conversationID := f.conversation(t, nil)
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventCSATSubmitted, conversationID, map[string]any{"csat": map[string]any{"rating": 5}})
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventSLABreached, conversationID, map[string]any{"sla": map[string]any{"metrics": []string{"first_response"}}})
	f.process(t)
	if f.receiver.count() != 1 {
		t.Fatalf("receiver got %d requests; only the subscribed event should be sent", f.receiver.count())
	}
	req, body := f.receiver.requests[0], f.receiver.bodies[0]
	signature := req.Header.Get("Helpin-Signature")
	parts := strings.Split(signature, ",")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "t=") || !strings.HasPrefix(parts[1], "v1=") {
		t.Fatalf("signature header: %q", signature)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.TrimPrefix(parts[0], "t=") + "." + string(body)))
	if strings.TrimPrefix(parts[1], "v1=") != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("signature does not verify with the endpoint secret")
	}
	if req.Header.Get("Helpin-Event-Type") != model.WebhookEventSLABreached || req.Header.Get("Helpin-Event-Id") == "" || req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("headers: %v", req.Header)
	}
	var envelope struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Conversation model.WebhookConversation `json:"conversation"`
			SLA          map[string]any            `json:"sla"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	c := envelope.Data.Conversation
	if envelope.ID != req.Header.Get("Helpin-Event-Id") || c.ID != conversationID || c.Number != 7 || c.Priority != "high" ||
		c.Customer.Email == nil || *c.Customer.Email != "ada@customer.test" || !strings.HasSuffix(c.URL, "/support/"+conversationID) || envelope.Data.SLA == nil {
		t.Fatalf("payload: %s", body)
	}
	deliveries := f.deliveries(t)
	if len(deliveries) != 1 || deliveries[0].Status != model.WebhookDeliverySucceeded || deliveries[0].Attempts != 1 || *deliveries[0].LastStatusCode != http.StatusNoContent {
		t.Fatalf("delivery: %+v", deliveries)
	}
	if got := mustWebhookEndpoint(t, f, endpoint.ID); got.LastSuccessAt == nil || got.FailureCount != 0 {
		t.Fatalf("endpoint health: %+v", got)
	}
}

func TestWebhookDeliveryRetriesAndDisables(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()
	f.receiver.status = http.StatusInternalServerError
	endpoint, _ := f.endpoint(t, model.WebhookEndpointRequest{})
	conversationID := f.conversation(t, nil)
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventCSATSubmitted, conversationID, map[string]any{})
	f.process(t)
	delivery := f.deliveries(t)[0]
	if delivery.Status != model.WebhookDeliveryPending || delivery.Attempts != 1 || !delivery.NextAttemptAt.Equal(f.now.Add(time.Minute)) ||
		!strings.Contains(delivery.LastError, "HTTP 500") {
		t.Fatalf("first failure: %+v", delivery)
	}
	// Not due yet: nothing is sent.
	f.process(t)
	if f.receiver.count() != 1 {
		t.Fatalf("retried before the backoff: %d", f.receiver.count())
	}
	// The last retry fails for good.
	f.db.Model(&model.WebhookDelivery{}).Where("id = ?", delivery.ID).Updates(map[string]any{"attempts": len(webhookRetryDelays), "next_attempt_at": f.now})
	f.process(t)
	if got := f.deliveries(t)[0]; got.Status != model.WebhookDeliveryFailed || got.Attempts != len(webhookRetryDelays)+1 {
		t.Fatalf("final failure: %+v", got)
	}
	// 410 Gone turns the endpoint off and ends its pending deliveries.
	f.receiver.status = http.StatusGone
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventCSATSubmitted, conversationID, map[string]any{})
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventCSATSubmitted, conversationID, map[string]any{})
	if _, err := f.service.ProcessEvents(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ProcessDeliveries(ctx, 1); err != nil {
		t.Fatal(err)
	}
	got := mustWebhookEndpoint(t, f, endpoint.ID)
	if got.Enabled || !strings.Contains(got.DisabledReason, "410") {
		t.Fatalf("endpoint after 410: %+v", got)
	}
	var pending int64
	f.db.Model(&model.WebhookDelivery{}).Where("status = ?", model.WebhookDeliveryPending).Count(&pending)
	if pending != 0 {
		t.Fatalf("%d deliveries still pending for a disabled endpoint", pending)
	}
	// A disabled endpoint receives nothing new.
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventCSATSubmitted, conversationID, map[string]any{})
	var events int64
	f.db.Model(&model.WebhookEvent{}).Count(&events)
	if events != 0 {
		t.Fatal("event recorded for a disabled endpoint")
	}
	if _, err := f.service.Redeliver(ctx, f.workspaceID, endpoint.ID, delivery.ID); !errors.Is(err, ErrWebhookInvalid) {
		t.Fatalf("redelivered to a disabled endpoint: %v", err)
	}
	// Turning it back on clears the failure history; redelivery keeps the event ID.
	enabled := true
	if _, err := f.service.UpdateEndpoint(ctx, f.workspaceID, endpoint.ID, model.WebhookEndpointRequest{URL: endpoint.URL, Enabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	if got := mustWebhookEndpoint(t, f, endpoint.ID); !got.Enabled || got.DisabledReason != "" || got.FailureCount != 0 {
		t.Fatalf("re-enabled endpoint: %+v", got)
	}
	again, err := f.service.Redeliver(ctx, f.workspaceID, endpoint.ID, delivery.ID)
	if err != nil || again.EventID != delivery.EventID || again.Status != model.WebhookDeliveryPending {
		t.Fatalf("redeliver: %+v %v", again, err)
	}
}

func TestWebhookAutoDisableAfterRepeatedFailures(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()
	f.receiver.status = http.StatusServiceUnavailable
	endpoint, _ := f.endpoint(t, model.WebhookEndpointRequest{})
	f.db.Model(&model.WebhookEndpoint{}).Where("id = ?", endpoint.ID).Update("failure_count", webhookDisableAfterFailures-1)
	if _, err := f.service.SendTest(ctx, f.workspaceID, endpoint.ID); err != nil {
		t.Fatal(err)
	}
	if got := mustWebhookEndpoint(t, f, endpoint.ID); got.Enabled || !strings.Contains(got.DisabledReason, "failed deliveries") {
		t.Fatalf("endpoint after repeated failures: %+v", got)
	}
}

func TestWebhookTestEventAndInboxFilter(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()
	mailbox := uuid.NewString()
	mustExec(t, f.db, `INSERT INTO support_mailboxes (id, workspace_id, name, handle, active, created_by_id) VALUES (?, ?, 'Billing', 'billing', 1, ?)`, mailbox, f.workspaceID, f.userID)
	endpoint, _ := f.endpoint(t, model.WebhookEndpointRequest{MailboxIDs: []string{mailbox}})
	delivery, err := f.service.SendTest(ctx, f.workspaceID, endpoint.ID)
	if err != nil || delivery.Status != model.WebhookDeliverySucceeded || delivery.EventType != model.WebhookEventPing {
		t.Fatalf("test event: %+v %v", delivery, err)
	}
	shared := f.conversation(t, nil)
	billing := f.conversation(t, &mailbox)
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventCSATSubmitted, shared, map[string]any{})
	f.service.Emit(ctx, f.workspaceID, model.WebhookEventCSATSubmitted, billing, map[string]any{})
	f.process(t)
	var sent []model.WebhookDelivery
	f.db.Where("event_type = ?", model.WebhookEventCSATSubmitted).Find(&sent)
	if len(sent) != 1 || !strings.Contains(string(sent[0].Payload), billing) {
		t.Fatalf("inbox filter: %d deliveries", len(sent))
	}
}

func TestWebhookDropsDeletedMessages(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()
	f.endpoint(t, model.WebhookEndpointRequest{})
	conversationID := f.conversation(t, nil)
	messages := repository.NewSupportMessageRepository(f.db)
	kept := &model.SupportMessage{WorkspaceID: f.workspaceID, ConversationID: conversationID, SenderType: "user", MessageType: "reply", Content: "Sent"}
	undone := &model.SupportMessage{WorkspaceID: f.workspaceID, ConversationID: conversationID, SenderType: "user", MessageType: "reply", Content: "Undone"}
	for _, message := range []*model.SupportMessage{kept, undone} {
		if err := messages.Create(ctx, message); err != nil {
			t.Fatal(err)
		}
		// SQLite has no triggers: record the events they would.
		if err := f.service.repo.InsertEvent(ctx, &model.WebhookEvent{ID: uuid.NewString(), WorkspaceID: f.workspaceID, EventType: model.WebhookEventMessageCreated,
			ConversationID: &conversationID, MessageID: &message.ID, Data: json.RawMessage(`{}`), AvailableAt: f.now, CreatedAt: f.now}); err != nil {
			t.Fatal(err)
		}
	}
	f.db.Delete(undone)
	f.process(t)
	if f.receiver.count() != 1 || !strings.Contains(string(f.receiver.bodies[0]), `"content":"Sent"`) {
		t.Fatalf("deliveries: %d", f.receiver.count())
	}
}
