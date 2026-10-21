package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// An ineligible anonymous submitter never gets portal access, so an agent's
// reply must reach them through the support email path.
func TestCustomerPortalAgentReplyReachesIneligibleSubmitterByEmail(t *testing.T) {
	ctx := context.Background()
	settings := model.DefaultSupportInboxSettings()
	settings.EmailFallbackEnabled = true
	settings.EmailFallbackDelaySecs = 1
	settings.PortalEnabled = true
	settings.PortalIntakeEnabled = true
	settings.PortalAnonymousIntakeEnabled = true
	env := setupEmailFallbackTestEnv(t, settings)
	db := env.convRepo.DB()
	createPortalTables(t, db)

	inbox := NewSupportInboxService(env.convRepo, nil, env.messageRepo, nil, nil, env.installRepo, env.sessionRepo, nil, nil, nil, nil, nil, nil, nil, nil)
	inbox.SetEmailFallbackService(env.service)
	sender := &recordingPortalSender{}
	portal := NewCustomerPortalService(repository.NewCustomerPortalRepository(db), inbox, sender, "https://app.helpin.ai")
	ws, err := portal.ResolveWorkspace(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	token, err := portal.StartAnonymousIntake(ctx, ws, "submitter@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := portal.CreateAnonymousRequest(ctx, ws, token, "Broken export", "The export fails", nil); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 || strings.Contains(sender.sent[0].text, "token=") {
		t.Fatalf("ineligible submitter should get a receipt without a link: %+v", sender.sent)
	}
	var conversation model.SupportConversation
	if err := db.Where("workspace_id = ? AND channel = 'portal'", ws.ID).First(&conversation).Error; err != nil {
		t.Fatal(err)
	}

	ownerID := "22222222-2222-2222-2222-222222222222"
	reply, err := inbox.CreateConversationMessage(ctx, ws.ID, conversation.ID, model.CreateMessageRequest{Content: "We fixed the export.", MessageType: "reply"}, "user", &ownerID, nil, strPtr("Alex Agent"))
	if err != nil {
		t.Fatal(err)
	}
	// The agent reply enqueues email asynchronously.
	var queued []string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		queued, err = env.redis.LRange(ctx, env.service.msgListKey(conversation.ID), 0, -1).Result()
		if err == nil && len(queued) > 0 {
			break
		}
	}
	if len(queued) != 1 || queued[0] != reply.ID {
		t.Fatalf("agent reply not queued for email: %v %v", queued, err)
	}

	// Replies stay cancellable briefly; the outbox poller sends after that window.
	sendAt := time.Now().Add(2 * time.Minute)
	env.service.now = func() time.Time { return sendAt }
	var captured capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &captured); err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK","MessageID":"pm-portal-1","To":"submitter@example.com"}`))}, nil
	})})
	if err := env.service.fireEmail(ctx, conversation.ID, queued); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(captured.To, "submitter@example.com") || !strings.Contains(captured.TextBody, "We fixed the export.") {
		t.Fatalf("reply not emailed to submitter: to=%q body=%q", captured.To, captured.TextBody)
	}
	if strings.Contains(captured.TextBody, "support portal") {
		t.Fatalf("ineligible submitter was sent a portal link: %q", captured.TextBody)
	}
}
