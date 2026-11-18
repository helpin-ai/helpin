package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportTranscriptRendersRichMessages(t *testing.T) {
	conversation := &model.SupportConversation{DisplayID: 42, Subject: "Billing & renewal"}
	messages := []model.SupportMessage{
		{SenderType: "customer", MessageType: "reply", SenderDisplayName: strPtr("Ada"),
			Content: "Fallback", HTMLBody: `<p>Hello <strong>team</strong>.</p><table><tr><th>Plan</th><th>Seats</th></tr><tr><td>Pro</td><td>3</td></tr></table><p><a href="https://example.com/invoice">Invoice</a></p><div class="gmail_quote">OLD QUOTED HISTORY</div><script>UNSAFE SCRIPT</script>`, CreatedAt: time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)},
		{SenderType: "ai", MessageType: "reply", SenderDisplayName: strPtr("Atlas"),
			Content: "**Your plan is active.**\n\n- Three seats\n- Monthly billing\n\n[Manage plan](https://example.com/billing)\n\n```js\nconst seats = 3;\n```", Metadata: `{"ai_reply_kind":"answer"}`},
		{SenderType: "user", MessageType: "reply", SenderDisplayName: strPtr("Alex"), Content: `<p>Here is your <em>receipt</em>.</p>`},
	}
	htmlBody, textBody := renderSupportTranscriptBodies("Acme", "", conversation, messages)
	for _, want := range []string{"<strong>team</strong>", "<table", "<td", "Pro", "<strong>Your plan is active.</strong>", "<ul", "<li", "<pre", "<code", "<em>receipt</em>", "Atlas", "Nov 18, 2026 10:00 UTC", `href="https://example.com/invoice"`} {
		if !strings.Contains(htmlBody, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	for _, excluded := range []string{"OLD QUOTED HISTORY", "UNSAFE SCRIPT", "<script", "&lt;p&gt;", "Helpin AI"} {
		if strings.Contains(htmlBody, excluded) || strings.Contains(textBody, excluded) {
			t.Errorf("transcript contains %q", excluded)
		}
	}
	if strings.Contains(textBody, "<p>") || !strings.Contains(textBody, "receipt") {
		t.Errorf("invalid text alternative: %s", textBody)
	}
	if !strings.Contains(textBody, "Plan | Seats") || !strings.Contains(textBody, "Pro | 3") {
		t.Error("plain-text table lost its rows")
	}
	if !strings.Contains(textBody, "Atlas · AI assistant") {
		t.Error("plain-text version lost AI attribution")
	}
}

func TestSupportTranscriptIncludesOnlyPublicConversationContent(t *testing.T) {
	for _, tc := range []struct {
		name     string
		message  model.SupportMessage
		included bool
	}{
		{"customer reply", model.SupportMessage{SenderType: "customer", MessageType: "reply"}, true},
		{"legacy reply", model.SupportMessage{SenderType: "user"}, true},
		{"AI answer", model.SupportMessage{SenderType: "ai", MessageType: "reply", Metadata: `{"ai_reply_kind":"answer"}`}, true},
		{"AI clarification", model.SupportMessage{SenderType: "ai", MessageType: "reply", Metadata: `{"ai_reply_kind":"clarify"}`}, true},
		{"AI greeting", model.SupportMessage{SenderType: "ai", MessageType: "reply", Metadata: `{"ai_reply_kind":"greeting"}`}, true},
		{"AI follow up", model.SupportMessage{SenderType: "ai", MessageType: "reply", Metadata: `{"ai_reply_kind":"inactivity_follow_up"}`}, true},
		{"AI final reply with progress metadata", model.SupportMessage{SenderType: "ai", MessageType: "reply", Metadata: `{"ai_reply_kind":"answer","ai_progress_state":"resolved"}`}, true},
		{"AI progress", model.SupportMessage{SenderType: "ai", MessageType: "reply", Metadata: `{"ai_reply_kind":"conversational","ai_progress_state":"checking"}`}, false},
		{"internal AI draft", model.SupportMessage{SenderType: "ai", MessageType: "reply", IsInternal: true}, false},
		{"internal reply", model.SupportMessage{SenderType: "user", MessageType: "reply", IsInternal: true}, false},
		{"note without internal flag", model.SupportMessage{SenderType: "user", MessageType: "note"}, false},
		{"status", model.SupportMessage{SenderType: "user", MessageType: "system"}, false},
		{"system sender", model.SupportMessage{SenderType: "system", MessageType: "reply"}, false},
		{"typed event", model.SupportMessage{SenderType: "ai", MessageType: "reply", SystemEventType: strPtr("ai_escalated")}, false},
		{"survey", model.SupportMessage{SenderType: "ai", MessageType: "csat_survey"}, false},
		{"email notice", model.SupportMessage{SenderType: "customer", MessageType: "email_notice"}, false},
		{"deleted", model.SupportMessage{SenderType: "customer", MessageType: "reply", DeletedAt: gorm.DeletedAt{Valid: true}}, false},
		{"unknown type", model.SupportMessage{SenderType: "ai", MessageType: "future_internal_event"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.message.Content = "unique message body"
			htmlBody, textBody := renderSupportTranscriptBodies("Acme", "", &model.SupportConversation{}, []model.SupportMessage{tc.message})
			for _, body := range []string{htmlBody, textBody} {
				if strings.Contains(body, tc.message.Content) != tc.included {
					t.Errorf("included = %v, want %v", strings.Contains(body, tc.message.Content), tc.included)
				}
			}
		})
	}
}

func TestSupportTranscriptPreservesAttachmentOnlyReplies(t *testing.T) {
	messages := []model.SupportMessage{{SenderType: "customer", MessageType: "reply", Attachments: []model.SupportAttachmentPayload{{FileName: "receipt.pdf", URL: "https://example.com/receipt.pdf"}, {FileName: "unavailable.txt"}, {FileName: "unsafe.txt", URL: "javascript:alert(1)"}}}}
	htmlBody, textBody := renderSupportTranscriptBodies("Acme", "", &model.SupportConversation{}, messages)
	for _, name := range []string{"receipt.pdf", "unavailable.txt", "unsafe.txt"} {
		if !strings.Contains(htmlBody, name) || !strings.Contains(textBody, name) {
			t.Errorf("missing attachment %q", name)
		}
	}
	if !strings.Contains(htmlBody, `href="https://example.com/receipt.pdf"`) {
		t.Error("missing attachment download link")
	}
	if strings.Contains(htmlBody, "javascript:") || strings.Contains(textBody, "javascript:") {
		t.Error("unsafe attachment link")
	}
}

func setupSupportTranscriptTest(t *testing.T) (*SupportInboxService, *model.SupportConversation, *capturedPostmarkRequest) {
	t.Helper()
	db := newTestDB(t)
	const ws = "transcript-workspace"
	seedWorkspace(t, db, ws, "Acme Support", "acme", "user-123")
	convRepo := repository.NewSupportConversationRepository(db)
	conversation := &model.SupportConversation{WorkspaceID: ws, Subject: "Billing question", Status: "open", Channel: "email", CustomerName: strPtr("Ada"), CustomerEmail: strPtr("ada@example.com"), CRMContactID: strPtr("original-contact")}
	if err := convRepo.Create(context.Background(), conversation); err != nil {
		t.Fatal(err)
	}
	svc := NewSupportInboxService(convRepo, repository.NewSupportMailboxRepository(db), repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), repository.NewSupportInboxSessionRepository(db), nil, nil, nil, nil, nil, nil, nil, nil)
	svc.workspaceRepo = repository.NewWorkspaceRepository(db)
	captured := &capturedPostmarkRequest{}
	client := email.NewClient("test-token", "noreply@example.com")
	client.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(req.Body).Decode(captured); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK","MessageID":"test-transcript"}`))}, nil
	})})
	svc.SetEmailFallbackService(NewEmailFallbackService(nil, nil, nil, client, nil, nil, nil, nil, nil, nil, nil, "", "", ""))
	return svc, conversation, captured
}

func TestSendSupportTranscriptPreservesCustomerIdentity(t *testing.T) {
	svc, conv, captured := setupSupportTranscriptTest(t)
	// Old clients may still send the removed flag; it must have no effect.
	var req model.SendSupportConversationTranscriptRequest
	if err := json.Unmarshal([]byte(`{"email":"colleague@example.com","update_customer_email":true}`), &req); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendSupportConversationTranscript(context.Background(), conv.WorkspaceID, conv.ID, req.Email, ""); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.conversationRepo.GetByID(context.Background(), conv.WorkspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if derefString(updated.CustomerEmail) != "ada@example.com" || derefString(updated.CustomerName) != "Ada" || derefString(updated.CRMContactID) != "original-contact" {
		t.Error("sending transcript changed customer identity")
	}
	if captured.To != "colleague@example.com" {
		t.Errorf("recipient = %q", captured.To)
	}
}

func TestSendSupportTranscriptNamesTheTeammate(t *testing.T) {
	svc, conv, captured := setupSupportTranscriptTest(t)
	db := svc.conversationRepo.DB()
	if err := db.Exec("INSERT INTO users (id,email,password_hash,full_name) VALUES (?,?,?,?)", "sender", "alex@example.com", "test", "Alex Morgan").Error; err != nil {
		t.Fatal(err)
	}
	svc.userRepo = repository.NewUserRepository(db)
	if _, err := svc.SendSupportConversationTranscript(context.Background(), conv.WorkspaceID, conv.ID, "colleague@example.com", "sender"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{captured.HtmlBody, captured.TextBody} {
		if !strings.Contains(body, "Alex Morgan from Acme Support sent you a copy of this conversation.") {
			t.Error("missing teammate attribution")
		}
	}
}

func TestSendSupportTranscriptValidatesRecipient(t *testing.T) {
	for _, recipient := range []string{"not-an-email", "a@example.com,b@example.com", "a@example.com\r\nBcc: hidden@example.com"} {
		t.Run(recipient, func(t *testing.T) {
			svc, conv, captured := setupSupportTranscriptTest(t)
			if _, err := svc.SendSupportConversationTranscript(context.Background(), conv.WorkspaceID, conv.ID, recipient, ""); err == nil {
				t.Error("invalid recipient accepted")
			}
			if captured.To != "" {
				t.Error("sent to invalid recipient")
			}
		})
	}
}

func TestSendSupportTranscriptRejectsOtherWorkspace(t *testing.T) {
	svc, conv, captured := setupSupportTranscriptTest(t)
	if _, err := svc.SendSupportConversationTranscript(context.Background(), "other-workspace", conv.ID, "colleague@example.com", ""); err == nil {
		t.Error("other workspace conversation accepted")
	}
	if captured.To != "" {
		t.Error("sent another workspace's transcript")
	}
}

func TestSendSupportTranscriptDoesNotIdentifyAnonymousCustomer(t *testing.T) {
	svc, conv, captured := setupSupportTranscriptTest(t)
	if err := svc.conversationRepo.UpdateFields(context.Background(), conv.WorkspaceID, conv.ID, map[string]any{"customer_email": nil}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendSupportConversationTranscript(context.Background(), conv.WorkspaceID, conv.ID, "colleague@example.com", ""); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.conversationRepo.GetByID(context.Background(), conv.WorkspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CustomerEmail != nil {
		t.Error("transcript recipient identified anonymous customer")
	}
	if captured.To != "colleague@example.com" {
		t.Error("transcript was not sent")
	}
}

func TestSendSupportTranscriptAttachesOnlyPublicFiles(t *testing.T) {
	svc, conv, _ := setupSupportTranscriptTest(t)
	svc.emailFallbackService.attachmentDownloader = fakeSupportEmailAttachmentDownloader{data: map[string][]byte{"public-file": []byte("public pdf"), "private-file": []byte("private pdf")}}
	var captured struct {
		Attachments []email.Attachment
		HtmlBody    string
	}
	svc.emailFallbackService.emailClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(req.Body).Decode(&captured); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ErrorCode":0,"Message":"OK"}`))}, nil
	})})
	attachment := func(id, name string) model.SupportAttachmentPayload {
		return model.SupportAttachmentPayload{ID: id, FileName: name, FileKey: "support/" + id, FileType: "application/pdf", FileSize: 10, URL: "https://example.com/" + id}
	}
	messages := []model.SupportMessage{
		{SenderType: "user", MessageType: "reply", Content: "Receipt attached.", Attachments: []model.SupportAttachmentPayload{attachment("public-file", "receipt.pdf")}},
		{SenderType: "customer", MessageType: "reply", Attachments: []model.SupportAttachmentPayload{{ID: "large-file", FileName: "recording.mp4", FileKey: "support/large-file", FileSize: supportEmailAttachmentMaxFileBytes + 1, URL: "https://example.com/large-file"}}},
		{SenderType: "user", MessageType: "note", IsInternal: true, Content: "Private note", Attachments: []model.SupportAttachmentPayload{attachment("private-file", "private.pdf")}},
		{SenderType: "ai", MessageType: "reply", Content: "Checking...", Metadata: `{"ai_progress_state":"checking"}`, Attachments: []model.SupportAttachmentPayload{attachment("private-file", "internal-check.pdf")}},
	}
	if err := svc.sendConversationTranscript(context.Background(), conv.WorkspaceID, conv, "colleague@example.com", "", messages); err != nil {
		t.Fatal(err)
	}
	if len(captured.Attachments) != 1 || captured.Attachments[0].Name != "receipt.pdf" {
		t.Fatalf("attachments = %+v", captured.Attachments)
	}
	if strings.Contains(captured.HtmlBody, "private.pdf") || strings.Contains(captured.HtmlBody, "https://example.com/public-file") {
		t.Error("private file or temporary link leaked into transcript")
	}
	if !strings.Contains(captured.HtmlBody, `href="https://example.com/large-file"`) {
		t.Error("large attachment lost its download link")
	}
}

func TestSendWidgetConversationTranscriptFiltersHistoricalProgressAndEmailOnlyReplies(t *testing.T) {
	svc, conv, captured := setupSupportTranscriptTest(t)
	ctx := context.Background()
	if err := svc.conversationRepo.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"anonymous_id": "transcript-visitor"}); err != nil {
		t.Fatal(err)
	}
	session := &model.SupportWidgetSession{WorkspaceID: conv.WorkspaceID, SessionToken: "transcript-session", AnonymousID: "transcript-visitor", ExpiresAt: time.Now().Add(time.Hour)}
	if err := svc.sessionRepo.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	for _, msg := range []model.SupportMessage{
		{SenderType: "ai", MessageType: "reply", Content: "Historical checking update", Metadata: `{"ai_progress_state":"checking"}`},
		{SenderType: "ai", MessageType: "reply", Content: "Public AI answer", Metadata: `{"ai_reply_kind":"answer"}`},
		{SenderType: "user", MessageType: "reply", Content: "Email audience only", Metadata: `{"delivery_mode":"email_only"}`},
		{SenderType: "user", MessageType: "note", Content: "Private agent note", IsInternal: true},
	} {
		msg.WorkspaceID, msg.ConversationID = conv.WorkspaceID, conv.ID
		if err := svc.messageRepo.Create(ctx, &msg); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.SendWidgetConversationTranscript(ctx, session.SessionToken, conv.ID, "ada@example.com"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{captured.HtmlBody, captured.TextBody} {
		if !strings.Contains(body, "Public AI answer") || !strings.Contains(body, "Here is a copy of the conversation") {
			t.Error("widget transcript missing answer or neutral introduction")
		}
		for _, excluded := range []string{"Historical checking update", "Email audience only", "Private agent note"} {
			if strings.Contains(body, excluded) {
				t.Errorf("widget transcript leaked %q", excluded)
			}
		}
	}
}

func TestSendSupportTranscriptIncludesEmailOnlyReplies(t *testing.T) {
	svc, conv, captured := setupSupportTranscriptTest(t)
	msg := &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "user", MessageType: "reply", Content: "Email-only answer", Metadata: `{"delivery_mode":"email_only"}`}
	if err := svc.messageRepo.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendSupportConversationTranscript(context.Background(), conv.WorkspaceID, conv.ID, "colleague@example.com", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(captured.HtmlBody, "Email-only answer") || !strings.Contains(captured.TextBody, "Email-only answer") {
		t.Error("email-only reply missing from transcript")
	}
	if captured.Subject != "Transcript: Billing question — Acme Support" {
		t.Errorf("subject = %q", captured.Subject)
	}
}

func TestSupportTranscriptPreservesCodeThatLooksLikeHTML(t *testing.T) {
	msg := model.SupportMessage{SenderType: "customer", MessageType: "reply", Content: "The template fails:\n\n```html\n<div class=\"panel\">Example</div>\n```"}
	htmlBody, textBody := renderSupportTranscriptBodies("Acme", "", &model.SupportConversation{}, []model.SupportMessage{msg})
	if !strings.Contains(htmlBody, "&lt;div") || !strings.Contains(textBody, `<div class="panel">Example</div>`) {
		t.Error("HTML code example was altered")
	}
}

func TestSupportTranscriptRemovesEmailPreviewPadding(t *testing.T) {
	padding := strings.Repeat("\u00a0\u200c\u200b\u200d\u200e\u200f\ufeff", 30)
	msg := model.SupportMessage{SenderType: "customer", MessageType: "reply", EmailVisibleText: "Subscription confirmed\n\n" + padding + "\n\nThanks 👩‍💻"}
	htmlBody, textBody := renderSupportTranscriptBodies("Acme", "", &model.SupportConversation{}, []model.SupportMessage{msg})
	for _, body := range []string{htmlBody, textBody} {
		if strings.Contains(body, padding) {
			t.Error("email preview padding leaked into transcript")
		}
		if !strings.Contains(body, "Thanks 👩‍💻") {
			t.Error("visible text or emoji was altered")
		}
	}
}

func TestSupportTranscriptUntrustedContent(t *testing.T) {
	msg := model.SupportMessage{SenderType: "customer", MessageType: "reply", SenderDisplayName: strPtr(`<img src=x onerror=alert(1)>`), Content: "[Unsafe](javascript:alert%281%29)\n\n[Safe](https://example.com/help)\n\n**Readable** text."}
	htmlBody, textBody := renderSupportTranscriptBodies("<Acme>", "<Alex>", &model.SupportConversation{Subject: `<script>alert(1)</script>`}, []model.SupportMessage{msg})
	for _, unsafe := range []string{`href="javascript:`, `<script>`, `<img src="x"`} {
		if strings.Contains(htmlBody, unsafe) {
			t.Errorf("unsafe HTML %q", unsafe)
		}
	}
	if strings.Contains(textBody, "javascript:") || strings.Contains(textBody, "**Readable**") {
		t.Error("plain text should use sanitized, formatted content")
	}
	if !strings.Contains(textBody, "https://example.com/help") {
		t.Error("plain text lost safe link")
	}
	if !strings.Contains(htmlBody, "&lt;Alex&gt; from &lt;Acme&gt; sent you a copy") {
		t.Error("missing escaped sender attribution")
	}
}

func TestSupportTranscriptSubjectFallbacks(t *testing.T) {
	for _, tc := range []struct {
		subject, workspace string
		id                 int
		want               string
	}{
		{"Billing question", "Acme", 42, "Transcript: Billing question — Acme"},
		{"", "Acme", 42, "Transcript: Conversation #42 — Acme"},
		{"", "", 0, "Transcript: Conversation — Support"},
		{"Billing\r\nquestion", "Acme\nSupport", 0, "Transcript: Billing question — Acme Support"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			got := supportTranscriptSubject(tc.workspace, &model.SupportConversation{Subject: tc.subject, DisplayID: tc.id})
			if got != tc.want {
				t.Errorf("subject = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSupportTranscriptEmptyConversation(t *testing.T) {
	htmlBody, textBody := renderSupportTranscriptBodies("Acme", "", &model.SupportConversation{}, []model.SupportMessage{{SenderType: "customer", MessageType: "reply", Content: "  "}})
	for _, body := range []string{htmlBody, textBody} {
		if !strings.Contains(body, "No public messages") || strings.Contains(body, "Jan 1, 0001") || strings.Contains(body, "#0") {
			t.Error("invalid empty conversation transcript")
		}
	}
}
