package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	sharedTestWorkspaceID = "11111111-1111-1111-1111-111111111111"
	otherTestWorkspaceID  = "99999999-9999-9999-9999-999999999999"
	// sharedTestSender is the shared fallback sender of the email test env.
	sharedTestSender   = "noreply@example.com"
	sharedTestCustomer = "taylor@customer.test"
)

var sharedTestAuthenticated = model.PostmarkHeader{Name: "X-Spam-Tests", Value: "DKIM_SIGNED,DKIM_VALID,DKIM_VALID_AU,SPF_PASS"}

// seedSharedSenderThread creates a conversation and the outbound email Helpin
// sent in it from the shared sender, returning that email's Message-ID.
func seedSharedSenderThread(t *testing.T, env *emailFallbackTestEnv, workspaceID, conversationID, subject string) string {
	t.Helper()
	ctx := context.Background()
	customer := sharedTestCustomer
	if err := env.convRepo.Create(ctx, &model.SupportConversation{
		ID: conversationID, WorkspaceID: workspaceID, Subject: subject, Status: "open", CustomerEmail: &customer,
	}); err != nil {
		t.Fatal(err)
	}
	logID := uuid.NewString()
	messageID := "<helpin-" + logID + "@replies.helpin.ai>"
	if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
		ID: logID, WorkspaceID: workspaceID, ConversationID: conversationID, Direction: "outbound",
		FromEmail: "noreply+conv-" + conversationID + "@example.com", ToEmail: customer,
		Subject: "Re: " + subject + " (#12)", RFCMessageID: messageID, Status: "sent",
	}); err != nil {
		t.Fatal(err)
	}
	return messageID
}

func seedOtherWorkspace(t *testing.T, env *emailFallbackTestEnv) {
	t.Helper()
	seedWorkspace(t, env.convRepo.DB(), otherTestWorkspaceID, "Globex", "globex", "22222222-2222-2222-2222-222222222222")
}

func sharedSenderPayload(id, subject, body string, headers ...model.PostmarkHeader) model.PostmarkInboundPayload {
	return model.PostmarkInboundPayload{
		MessageID:         "pm-" + id,
		From:              sharedTestCustomer,
		FromFull:          model.PostmarkAddress{Email: sharedTestCustomer, Name: "Taylor"},
		To:                sharedTestSender,
		ToFull:            []model.PostmarkAddress{{Email: sharedTestSender}},
		OriginalRecipient: sharedTestSender,
		Subject:           subject,
		TextBody:          body,
		StrippedTextReply: body,
		Headers:           headers,
	}
}

func customerMessages(t *testing.T, env *emailFallbackTestEnv, workspaceID, conversationID string) []model.SupportMessage {
	t.Helper()
	messages, err := env.messageRepo.ListByConversation(context.Background(), workspaceID, conversationID, false)
	if err != nil {
		t.Fatal(err)
	}
	var customer []model.SupportMessage
	for _, message := range messages {
		if message.SenderType == "customer" {
			customer = append(customer, message)
		}
	}
	return customer
}

func countConversations(t *testing.T, env *emailFallbackTestEnv, workspaceID string) int64 {
	t.Helper()
	var count int64
	if err := env.convRepo.DB().Model(&model.SupportConversation{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

// capturePostmark records sent emails; every send succeeds.
func capturePostmark(t *testing.T, env *emailFallbackTestEnv) *[]capturedPostmarkRequest {
	t.Helper()
	var sent []capturedPostmarkRequest
	env.service.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var request capturedPostmarkRequest
		if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		sent = append(sent, request)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"MessageID":"notice"}`))}, nil
	})})
	return &sent
}

func undeliverableReason(t *testing.T, err error) string {
	t.Helper()
	var undeliverable *repository.InboundUndeliverableError
	if !errors.As(err, &undeliverable) {
		t.Fatalf("expected an undeliverable email, got %v", err)
	}
	return undeliverable.Reason
}

func TestSharedSenderReplyJoinsItsConversationInAnyWorkspace(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	seedOtherWorkspace(t, env)
	const conversationID = "44444444-4444-4444-4444-444444444444"
	sentID := seedSharedSenderThread(t, env, otherTestWorkspaceID, conversationID, "Pricing")

	payload := sharedSenderPayload("reply", "Re: Pricing (#12)", "Thanks, that works.",
		model.PostmarkHeader{Name: "Message-ID", Value: "<reply@customer.test>"},
		model.PostmarkHeader{Name: "In-Reply-To", Value: sentID})
	if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
		t.Fatalf("process reply: %v", err)
	}
	if got := customerMessages(t, env, otherTestWorkspaceID, conversationID); len(got) != 1 || got[0].Content != "Thanks, that works." {
		t.Fatalf("expected the reply in its conversation, got %+v", got)
	}
}

func TestSharedSenderReplyIgnoresMessageIDsHelpinDidNotSend(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	const conversationID = "44444444-4444-4444-4444-444444444444"
	sentID := seedSharedSenderThread(t, env, sharedTestWorkspaceID, conversationID, "Pricing")
	// Same log ID, different domain: not the Message-ID Helpin wrote.
	forged := strings.Replace(sentID, "replies.helpin.ai", "attacker.test", 1)

	payload := sharedSenderPayload("forged", "Re: Something else", "Hello",
		model.PostmarkHeader{Name: "In-Reply-To", Value: forged})
	conv, err := env.service.conversationRepliedTo(ctx, payload)
	if err != nil || conv != nil {
		t.Fatalf("expected no match for a foreign Message-ID, got %+v, %v", conv, err)
	}
}

func TestSharedSenderReplyCopyIsAddedOnce(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	const conversationID = "44444444-4444-4444-4444-444444444444"
	sentID := seedSharedSenderThread(t, env, sharedTestWorkspaceID, conversationID, "Pricing")
	headers := []model.PostmarkHeader{
		{Name: "Message-ID", Value: "<one-email@customer.test>"},
		{Name: "In-Reply-To", Value: sentID},
	}

	toReplyAddress := sharedSenderPayload("reply-address", "Re: Pricing (#12)", "Sending to both.", headers...)
	replyAddress := "conv-" + conversationID + "@replies.helpin.ai"
	toReplyAddress.To, toReplyAddress.OriginalRecipient, toReplyAddress.MailboxHash = replyAddress, replyAddress, "conv-"+conversationID
	toReplyAddress.ToFull = []model.PostmarkAddress{{Email: replyAddress}}
	toShared := sharedSenderPayload("shared", "Re: Pricing (#12)", "Sending to both.", headers...)

	for _, payload := range []model.PostmarkInboundPayload{toShared, toReplyAddress} {
		if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
			t.Fatalf("process %s: %v", payload.MessageID, err)
		}
	}
	if got := customerMessages(t, env, sharedTestWorkspaceID, conversationID); len(got) != 1 {
		t.Fatalf("expected one message for two copies of one email, got %d", len(got))
	}
}

func TestSameTextSentSeparatelyToSharedSenderIsAddedOnce(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	const conversationID = "44444444-4444-4444-4444-444444444444"
	sentID := seedSharedSenderThread(t, env, sharedTestWorkspaceID, conversationID, "Pricing")

	first := sharedSenderPayload("first", "Re: Pricing (#12)", "Any update on this?",
		model.PostmarkHeader{Name: "Message-ID", Value: "<first@customer.test>"},
		model.PostmarkHeader{Name: "In-Reply-To", Value: sentID})
	replyAddress := "conv-" + conversationID + "@replies.helpin.ai"
	first.To, first.OriginalRecipient, first.MailboxHash = replyAddress, replyAddress, "conv-"+conversationID
	first.ToFull = []model.PostmarkAddress{{Email: replyAddress}}
	second := sharedSenderPayload("second", "Re: Pricing (#12)", "Any update on this?",
		model.PostmarkHeader{Name: "Message-ID", Value: "<second@customer.test>"},
		model.PostmarkHeader{Name: "In-Reply-To", Value: sentID})

	for _, payload := range []model.PostmarkInboundPayload{first, second} {
		if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
			t.Fatalf("process %s: %v", payload.MessageID, err)
		}
	}
	if got := customerMessages(t, env, sharedTestWorkspaceID, conversationID); len(got) != 1 {
		t.Fatalf("expected the same text sent to two addresses once, got %d", len(got))
	}
}

// Writing to both the tagged From and the Reply-To of one Helpin email must
// not add the message twice.
func TestTaggedSenderAndReplyToCopiesAreAddedOnce(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	const conversationID = "44444444-4444-4444-4444-444444444444"
	seedSharedSenderThread(t, env, sharedTestWorkspaceID, conversationID, "Pricing")

	for _, address := range []string{"noreply+conv-" + conversationID + "@example.com", "conv-" + conversationID + "@replies.helpin.ai"} {
		payload := sharedSenderPayload(address, "Re: Pricing (#12)", "Reply all.",
			model.PostmarkHeader{Name: "Message-ID", Value: "<reply-all@customer.test>"})
		payload.To, payload.OriginalRecipient, payload.MailboxHash = address, address, "conv-"+conversationID
		payload.ToFull = []model.PostmarkAddress{{Email: address}}
		if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
			t.Fatalf("process copy to %s: %v", address, err)
		}
	}
	if got := customerMessages(t, env, sharedTestWorkspaceID, conversationID); len(got) != 1 {
		t.Fatalf("expected one message, got %d", len(got))
	}
}

func TestSharedSenderNewEmailOpensConversationInTheOnlyWorkspace(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	seedOtherWorkspace(t, env)
	seedSharedSenderThread(t, env, otherTestWorkspaceID, "44444444-4444-4444-4444-444444444444", "Pricing")
	before := countConversations(t, env, otherTestWorkspaceID)

	payload := sharedSenderPayload("new", "A new question", "Do you support invoices?",
		model.PostmarkHeader{Name: "Message-ID", Value: "<new@customer.test>"})
	if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
		t.Fatalf("process new email: %v", err)
	}
	if after := countConversations(t, env, otherTestWorkspaceID); after != before+1 {
		t.Fatalf("expected a new conversation in the only workspace, had %d now %d", before, after)
	}
	if other := countConversations(t, env, sharedTestWorkspaceID); other != 0 {
		t.Fatalf("expected nothing in the unrelated workspace, got %d", other)
	}
	var conv model.SupportConversation
	if err := env.convRepo.DB().Where("workspace_id = ? AND subject = ?", otherTestWorkspaceID, "A new question").First(&conv).Error; err != nil {
		t.Fatal(err)
	}
	if conv.CustomerEmail == nil || *conv.CustomerEmail != sharedTestCustomer {
		t.Fatalf("unexpected customer on new conversation: %+v", conv.CustomerEmail)
	}
	var logRow model.SupportEmailLog
	if err := env.convRepo.DB().Where("conversation_id = ? AND direction = ?", conv.ID, "inbound").First(&logRow).Error; err != nil {
		t.Fatal(err)
	}
	if logRow.EmailRouteID != nil {
		t.Fatalf("shared sender delivery must not claim a route, got %v", *logRow.EmailRouteID)
	}
}

func TestSharedSenderNewEmailWithThreadSubjectJoinsThatThread(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	const conversationID = "44444444-4444-4444-4444-444444444444"
	seedSharedSenderThread(t, env, sharedTestWorkspaceID, conversationID, "Pricing")

	// Written by hand, so it has no In-Reply-To, but it names the thread.
	payload := sharedSenderPayload("by-hand", "RE: Pricing (#12)", "One more thing.")
	if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
		t.Fatalf("process email: %v", err)
	}
	if got := customerMessages(t, env, sharedTestWorkspaceID, conversationID); len(got) != 1 {
		t.Fatalf("expected the email in its thread, got %d messages", len(got))
	}
	if count := countConversations(t, env, sharedTestWorkspaceID); count != 1 {
		t.Fatalf("expected no new conversation, got %d", count)
	}
}

func TestSharedSenderNewEmailLeftToTheWorkspaceAddressCopy(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	seedSharedSenderThread(t, env, sharedTestWorkspaceID, "44444444-4444-4444-4444-444444444444", "Pricing")
	source := "help@acme.test"
	if err := env.routeRepo.Create(ctx, &model.SupportEmailRoute{
		WorkspaceID: sharedTestWorkspaceID, RouteKey: "route-acme", InboundAddress: "acme@acme.on.helpin.email",
		SourceAddress: &source, Active: true, CreatedByID: "22222222-2222-2222-2222-222222222222",
	}); err != nil {
		t.Fatal(err)
	}

	payload := sharedSenderPayload("cc", "A new question", "Copying both of you.")
	payload.To = source + ", " + sharedTestSender
	payload.ToFull = []model.PostmarkAddress{{Email: source}, {Email: sharedTestSender}}
	if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
		t.Fatalf("process email: %v", err)
	}
	if count := countConversations(t, env, sharedTestWorkspaceID); count != 1 {
		t.Fatalf("expected the copy sent to %s to deliver it, got %d conversations", source, count)
	}
}

func TestSharedSenderNewEmailNotDeliveredWhenSeveralWorkspacesSentFromIt(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackTestEnvWithRedis(t, model.DefaultSupportInboxSettings(), true)
	seedOtherWorkspace(t, env)
	seedSharedSenderThread(t, env, sharedTestWorkspaceID, "44444444-4444-4444-4444-444444444444", "Pricing")
	seedSharedSenderThread(t, env, otherTestWorkspaceID, "55555555-5555-5555-5555-555555555555", "Onboarding")
	sent := capturePostmark(t, env)

	payload := sharedSenderPayload("ambiguous", "Hello", "Who is this?",
		sharedTestAuthenticated, model.PostmarkHeader{Name: "Message-ID", Value: "<ambiguous@customer.test>"})
	reason := undeliverableReason(t, env.service.ProcessInboundEmail(ctx, payload, ""))
	if !strings.Contains(reason, "2 workspaces have emailed this sender") || !strings.Contains(reason, "The sender was told") {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if countConversations(t, env, sharedTestWorkspaceID) != 1 || countConversations(t, env, otherTestWorkspaceID) != 1 {
		t.Fatal("an ambiguous email must not open a conversation in either workspace")
	}
	if len(*sent) != 1 {
		t.Fatalf("expected one notice, got %d", len(*sent))
	}
	notice := (*sent)[0]
	if notice.To != sharedTestCustomer || notice.Subject != "Not delivered: Hello" || notice.ReplyTo != "" {
		t.Fatalf("unexpected notice: %+v", notice)
	}
	if strings.Contains(notice.TextBody, "Acme") || strings.Contains(notice.TextBody, "Globex") {
		t.Fatal("the notice must not name a workspace")
	}
	headers := map[string]string{}
	for _, header := range notice.Headers {
		headers[header.Name] = header.Value
	}
	if headers["Auto-Submitted"] != "auto-replied" || headers["In-Reply-To"] != "<ambiguous@customer.test>" {
		t.Fatalf("notice headers missing: %+v", headers)
	}

	again := sharedSenderPayload("ambiguous-again", "Hello?", "Anyone?", sharedTestAuthenticated)
	if reason := undeliverableReason(t, env.service.ProcessInboundEmail(ctx, again, "")); !strings.Contains(reason, "already got one") {
		t.Fatalf("expected the daily limit, got %q", reason)
	}
	if len(*sent) != 1 {
		t.Fatalf("expected no second notice the same day, got %d", len(*sent))
	}
}

// Many recent emails from one workspace must not hide an older one from
// another workspace: that person is still ambiguous.
func TestSharedSenderNewEmailCountsEveryWorkspaceThatSentFromIt(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	seedOtherWorkspace(t, env)
	seedSharedSenderThread(t, env, otherTestWorkspaceID, "55555555-5555-5555-5555-555555555555", "Onboarding")
	const conversationID = "44444444-4444-4444-4444-444444444444"
	seedSharedSenderThread(t, env, sharedTestWorkspaceID, conversationID, "Pricing")
	for i := 0; i < sharedSenderHistoryLimit; i++ {
		if err := env.emailLogRepo.Create(ctx, &model.SupportEmailLog{
			WorkspaceID: sharedTestWorkspaceID, ConversationID: conversationID, Direction: "outbound",
			FromEmail: sharedTestSender, ToEmail: strings.ToUpper(sharedTestCustomer), Subject: "Update", Status: "sent",
		}); err != nil {
			t.Fatal(err)
		}
	}

	payload := sharedSenderPayload("many", "Hello", "Who is this?")
	if reason := undeliverableReason(t, env.service.ProcessInboundEmail(ctx, payload, "")); !strings.Contains(reason, "2 workspaces") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestSharedSenderNoticeSkippedWhenSenderIsNotConfirmed(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackTestEnvWithRedis(t, model.DefaultSupportInboxSettings(), true)
	sent := capturePostmark(t, env)

	payload := sharedSenderPayload("unverified", "Hello", "Who is this?",
		model.PostmarkHeader{Name: "Received-SPF", Value: "Pass (sender SPF authorized) identity=mailfrom; envelope-from=bounce@bulk-mailer.test; receiver=x"})
	reason := undeliverableReason(t, env.service.ProcessInboundEmail(ctx, payload, ""))
	if !strings.Contains(reason, "No workspace has emailed this sender") || !strings.Contains(reason, "didn't confirm the address") {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if len(*sent) != 0 {
		t.Fatalf("a notice must not go to an unconfirmed sender, got %d", len(*sent))
	}
}

func TestSharedSenderAutomaticEmailIsIgnored(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackTestEnvWithRedis(t, model.DefaultSupportInboxSettings(), true)
	sent := capturePostmark(t, env)

	for _, payload := range []model.PostmarkInboundPayload{
		sharedSenderPayload("ooo", "Automatic reply", "I am away.", sharedTestAuthenticated, model.PostmarkHeader{Name: "Auto-Submitted", Value: "auto-replied"}),
		sharedSenderPayload("newsletter", "Our news", "Read more.", sharedTestAuthenticated, model.PostmarkHeader{Name: "List-Id", Value: "<news.example.test>"}),
	} {
		if err := env.service.ProcessInboundEmail(ctx, payload, ""); err != nil {
			t.Fatalf("automatic email %s: %v", payload.MessageID, err)
		}
	}
	// Helpin's own delivery check writes from the shared sender to itself.
	check := sharedSenderPayload("check", "Delivery check", "Ping.", sharedTestAuthenticated)
	check.FromFull.Email, check.From = sharedTestSender, sharedTestSender
	if err := env.service.ProcessInboundEmail(ctx, check, ""); err != nil {
		t.Fatalf("Helpin delivery check: %v", err)
	}
	// Another address on the shared sender's domain may be a real person (a
	// self-hosted install sends from its own domain), but never gets a notice.
	colleague := sharedSenderPayload("colleague", "Question", "Is this working?", sharedTestAuthenticated)
	colleague.FromFull.Email, colleague.From = "colleague@example.com", "colleague@example.com"
	if reason := undeliverableReason(t, env.service.ProcessInboundEmail(ctx, colleague, "")); !strings.Contains(reason, "the sender is a Helpin address") {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if len(*sent) != 0 {
		t.Fatalf("automatic or internal email must never get a notice, got %d", len(*sent))
	}
}

func TestUnknownHelpinAddressFailsOnceWithReason(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	payload := sharedSenderPayload("old-route", "Hello", "Is anyone there?")
	payload.To, payload.OriginalRecipient = "old@acme.on.helpin.email", "old@acme.on.helpin.email"
	payload.ToFull = []model.PostmarkAddress{{Email: "old@acme.on.helpin.email"}}
	if reason := undeliverableReason(t, env.service.ProcessInboundEmail(ctx, payload, "")); !strings.Contains(reason, "old@acme.on.helpin.email") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

// An undeliverable email fails at once with its reason: it never sits in the
// retry backlog (which would raise the stuck-email alert) and admins see why.
func TestUndeliverableSharedSenderEmailFailsOnceForAdmins(t *testing.T) {
	ctx := context.Background()
	env := setupEmailFallbackInboundTestEnv(t, model.DefaultSupportInboxSettings())
	payload := sharedSenderPayload("queued", "Hello", "Who is this?")
	if err := env.service.AcceptInboundEmail(ctx, payload, ""); err != nil {
		t.Fatal(err)
	}
	db := env.convRepo.DB()
	var job model.SupportInboundJob
	if err := db.Where("kind = ?", "email").First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if wait := job.AvailableAt.Sub(job.CreatedAt); wait < 50*time.Second {
		t.Fatalf("expected a shared sender copy to wait about a minute, waits %s", wait)
	}
	if err := db.Model(&model.SupportInboundJob{}).Where("id = ?", job.ID).Update("available_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if err := env.service.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", job.ID).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != "failed" || job.Attempts != 1 || !strings.Contains(job.LastError, "No workspace has emailed this sender") {
		t.Fatalf("expected one failed attempt with a reason, got status %q attempts %d error %q", job.Status, job.Attempts, job.LastError)
	}
	backlog, err := repository.NewSupportInboundJobRepository(db).WaitingSummary(ctx, "email")
	if err != nil {
		t.Fatal(err)
	}
	if backlog.Waiting != 0 {
		t.Fatalf("an undeliverable email must not count as waiting, got %d", backlog.Waiting)
	}
	detail, err := env.service.GetAdminInboundEmail(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.NotDeliveredReason != job.LastError || !detail.RequiresDestination {
		t.Fatalf("admin detail should explain the email: %+v", detail)
	}
}

func TestHelpinEmailLogIDFromMessageID(t *testing.T) {
	const id = "8f9c2b8e-1f0a-4c55-9a51-0b8e9a3f2d10"
	tests := map[string]string{
		"<helpin-" + id + "@replies.helpin.email>": id,
		"helpin-" + id + "@replies.helpin.email":   id,
		"<helpin-not-a-uuid@replies.helpin.email>": "",
		"<" + id + "@replies.helpin.email>":        "",
		"<CAF=abc@mail.gmail.com>":                 "",
		"":                                         "",
	}
	for reference, want := range tests {
		if got := helpinEmailLogIDFromMessageID(reference); got != want {
			t.Errorf("helpinEmailLogIDFromMessageID(%q) = %q, want %q", reference, got, want)
		}
	}
}

func TestNormalizedEmailThreadSubject(t *testing.T) {
	tests := map[string]string{
		"Re: Pricing (#12)":         "pricing (#12)",
		"RE: Fwd:  Pricing   (#12)": "pricing (#12)",
		"AW: Re: Pricing (#12)":     "pricing (#12)",
		"Pricing (#12)":             "pricing (#12)",
		"Retainer question":         "retainer question",
		"   ":                       "",
	}
	for subject, want := range tests {
		if got := normalizedEmailThreadSubject(subject); got != want {
			t.Errorf("normalizedEmailThreadSubject(%q) = %q, want %q", subject, got, want)
		}
	}
}

func TestInboundSenderAuthenticated(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		header model.PostmarkHeader
		want   bool
	}{
		{name: "dkim from the sender's domain", from: "a@customer.test", header: sharedTestAuthenticated, want: true},
		{name: "dkim from another domain only", from: "a@customer.test", header: model.PostmarkHeader{Name: "X-Spam-Tests", Value: "DKIM_SIGNED,DKIM_VALID,SPF_PASS"}},
		{name: "spf for the same domain", from: "a@customer.test", header: model.PostmarkHeader{Name: "Received-SPF", Value: "Pass (sender SPF authorized) identity=mailfrom; client-ip=1.2.3.4; envelope-from=a@customer.test; receiver=x"}, want: true},
		{name: "spf for a subdomain", from: "a@customer.test", header: model.PostmarkHeader{Name: "Received-SPF", Value: "pass identity=mailfrom; envelope-from=<bounces@mail.customer.test>"}, want: true},
		{name: "spf for a different domain", from: "a@customer.test", header: model.PostmarkHeader{Name: "Received-SPF", Value: "Pass identity=mailfrom; envelope-from=x@mailer.test"}},
		{name: "spf fail", from: "a@customer.test", header: model.PostmarkHeader{Name: "Received-SPF", Value: "Fail identity=mailfrom; envelope-from=a@customer.test"}},
		{name: "no evidence", from: "a@customer.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := model.PostmarkInboundPayload{FromFull: model.PostmarkAddress{Email: tt.from}}
			if tt.header.Name != "" {
				payload.Headers = []model.PostmarkHeader{tt.header}
			}
			if got := inboundSenderAuthenticated(payload); got != tt.want {
				t.Fatalf("inboundSenderAuthenticated() = %v, want %v", got, tt.want)
			}
		})
	}
}
