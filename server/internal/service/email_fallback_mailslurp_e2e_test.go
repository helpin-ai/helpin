//go:build email_e2e
// +build email_e2e

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const mailslurpBaseURL = "https://api.mailslurp.com"

type mailslurpInbox struct {
	ID           string `json:"id"`
	EmailAddress string `json:"emailAddress"`
}

type mailslurpEmail struct {
	ID      string            `json:"id"`
	Subject string            `json:"subject"`
	Body    string            `json:"body"`
	From    string            `json:"from"`
	To      []string          `json:"to"`
	Headers map[string]string `json:"headers"`
}

type mailslurpClient struct {
	apiKey     string
	httpClient *http.Client
}

func TestMailSlurpScenarioSupportOfflineFallbackDelivery(t *testing.T) {
	ctx := context.Background()
	mailslurpAPIKey := firstNonEmptyEnv("MAILSLURP_API_KEY")
	postmarkToken := firstNonEmptyEnv("POSTMARK_REPLY_SERVER_TOKEN", "POSTMARK_SERVER_TOKEN")
	postmarkFromEmail := firstNonEmptyEnv("POSTMARK_REPLY_FROM_EMAIL", "POSTMARK_FROM_EMAIL")
	replyDomain := firstNonEmptyEnv("SUPPORT_REPLY_DOMAIN", "SUPPORT_EMAIL_REPLY_DOMAIN")
	if replyDomain == "" {
		replyDomain = "replies.helpin.email"
	}
	if mailslurpAPIKey == "" {
		t.Skip("MAILSLURP_API_KEY is required for MailSlurp email E2E")
	}
	if postmarkToken == "" || postmarkFromEmail == "" {
		t.Skip("Postmark token and from email are required for MailSlurp email E2E")
	}

	ms := mailslurpClient{
		apiKey: mailslurpAPIKey,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
	inbox, err := ms.createInbox(ctx)
	skipOnProviderError(t, "create MailSlurp inbox", err)
	if inbox.ID == "" || inbox.EmailAddress == "" {
		t.Fatalf("MailSlurp returned incomplete inbox: id=%q email=%q", inbox.ID, inbox.EmailAddress)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = ms.deleteInbox(cleanupCtx, inbox.ID)
	})

	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 30
	settings.EmailFallbackMaxDeliveryAgeSecs = 600
	settings.EmailFallbackFromName = "Helpin E2E Support"
	env := setupEmailFallbackTestEnv(t, settings)
	defer env.redisServer.Close()
	env.service.emailClient = email.NewClient(postmarkToken, postmarkFromEmail)
	env.service.replyDomain = replyDomain
	env.service.supportInboxService = nil

	fixedNow := time.Now().UTC().Truncate(time.Second)
	env.service.now = func() time.Time { return fixedNow }

	workspaceID := "11111111-1111-1111-1111-111111111111"
	conversationID := "66666666-6666-6666-6666-666666660001"
	anonymousID := "mailslurp-offline-visitor"
	customerName := "MailSlurp Visitor"
	customerEmail := inbox.EmailAddress
	uniqueNeedle := "MailSlurp offline fallback " + conversationID

	conv := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "MailSlurp offline fallback test",
		Status:        "open",
		AnonymousID:   &anonymousID,
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
	}
	if err := env.convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	pageURL := "https://example.com/support"
	if err := env.sessionRepo.Create(ctx, &model.SupportWidgetSession{
		WorkspaceID:    workspaceID,
		ConversationID: &conversationID,
		SessionToken:   "mailslurp-session-token",
		AnonymousID:    anonymousID,
		IsAnonymous:    false,
		CustomerName:   &customerName,
		CustomerEmail:  &customerEmail,
		LastPageURL:    &pageURL,
		ExpiresAt:      fixedNow.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("create widget session: %v", err)
	}

	msg := &model.SupportMessage{
		ID:                "77777777-7777-7777-7777-777777770001",
		WorkspaceID:       workspaceID,
		ConversationID:    conversationID,
		SenderType:        "user",
		SenderDisplayName: strPtr("E2E Agent"),
		Content:           "This visitor is offline. " + uniqueNeedle,
		MessageType:       "reply",
		CreatedAt:         fixedNow,
	}
	if err := env.messageRepo.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := env.service.OnAgentReply(ctx, workspaceID, msg, conv); err != nil {
		t.Fatalf("enqueue fallback email: %v", err)
	}
	env.service.now = func() time.Time { return fixedNow.Add(31 * time.Second) }
	if err := env.service.claimAndFire(ctx, conversationID); err != nil {
		t.Fatalf("fire fallback email: %v", err)
	}

	received, err := ms.waitForLatestEmail(ctx, inbox.ID, 90*time.Second)
	skipOnProviderError(t, "wait for MailSlurp delivery", err)
	if !strings.Contains(received.Subject, "MailSlurp offline fallback test") {
		t.Fatalf("received subject %q does not include conversation context", received.Subject)
	}
	if !strings.Contains(received.Body, uniqueNeedle) {
		t.Fatalf("received email body does not include agent reply; email_id=%s", received.ID)
	}
	if got, want := strings.TrimSpace(received.Headers["Reply-To"]), "conv-"+conversationID+"@"+replyDomain; !strings.Contains(got, want) {
		t.Fatalf("received Reply-To %q, want %q; email_id=%s", got, want, received.ID)
	}
	if strings.Contains(received.Body, "postmark-token") || strings.Contains(received.Body, "MAILSLURP_API_KEY") {
		t.Fatalf("received email body appears to leak internal token text; email_id=%s", received.ID)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one outbound email log, got %d", len(logs))
	}
	logRow := logs[0]
	if logRow.ToEmail != inbox.EmailAddress {
		t.Fatalf("email log to_email = %q, want %q", logRow.ToEmail, inbox.EmailAddress)
	}
	if logRow.Status != "sent" {
		t.Fatalf("email log status = %q, want sent", logRow.Status)
	}
	if logRow.PostmarkMessageID == nil || strings.TrimSpace(*logRow.PostmarkMessageID) == "" {
		t.Fatal("expected email log to include postmark_message_id")
	}
	if len(logRow.MessageIDs) != 1 || logRow.MessageIDs[0] != msg.ID {
		t.Fatalf("email log message ids = %#v, want [%s]", logRow.MessageIDs, msg.ID)
	}
}

func (c mailslurpClient) createInbox(ctx context.Context) (mailslurpInbox, error) {
	endpoint, err := url.Parse(mailslurpBaseURL + "/inboxes")
	if err != nil {
		return mailslurpInbox{}, err
	}
	query := endpoint.Query()
	query.Set("useShortAddress", "true")
	query.Set("expiresIn", fmt.Sprintf("%d", int64(time.Hour/time.Millisecond)))
	endpoint.RawQuery = query.Encode()

	var inbox mailslurpInbox
	if err := c.doJSON(ctx, http.MethodPost, endpoint.String(), nil, &inbox); err != nil {
		return mailslurpInbox{}, err
	}
	return inbox, nil
}

func (c mailslurpClient) waitForLatestEmail(ctx context.Context, inboxID string, timeout time.Duration) (mailslurpEmail, error) {
	endpoint, err := url.Parse(mailslurpBaseURL + "/waitForLatestEmail")
	if err != nil {
		return mailslurpEmail{}, err
	}
	query := endpoint.Query()
	query.Set("inboxId", inboxID)
	query.Set("timeout", fmt.Sprintf("%d", int64(timeout/time.Millisecond)))
	query.Set("unreadOnly", "true")
	endpoint.RawQuery = query.Encode()

	var email mailslurpEmail
	if err := c.doJSON(ctx, http.MethodGet, endpoint.String(), nil, &email); err != nil {
		return mailslurpEmail{}, err
	}
	return email, nil
}

func (c mailslurpClient) deleteInbox(ctx context.Context, inboxID string) error {
	return c.doJSON(ctx, http.MethodDelete, mailslurpBaseURL+"/inboxes/"+url.PathEscape(inboxID), nil, nil)
}

func (c mailslurpClient) doJSON(ctx context.Context, method, endpoint string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return providerHTTPError{Provider: "MailSlurp", StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode MailSlurp response: %w", err)
	}
	return nil
}

type providerHTTPError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e providerHTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s returned HTTP %d", e.Provider, e.StatusCode)
	}
	return fmt.Sprintf("%s returned HTTP %d: %s", e.Provider, e.StatusCode, e.Body)
}

func skipOnProviderError(t *testing.T, step string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var httpErr providerHTTPError
	if errors.As(err, &httpErr) {
		if httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= 500 {
			t.Skipf("%s skipped because provider is unavailable: %v", step, err)
		}
	}
	if strings.Contains(err.Error(), "Client.Timeout") ||
		strings.Contains(err.Error(), "context deadline exceeded") ||
		strings.Contains(err.Error(), "connection reset by peer") ||
		strings.Contains(err.Error(), "no such host") {
		t.Skipf("%s skipped because provider/network is unavailable: %v", step, err)
	}
	t.Fatalf("%s failed: %v", step, err)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
