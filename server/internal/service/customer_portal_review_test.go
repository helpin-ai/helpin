package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func portalRequestConversationID(t *testing.T, db *gorm.DB, reference string) string {
	t.Helper()
	var conversationID string
	if err := db.Table("support_portal_request_references").Where("reference = ?", reference).Pluck("conversation_id", &conversationID).Error; err != nil || conversationID == "" {
		t.Fatalf("conversation for %s: %q %v", reference, conversationID, err)
	}
	return conversationID
}

func insertPortalTestMessage(t *testing.T, db *gorm.DB, workspaceID, conversationID, id, sender, content, metadata string, at time.Time) {
	t.Helper()
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, sender_display_name, message_type, content, metadata, created_at) VALUES (?, ?, ?, ?, 'Priya', 'reply', ?, ?, ?)`,
		id, workspaceID, conversationID, sender, content, metadata, at).Error; err != nil {
		t.Fatal(err)
	}
}

// Merged requests keep the replies shared into them, as the chat widget does,
// and a merged inbound email shows only its customer-safe text.
func TestCustomerPortalMergedRequestShowsSharedReplies(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Export broken", "The export fails.", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	source := portalRequestConversationID(t, db, request.Reference)

	// The request was merged into another conversation, whose replies are
	// shared back to it through the merge ledger.
	target := uuid.NewString()
	if err := repository.NewSupportConversationRepository(db).Create(ctx, &model.SupportConversation{
		ID: target, WorkspaceID: ws.ID, Subject: "Export issues", Status: "open", Priority: "medium", Channel: "email", Source: "email",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE support_conversations SET merged_into_id = ?, merge_revision = 1 WHERE id = ?`, target, source).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	insertPortalTestMessage(t, db, ws.ID, target, "shared-reply", "user", "We fixed the export.", `{}`, now.Add(time.Minute))
	insertPortalTestMessage(t, db, ws.ID, target, "shared-email", "customer", "Thanks!\n> quoted history from another customer", `{"merge_public_text":"Thanks!"}`, now.Add(2*time.Minute))
	mergeID := uuid.NewString()
	for _, messageID := range []string{"shared-reply", "shared-email"} {
		if err := db.Create(&model.SupportMergeMessage{WorkspaceID: ws.ID, ConversationID: source, MessageID: messageID, MergeID: mergeID}).Error; err != nil {
			t.Fatal(err)
		}
	}

	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil {
		t.Fatal(err)
	}
	var contents []string
	for _, message := range detail.Messages {
		contents = append(contents, message.Content)
	}
	joined := strings.Join(contents, " | ")
	if !strings.Contains(joined, "We fixed the export.") || !strings.Contains(joined, "Thanks!") {
		t.Fatalf("merged request lost shared replies: %s", joined)
	}
	if strings.Contains(joined, "quoted history") {
		t.Fatalf("merged email exposed its raw content: %s", joined)
	}

	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 || requests[0].LastMessagePreview != "Thanks!" {
		t.Fatalf("merged request preview = %+v %v", requests, err)
	}
}

// An email thread the customer sees in the portal can be answered there: a
// portal (chat) reply is accepted and appears in the request. Before, any chat
// mode was refused with "no chat session", leaving only email-only replies the
// portal hides.
func TestCustomerPortalEmailThreadReceivesPortalReplies(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	email := "customer@example.com"
	threadID := uuid.NewString()
	if err := repository.NewSupportConversationRepository(db).Create(ctx, &model.SupportConversation{
		ID: threadID, WorkspaceID: ws.ID, Subject: "Invoice question", Status: "open", Priority: "medium",
		Channel: "email", Source: "email", CustomerEmail: &email,
	}); err != nil {
		t.Fatal(err)
	}
	// Signing in claims the customer's email threads into the portal.
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, email, time.Now().Add(-time.Hour)))
	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 {
		t.Fatalf("email thread not in the portal: %+v %v", requests, err)
	}

	staff := "staff-1"
	reply, err := svc.inbox.CreateConversationMessage(ctx, ws.ID, threadID, model.CreateMessageRequest{
		Content: "Your invoice is attached.", DeliveryMode: model.SupportDeliveryChatOnly,
	}, "user", &staff, nil, stringPtr("Priya"))
	if err != nil {
		t.Fatalf("portal-visible email thread refused a portal reply: %v", err)
	}
	if reply.DeliveryMode() == model.SupportDeliveryEmailOnly {
		t.Fatalf("reply stored as email-only: %s", reply.Metadata)
	}
	detail, err := svc.RequestDetail(ctx, access, requests[0].Reference)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, message := range detail.Messages {
		found = found || message.Content == "Your invoice is attached."
	}
	if !found {
		t.Fatalf("staff reply missing from the portal: %+v", detail.Messages)
	}
}

// The list preview follows the detail view: an email-only reply is neither
// previewed nor counted as a new reply.
func TestCustomerPortalListPreviewSkipsEmailOnlyReplies(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Export broken", "The export fails.", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	conversationID := portalRequestConversationID(t, db, request.Reference)
	at := time.Now().UTC().Add(time.Minute)
	insertPortalTestMessage(t, db, ws.ID, conversationID, "email-only", "user", "Internal-looking email detail", `{"delivery_mode":"email_only"}`, at)
	if err := db.Exec(`UPDATE support_conversations SET last_public_message_id = 'email-only', last_public_sender_type = 'user', last_public_message_at = ? WHERE id = ?`, at, conversationID).Error; err != nil {
		t.Fatal(err)
	}

	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 {
		t.Fatalf("list: %+v %v", requests, err)
	}
	got := requests[0]
	if got.LastMessagePreview != "The export fails." || got.LastMessageFrom != "customer" || got.Unread {
		t.Fatalf("email-only reply leaked into the list: preview=%q from=%q unread=%v", got.LastMessagePreview, got.LastMessageFrom, got.Unread)
	}
	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range detail.Messages {
		if message.Content == "Internal-looking email detail" {
			t.Fatal("email-only reply shown in the request")
		}
	}
}
