package service

import (
	"context"
	"errors"
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
		`CREATE TABLE support_portal_request_references (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, portal_identity_id TEXT, reference TEXT, created_at DATETIME, UNIQUE(workspace_id, conversation_id), UNIQUE(workspace_id, reference))`,
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
	policy := svc.Configuration(ws).Attachments
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
