package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type portalSentEmail struct{ to, subject, text string }

type recordingPortalSender struct{ sent []portalSentEmail }

func (s *recordingPortalSender) SendEmail(to, subject, _, textBody string) error {
	s.sent = append(s.sent, portalSentEmail{to: to, subject: subject, text: textBody})
	return nil
}

func setupCustomerPortalService(t *testing.T) (*CustomerPortalService, *gorm.DB, *recordingPortalSender, *PortalWorkspace) {
	t.Helper()
	db := newTestDB(t)
	createPortalTables(t, db)
	workspaceID := uuid.NewString()
	if err := db.Exec(`INSERT INTO workspaces (id, name, slug, owner_id) VALUES (?, 'Acme', 'acme', 'owner')`, workspaceID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, settings, active) VALUES (?, ?, 'key', 'secret', ?, true)`, uuid.NewString(), workspaceID, `{"portal_enabled":true,"portal_intake_enabled":true,"portal_anonymous_intake_enabled":true}`).Error; err != nil {
		t.Fatal(err)
	}
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), nil, repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, repository.NewCRMContactRepository(db), nil, nil, nil, nil)
	// Anonymous intake needs agent replies to reach submitters by email.
	inbox.SetEmailFallbackService(&EmailFallbackService{
		redis:       redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}),
		emailClient: email.NewClient("postmark-token", "support@example.com"),
	})
	sender := &recordingPortalSender{}
	svc := NewCustomerPortalService(repository.NewCustomerPortalRepository(db), inbox, sender, "https://app.example.com")
	ws, err := svc.ResolveWorkspace(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, sender, ws
}

// createPortalTables adds the portal tables to a service test database.
func createPortalTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE support_portal_intake_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, token_hash TEXT UNIQUE, expires_at DATETIME, used_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_identities (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, display_name TEXT, auth_subject TEXT, crm_contact_id TEXT, created_at DATETIME, updated_at DATETIME, UNIQUE(workspace_id, email))`,
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, reference TEXT, customer_last_read_at DATETIME, created_at DATETIME, UNIQUE(workspace_id, conversation_id), UNIQUE(workspace_id, reference))`,
		`CREATE TABLE support_portal_audit_events (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, actor_type TEXT, actor_user_id TEXT, event_type TEXT, metadata TEXT, occurred_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_magic_links (id TEXT PRIMARY KEY, workspace_id TEXT, email TEXT, token_hash TEXT UNIQUE, conversation_id TEXT, expires_at DATETIME, used_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_portal_sessions (id TEXT PRIMARY KEY, workspace_id TEXT, identity_id TEXT, crm_contact_id TEXT, token_hash TEXT UNIQUE, expires_at DATETIME, revoked_at DATETIME, reconciled_at DATETIME, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func portalLinkToken(t *testing.T, email portalSentEmail) string {
	t.Helper()
	_, after, found := strings.Cut(email.text, "token=")
	if !found {
		t.Fatalf("no token in email: %q", email.text)
	}
	token, err := url.QueryUnescape(strings.Fields(after)[0])
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// issuePortalLink stores a sign-in link directly, bypassing throttling.
func issuePortalLink(t *testing.T, db *gorm.DB, workspaceID, email string, createdAt time.Time) string {
	t.Helper()
	secret, err := portalSecret()
	if err != nil {
		t.Fatal(err)
	}
	link := &model.PortalMagicLink{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: email, TokenHash: portalHash(secret), ExpiresAt: time.Now().Add(time.Hour), CreatedAt: createdAt}
	if err := db.Create(link).Error; err != nil {
		t.Fatal(err)
	}
	return secret
}

// setPortalContact stores a CRM contact with an optional portal decision.
func setPortalContact(t *testing.T, db *gorm.DB, workspaceID, email string, access *string) string {
	t.Helper()
	id := uuid.NewString()
	if err := db.Exec(`INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, portal_access) VALUES (?, ?, ?, 'Customer', ?, ?)`, id, workspaceID, "C-"+id[:8], email, access).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func portalAccess(value string) *string { return &value }

// setPortalSettings replaces the installation settings and reloads the portal.
func setPortalSettings(t *testing.T, svc *CustomerPortalService, db *gorm.DB, ws *PortalWorkspace, settings string) *PortalWorkspace {
	t.Helper()
	if err := db.Exec(`UPDATE support_widget_installations SET settings = ? WHERE workspace_id = ?`, settings, ws.ID).Error; err != nil {
		t.Fatal(err)
	}
	reloaded, err := svc.ResolveWorkspace(context.Background(), ws.Slug)
	if err != nil {
		t.Fatal(err)
	}
	return reloaded
}

func signInPortal(t *testing.T, svc *CustomerPortalService, ws *PortalWorkspace, linkSecret string) *PortalAccess {
	t.Helper()
	ctx := context.Background()
	sessionSecret, _, err := svc.Exchange(ctx, ws, linkSecret)
	if err != nil {
		t.Fatal(err)
	}
	access, err := svc.Authenticate(ctx, ws, sessionSecret)
	if err != nil {
		t.Fatal(err)
	}
	return access
}

func TestCustomerPortalAnonymousIntakeIsSingleUse(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	token, err := svc.StartAnonymousIntake(ctx, ws, "Customer@Example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateAnonymousRequest(ctx, ws, token, "Help", "Please help", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateAnonymousRequest(ctx, ws, token, "Again", "Duplicate", nil); err == nil {
		t.Fatal("one-use intake token created a second request")
	}
	var conversation model.SupportConversation
	if err := db.Where("workspace_id = ? AND channel = 'portal'", ws.ID).First(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.CustomerEmail == nil || *conversation.CustomerEmail != "customer@example.com" {
		t.Fatalf("unexpected request: %+v", conversation)
	}
	var count int64
	if err := db.Model(&model.SupportPortalAuditEvent{}).Where("conversation_id = ? AND event_type = ? AND portal_identity_id IS NULL", conversation.ID, model.SupportPortalAuditRequestCreated).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("request audit count=%d err=%v", count, err)
	}
}

// An unverified submitter must not be able to place content in the portal of
// whoever owns the address they typed.
func TestCustomerPortalAnonymousIntakeHiddenUntilAddressConfirmsIt(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "victim@example.com", portalAccess(model.PortalAccessAllowed))
	token, err := svc.StartAnonymousIntake(ctx, ws, "victim@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateAnonymousRequest(ctx, ws, token, "Spoofed", "Written by someone else", nil); err != nil {
		t.Fatal(err)
	}
	var conversation model.SupportConversation
	if err := db.Where("workspace_id = ? AND channel = 'portal'", ws.ID).First(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.PortalVisible {
		t.Fatal("anonymous request visible before confirmation")
	}
	var references int64
	if err := db.Model(&model.SupportPortalRequestReference{}).Count(&references).Error; err != nil || references != 0 {
		t.Fatalf("anonymous request referenced before confirmation: count=%d err=%v", references, err)
	}
	if len(sender.sent) != 1 || sender.sent[0].to != "victim@example.com" || sender.sent[0].subject != "Confirm your support request" {
		t.Fatalf("confirmation email: %+v", sender.sent)
	}

	// Signing in with an ordinary link does not publish the unconfirmed request.
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "victim@example.com", time.Now().Add(-time.Hour)))
	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 0 {
		t.Fatalf("unconfirmed request listed: %v %v", requests, err)
	}

	// The address owner may confirm the request from its own link.
	access = signInPortal(t, svc, ws, portalLinkToken(t, sender.sent[0]))
	requests, err = svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 || requests[0].Subject != "Spoofed" {
		t.Fatalf("confirmed request not listed: %v %v", requests, err)
	}
	var events int64
	if err := db.Model(&model.SupportPortalAuditEvent{}).Where("conversation_id = ? AND event_type = ?", conversation.ID, model.SupportPortalAuditVisibilityChanged).Count(&events).Error; err != nil || events != 1 {
		t.Fatalf("visibility audit count=%d err=%v", events, err)
	}
}

func TestCustomerPortalSignInLinkCooldown(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	svc.RequestLink(ctx, ws, "customer@example.com")
	svc.RequestLink(ctx, ws, "customer@example.com")
	if len(sender.sent) != 1 {
		t.Fatalf("sent %d links within the cooldown", len(sender.sent))
	}
	// A request confirmation is not dropped because a sign-in link was just sent.
	conversationID := uuid.NewString()
	svc.sendLink(ctx, ws, "customer@example.com", &conversationID)
	if len(sender.sent) != 2 {
		t.Fatalf("confirmation link suppressed by cooldown: %d sent", len(sender.sent))
	}
}

func TestCustomerPortalSignInLinkHourlyCaps(t *testing.T) {
	tests := []struct {
		name    string
		prior   func(workspaceID string) []string
		address string
	}{
		{
			name: "per address",
			prior: func(string) []string {
				return []string{"capped@example.com", "capped@example.com", "capped@example.com", "capped@example.com", "capped@example.com"}
			},
			address: "capped@example.com",
		},
		{
			name: "per workspace",
			prior: func(string) []string {
				emails := make([]string, portalLinksPerWorkspaceHour)
				for i := range emails {
					emails[i] = uuid.NewString() + "@example.com"
				}
				return emails
			},
			address: "fresh@example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db, sender, ws := setupCustomerPortalService(t)
			setPortalContact(t, db, ws.ID, tt.address, portalAccess(model.PortalAccessAllowed))
			for _, email := range tt.prior(ws.ID) {
				issuePortalLink(t, db, ws.ID, email, time.Now().Add(-10*time.Minute))
			}
			svc.RequestLink(context.Background(), ws, tt.address)
			if len(sender.sent) != 0 {
				t.Fatalf("link sent past the hourly cap: %+v", sender.sent)
			}
		})
	}
}

func TestCustomerPortalNewRequestUploadRequiresIntake(t *testing.T) {
	svc := &CustomerPortalService{inbox: &SupportInboxService{attachmentService: &SupportAttachmentService{}}}
	access := &PortalAccess{Workspace: PortalWorkspace{Settings: model.SupportInboxSettings{PortalEnabled: true, FileUploadsEnabled: true}}}
	if _, err := svc.UploadAttachment(context.Background(), access, "", model.CreateSupportAttachmentRequest{}); !errors.Is(err, ErrPortalIntakeDisabled) {
		t.Fatalf("new-request upload with intake disabled: %v", err)
	}
	if err := svc.ConfirmAttachment(context.Background(), access, "", uuid.NewString()); !errors.Is(err, ErrPortalIntakeDisabled) {
		t.Fatalf("new-request confirm with intake disabled: %v", err)
	}
}

func TestPortalReferenceIsOpaque(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Help", "Please help", nil)
	if err != nil {
		t.Fatal(err)
	}
	var conversationID string
	if err := db.Model(&model.SupportPortalRequestReference{}).Where("reference = ?", request.Reference).Pluck("conversation_id", &conversationID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(request.Reference, "req_") || strings.Contains(request.Reference, conversationID) {
		t.Fatalf("reference is not opaque: %q (conversation %q)", request.Reference, conversationID)
	}
}

func TestCustomerPortalConfigurationCarriesAttachmentPolicy(t *testing.T) {
	svc, _, _, ws := setupCustomerPortalService(t)
	policy := svc.Configuration(context.Background(), ws).Attachments
	if policy.MaxFiles != PortalMaxAttachmentsPerMessage || policy.MaxBytes != maxSupportFileSize || len(policy.ContentTypes) != len(supportAttachmentContentTypes) {
		t.Fatalf("unexpected attachment policy: %+v", policy)
	}
}

func TestCustomerPortalCapsAttachmentsPerMessage(t *testing.T) {
	svc := &CustomerPortalService{inbox: &SupportInboxService{attachmentService: &SupportAttachmentService{}}}
	ws := &PortalWorkspace{Settings: model.SupportInboxSettings{FileUploadsEnabled: true}}
	ids := make([]string, PortalMaxAttachmentsPerMessage+1)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	if err := svc.validateAttachments(context.Background(), ws, ids, "session", ""); !errors.Is(err, ErrPortalTooManyAttachments) {
		t.Fatalf("%d attachments accepted: %v", len(ids), err)
	}
}

// As in chat, a portal reply may be files only; an empty reply is refused.
func TestCustomerPortalReplyAcceptsFilesWithoutText(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Help", "Please help", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reply(ctx, access, request.Reference, "  ", nil); !errors.Is(err, ErrPortalReplyInvalid) {
		t.Fatalf("empty reply: %v", err)
	}
	svc.inbox.attachmentService = &SupportAttachmentService{attachmentRepo: repository.NewSupportAttachmentRepository(db)}
	access.Workspace.Settings.FileUploadsEnabled = true
	if _, err := svc.Reply(ctx, access, request.Reference, "", []string{uuid.NewString()}); errors.Is(err, ErrPortalReplyInvalid) {
		t.Fatalf("files-only reply refused as empty: %v", err)
	}
}

func TestCustomerPortalListShowsLatestActivityAndUnread(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Export broken", "The **export** fails\nevery time.", nil)
	if err != nil {
		t.Fatal(err)
	}
	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 {
		t.Fatalf("list: %v %v", requests, err)
	}
	first := requests[0]
	if first.Number == 0 || first.LastMessageFrom != "customer" || first.Unread || first.LastMessagePreview != "The export fails every time." {
		t.Fatalf("customer activity: %+v", first)
	}

	// An agent replies; the customer has not opened the request since.
	var conversationID string
	if err := db.Table("support_portal_request_references").Where("reference = ?", request.Reference).Pluck("conversation_id", &conversationID).Error; err != nil {
		t.Fatal(err)
	}
	replyAt := time.Now().UTC()
	if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, sender_display_name, message_type, content, created_at) VALUES ('agent-reply', ?, ?, 'user', 'Priya', 'reply', 'We are **on it**.', ?)`, ws.ID, conversationID, replyAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE support_conversations SET last_public_message_id = 'agent-reply', last_public_sender_type = 'user', last_public_message_at = ?, customer_awaiting_response = false WHERE id = ?`, replyAt, conversationID).Error; err != nil {
		t.Fatal(err)
	}
	requests, err = svc.Requests(ctx, access, "")
	if err != nil || !requests[0].Unread || requests[0].LastMessageFrom != "support" || requests[0].LastMessagePreview != "We are on it." || requests[0].Status != model.SupportConversationStatusWaitingOnCustomer {
		t.Fatalf("support reply not flagged: %+v %v", requests, err)
	}
	waiting, err := svc.Requests(ctx, access, model.SupportConversationStatusWaitingOnCustomer)
	if err != nil || len(waiting) != 1 || waiting[0].Reference != request.Reference {
		t.Fatalf("support reply missing from Waiting on you: %+v %v", waiting, err)
	}
	active, err := svc.Requests(ctx, access, "active")
	if err != nil || len(active) != 0 {
		t.Fatalf("support reply still appears Active: %+v %v", active, err)
	}
	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil || detail.Number != first.Number || detail.Status != model.SupportConversationStatusWaitingOnCustomer {
		t.Fatalf("detail: %+v %v", detail, err)
	}
	requests, err = svc.Requests(ctx, access, "")
	if err != nil || requests[0].Unread {
		t.Fatalf("opening the request did not clear unread: %+v %v", requests, err)
	}
	if _, err := svc.Reply(ctx, access, request.Reference, "It still fails.", nil); err != nil {
		t.Fatal(err)
	}
	// Production projects the latest public sender through a PostgreSQL trigger;
	// this SQLite fixture records the equivalent state explicitly.
	if err := db.Exec(`UPDATE support_conversations SET last_public_sender_type = 'customer', customer_awaiting_response = true WHERE id = ?`, conversationID).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil || updated.Status != "active" {
		t.Fatalf("customer reply should return to Active: %+v %v", updated, err)
	}
	active, err = svc.Requests(ctx, access, "active")
	if err != nil || len(active) != 1 || active[0].Reference != request.Reference {
		t.Fatalf("customer reply missing from Active: %+v %v", active, err)
	}
}

func TestCustomerPortalNamesEveryAIReplyConsistently(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Help", "Please help", nil)
	if err != nil {
		t.Fatal(err)
	}
	var conversationID string
	if err := db.Table("support_portal_request_references").Where("reference = ?", request.Reference).Pluck("conversation_id", &conversationID).Error; err != nil {
		t.Fatal(err)
	}
	for i, row := range []struct{ sender, name, metadata string }{
		{"ai", "Echo", "{}"},
		{"ai", "Helpin AI", "{}"},
		{"agent", "Automation", `{"ai_agent_id":"agent-1"}`},
		{"user", "Priya", "{}"},
	} {
		if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, sender_display_name, sender_avatar_url, message_type, content, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, 'reply', 'hello', ?, ?)`,
			fmt.Sprintf("m-%d", i), ws.ID, conversationID, row.sender, row.name, "https://avatars.example/"+row.name, row.metadata, time.Now().Add(time.Duration(i+1)*time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
	}
	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, message := range detail.Messages[1:] {
		names = append(names, fmt.Sprintf("%s:%s:%v", message.SenderType, derefString(message.SenderName), message.SenderAvatar != nil))
	}
	want := []string{"ai:AI assistant:false", "ai:AI assistant:false", "ai:AI assistant:false", "user:Priya:true"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("senders = %v, want %v", names, want)
	}
}

func TestCustomerPortalShowsTeammatesCurrentAvatars(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Help", "Please help", nil)
	if err != nil {
		t.Fatal(err)
	}
	var conversationID string
	if err := db.Table("support_portal_request_references").Where("reference = ?", request.Reference).Pluck("conversation_id", &conversationID).Error; err != nil {
		t.Fatal(err)
	}
	for _, user := range []struct{ id, url, style string }{{"u-photo", "https://avatars.example/new.png", ""}, {"u-generated", "", "micah"}} {
		if err := db.Exec(`INSERT INTO users (id, email, password_hash, full_name, avatar_url, avatar_style, avatar_seed, avatar_background_color) VALUES (?, ?, 'x', ?, NULLIF(?, ''), NULLIF(?, ''), 'seed-1', '#f97316')`,
			user.id, user.id+"@example.com", user.id, user.url, user.style).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i, sender := range []string{"u-photo", "u-generated"} {
		if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, sender_user_id, sender_display_name, sender_avatar_url, message_type, content, metadata, created_at) VALUES (?, ?, ?, 'user', ?, ?, 'https://avatars.example/stale.png', 'reply', 'hello', '{}', ?)`,
			fmt.Sprintf("m-%d", i), ws.ID, conversationID, sender, sender, time.Now().Add(time.Duration(i+1)*time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
	}
	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil {
		t.Fatal(err)
	}
	photo, generated := detail.Messages[1], detail.Messages[2]
	if derefString(photo.SenderAvatar) != "https://avatars.example/new.png" || photo.SenderAvatarStyle != nil {
		t.Fatalf("uploaded avatar = %v %+v, want the current upload", derefString(photo.SenderAvatar), photo.SenderAvatarStyle)
	}
	if generated.SenderAvatar != nil || generated.SenderAvatarStyle == nil || derefString(generated.SenderAvatarStyle.Style) != "micah" ||
		derefString(generated.SenderAvatarStyle.Seed) != "seed-1" || derefString(generated.SenderAvatarStyle.BackgroundColor) != "#f97316" {
		t.Fatalf("generated avatar = %v %+v, want the generated style", derefString(generated.SenderAvatar), generated.SenderAvatarStyle)
	}
}

func TestCustomerPortalBrandingUsesWidgetIdentity(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ws = setPortalSettings(t, svc, db, ws, `{"portal_enabled":true,"widget_name":"Usermaven Support","logo_url":"https://cdn.example/logo.png","brand_color":"#6366F1"}`)
	branding := svc.Configuration(context.Background(), ws).Branding
	if branding["name"] != "Usermaven Support" || branding["logo_url"] != "https://cdn.example/logo.png" || branding["brand_color"] != "#6366F1" || branding["assistant_name"] != "AI assistant" {
		t.Fatalf("branding: %v", branding)
	}
	ws = setPortalSettings(t, svc, db, ws, `{"portal_enabled":true}`)
	if name := svc.Configuration(context.Background(), ws).Branding["name"]; name != "Acme" {
		t.Fatalf("fallback name = %q, want workspace name", name)
	}
	if err := db.Exec(`UPDATE workspaces SET logo_url = ? WHERE id = ?`, "https://cdn.example/workspace.png", ws.ID).Error; err != nil {
		t.Fatal(err)
	}
	ws = setPortalSettings(t, svc, db, ws, `{"portal_enabled":true}`)
	if logo := svc.Configuration(context.Background(), ws).Branding["logo_url"]; logo != "https://cdn.example/workspace.png" {
		t.Fatalf("fallback logo = %q, want workspace logo", logo)
	}
	if err := db.Exec(`UPDATE workspaces SET logo_url = NULL, website_url = ? WHERE id = ?`, "https://www.Usermaven.com/pricing", ws.ID).Error; err != nil {
		t.Fatal(err)
	}
	ws = setPortalSettings(t, svc, db, ws, `{"portal_enabled":true}`)
	if logo := svc.Configuration(context.Background(), ws).Branding["logo_url"]; logo != "https://www.google.com/s2/favicons?domain=usermaven.com&sz=128" {
		t.Fatalf("favicon logo = %q, want the website favicon", logo)
	}
}
