package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const (
	portalSettingsApproved = `{"portal_enabled":true,"portal_intake_enabled":true,"portal_anonymous_intake_enabled":true,"portal_access_mode":"approved_contacts"}`
	portalSettingsAnyEmail = `{"portal_enabled":true,"portal_intake_enabled":true,"portal_anonymous_intake_enabled":true,"portal_access_mode":"any_verified_email"}`
)

func TestEvaluatePortalEligibility(t *testing.T) {
	allowed, blocked := portalAccess(model.PortalAccessAllowed), portalAccess(model.PortalAccessBlocked)
	contact := func(id string, access *string) repository.PortalContactAccess {
		return repository.PortalContactAccess{ID: id, PortalAccess: access}
	}
	tests := []struct {
		name        string
		mode        string
		contacts    []repository.PortalContactAccess
		wantOK      bool
		wantContact string
	}{
		{"approved: no contact", model.SupportPortalAccessModeApprovedContacts, nil, false, ""},
		{"approved: unset contact", model.SupportPortalAccessModeApprovedContacts, []repository.PortalContactAccess{contact("a", nil)}, false, ""},
		{"approved: one allowed and one unset", model.SupportPortalAccessModeApprovedContacts, []repository.PortalContactAccess{contact("a", nil), contact("b", allowed)}, true, "b"},
		{"approved: two allowed", model.SupportPortalAccessModeApprovedContacts, []repository.PortalContactAccess{contact("a", allowed), contact("b", allowed)}, false, ""},
		{"approved: allowed and blocked", model.SupportPortalAccessModeApprovedContacts, []repository.PortalContactAccess{contact("a", allowed), contact("b", blocked)}, false, ""},
		{"any email: no contact", model.SupportPortalAccessModeAnyVerifiedEmail, nil, true, ""},
		{"any email: sole contact is bound", model.SupportPortalAccessModeAnyVerifiedEmail, []repository.PortalContactAccess{contact("a", nil)}, true, "a"},
		{"any email: duplicates are not bound", model.SupportPortalAccessModeAnyVerifiedEmail, []repository.PortalContactAccess{contact("a", nil), contact("b", nil)}, true, ""},
		{"any email: blocked", model.SupportPortalAccessModeAnyVerifiedEmail, []repository.PortalContactAccess{contact("a", nil), contact("b", blocked)}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluatePortalEligibility(tt.mode, tt.contacts)
			if got.Eligible != tt.wantOK || derefString(got.ContactID) != tt.wantContact {
				t.Fatalf("eligibility = %v %q, want %v %q", got.Eligible, derefString(got.ContactID), tt.wantOK, tt.wantContact)
			}
		})
	}
}

// Anonymous intake creates a CRM contact automatically; that must not make
// the address eligible for the portal.
func TestCustomerPortalAutoCreatedContactIsNotEligible(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	token, err := svc.StartAnonymousIntake(ctx, ws, "new@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateAnonymousRequest(ctx, ws, token, "Help", "Please help", nil); err != nil {
		t.Fatal(err)
	}
	var contacts int64
	if err := db.Table("crm_contacts").Where("workspace_id = ? AND email = ?", ws.ID, "new@example.com").Count(&contacts).Error; err != nil || contacts != 1 {
		t.Fatalf("intake did not create a CRM contact: count=%d err=%v", contacts, err)
	}
	if len(sender.sent) != 1 || sender.sent[0].subject != "We received your support request" || strings.Contains(sender.sent[0].text, "token=") {
		t.Fatalf("ineligible submitter should get a receipt without a link: %+v", sender.sent)
	}
	svc.RequestLink(ctx, ws, "new@example.com")
	if len(sender.sent) != 1 {
		t.Fatalf("sign-in link sent to an automatically created contact: %+v", sender.sent)
	}
}

func TestCustomerPortalIneligibleSignInSendsNothing(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	setPortalContact(t, db, ws.ID, "unset@example.com", nil)
	for _, address := range []string{"stranger@example.com", "unset@example.com"} {
		svc.RequestLink(context.Background(), ws, address)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("ineligible addresses received email: %+v", sender.sent)
	}
}

func TestCustomerPortalEmailNormalizationMatchesContacts(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	setPortalContact(t, db, ws.ID, "  Customer@Example.COM ", portalAccess(model.PortalAccessAllowed))
	svc.RequestLink(context.Background(), ws, "customer@example.com")
	if len(sender.sent) != 1 {
		t.Fatalf("normalized address did not match its contact: %+v", sender.sent)
	}
}

// Deleting allowed contact A and approving B with the same email must not
// keep A's session alive; a fresh sign-in binds B and keeps the requests.
func TestCustomerPortalRebindRequiresFreshSignIn(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	contactA := setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	sessionSecret, identity, err := svc.Exchange(ctx, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if derefString(identity.CRMContactID) != contactA {
		t.Fatalf("identity bound to %q, want %q", derefString(identity.CRMContactID), contactA)
	}
	access, err := svc.Authenticate(ctx, ws, sessionSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRequest(ctx, access, "Help", "Please help", nil); err != nil {
		t.Fatal(err)
	}

	if err := db.Exec(`DELETE FROM crm_contacts WHERE id = ?`, contactA).Error; err != nil {
		t.Fatal(err)
	}
	contactB := setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	if _, err := svc.Authenticate(ctx, ws, sessionSecret); !errors.Is(err, ErrPortalAuthInvalid) {
		t.Fatalf("session bound to deleted contact still authorized: %v", err)
	}
	assertPortalSessionRevoked(t, db, sessionSecret)

	access = signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	if derefString(access.Identity.CRMContactID) != contactB {
		t.Fatalf("fresh sign-in bound to %q, want %q", derefString(access.Identity.CRMContactID), contactB)
	}
	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 {
		t.Fatalf("requests after rebinding: %v %v", requests, err)
	}
}

func assertPortalSessionRevoked(t *testing.T, db *gorm.DB, sessionSecret string) {
	t.Helper()
	var revoked int64
	if err := db.Model(&model.PortalSession{}).Where("token_hash = ? AND revoked_at IS NOT NULL", portalHash(sessionSecret)).Count(&revoked).Error; err != nil || revoked != 1 {
		t.Fatalf("session not revoked: count=%d err=%v", revoked, err)
	}
}

func TestCustomerPortalSessionRevokedWhenAccessChanges(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		access   *string
		change   func(t *testing.T, svc *CustomerPortalService, db *gorm.DB, ws *PortalWorkspace, contactID string) *PortalWorkspace
	}{
		{
			name: "contact blocked", settings: portalSettingsApproved, access: portalAccess(model.PortalAccessAllowed),
			change: func(t *testing.T, _ *CustomerPortalService, db *gorm.DB, ws *PortalWorkspace, contactID string) *PortalWorkspace {
				if err := repository.NewCustomerPortalRepository(db).SetContactPortalAccess(context.Background(), ws.ID, contactID, portalAccess(model.PortalAccessBlocked)); err != nil {
					t.Fatal(err)
				}
				return ws
			},
		},
		{
			name: "contact email changed", settings: portalSettingsApproved, access: portalAccess(model.PortalAccessAllowed),
			change: func(t *testing.T, _ *CustomerPortalService, db *gorm.DB, ws *PortalWorkspace, contactID string) *PortalWorkspace {
				if err := db.Exec(`UPDATE crm_contacts SET email = 'moved@example.com' WHERE id = ?`, contactID).Error; err != nil {
					t.Fatal(err)
				}
				return ws
			},
		},
		{
			name: "contact deleted", settings: portalSettingsApproved, access: portalAccess(model.PortalAccessAllowed),
			change: func(t *testing.T, _ *CustomerPortalService, db *gorm.DB, ws *PortalWorkspace, contactID string) *PortalWorkspace {
				if err := db.Exec(`DELETE FROM crm_contacts WHERE id = ?`, contactID).Error; err != nil {
					t.Fatal(err)
				}
				return ws
			},
		},
		{
			name: "switched to approved contacts", settings: portalSettingsAnyEmail, access: nil,
			change: func(t *testing.T, svc *CustomerPortalService, db *gorm.DB, ws *PortalWorkspace, _ string) *PortalWorkspace {
				return setPortalSettings(t, svc, db, ws, portalSettingsApproved)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db, _, ws := setupCustomerPortalService(t)
			ws = setPortalSettings(t, svc, db, ws, tt.settings)
			contactID := setPortalContact(t, db, ws.ID, "customer@example.com", tt.access)
			sessionSecret, _, err := svc.Exchange(context.Background(), ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Authenticate(context.Background(), ws, sessionSecret); err != nil {
				t.Fatalf("session not authorized before the change: %v", err)
			}
			ws = tt.change(t, svc, db, ws, contactID)
			if _, err := svc.Authenticate(context.Background(), ws, sessionSecret); !errors.Is(err, ErrPortalAuthInvalid) {
				t.Fatalf("session still authorized: %v", err)
			}
			assertPortalSessionRevoked(t, db, sessionSecret)
		})
	}
}

func TestCustomerPortalLinkRejectedAfterAccessRevoked(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	contactID := setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	svc.RequestLink(ctx, ws, "customer@example.com")
	if len(sender.sent) != 1 {
		t.Fatalf("eligible customer got no link: %+v", sender.sent)
	}
	if err := repository.NewCustomerPortalRepository(db).SetContactPortalAccess(ctx, ws.ID, contactID, portalAccess(model.PortalAccessBlocked)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Exchange(ctx, ws, portalLinkToken(t, sender.sent[0])); !errors.Is(err, ErrPortalAuthInvalid) {
		t.Fatalf("link exchanged after access was revoked: %v", err)
	}
	var sessions int64
	if err := db.Model(&model.PortalSession{}).Count(&sessions).Error; err != nil || sessions != 0 {
		t.Fatalf("session created for revoked customer: %d %v", sessions, err)
	}
}

func TestCustomerPortalIntakeReceiptIsThrottled(t *testing.T) {
	svc, _, sender, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	for i := 0; i < portalLinksPerAddressPerHour+2; i++ {
		token, err := svc.StartAnonymousIntake(ctx, ws, "flood@example.com")
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.CreateAnonymousRequest(ctx, ws, token, "Help", "Please help", nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(sender.sent) != portalLinksPerAddressPerHour {
		t.Fatalf("sent %d receipts, want %d", len(sender.sent), portalLinksPerAddressPerHour)
	}
}

// An admin can publish one earlier anonymous request after approving its
// submitter; other hidden requests stay hidden.
func TestCustomerPortalResendConfirmationPublishesOnlyThatRequest(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	var conversationIDs []string
	for _, subject := range []string{"First", "Second"} {
		token, err := svc.StartAnonymousIntake(ctx, ws, "later@example.com")
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.CreateAnonymousRequest(ctx, ws, token, subject, "Please help", nil); err != nil {
			t.Fatal(err)
		}
		var conversation model.SupportConversation
		if err := db.Where("workspace_id = ? AND subject = ?", ws.ID, subject).First(&conversation).Error; err != nil {
			t.Fatal(err)
		}
		conversationIDs = append(conversationIDs, conversation.ID)
	}
	if err := svc.ResendIntakeConfirmation(ctx, ws.ID, conversationIDs[0], ""); !errors.Is(err, ErrPortalCustomerNotEligible) {
		t.Fatalf("resend to ineligible submitter: %v", err)
	}
	var contactID string
	if err := db.Table("crm_contacts").Where("workspace_id = ? AND email = ?", ws.ID, "later@example.com").Pluck("id", &contactID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetContactAccess(ctx, ws.ID, contactID, "", portalAccess(model.PortalAccessAllowed)); err != nil {
		t.Fatal(err)
	}
	sentBefore := len(sender.sent)
	if err := svc.ResendIntakeConfirmation(ctx, ws.ID, conversationIDs[0], ""); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != sentBefore+1 || sender.sent[sentBefore].subject != "Confirm your support request" {
		t.Fatalf("confirmation not sent: %+v", sender.sent)
	}
	access := signInPortal(t, svc, ws, portalLinkToken(t, sender.sent[sentBefore]))
	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 || requests[0].Subject != "First" {
		t.Fatalf("resend published the wrong requests: %v %v", requests, err)
	}
}

func TestCustomerPortalAnonymousIntakeNeedsReplyDelivery(t *testing.T) {
	svc, _, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	svc.inbox.SetEmailFallbackService(nil)
	if svc.Configuration(context.Background(), ws).AnonymousIntakeEnabled {
		t.Fatal("portal offers intake without reply delivery")
	}
	if _, err := svc.StartAnonymousIntake(ctx, ws, "customer@example.com"); !errors.Is(err, ErrPortalAnonymousIntakeDisabled) {
		t.Fatalf("intake started without reply delivery: %v", err)
	}
	enabled := true
	if _, _, err := svc.inbox.UpdateInstallationSettings(ctx, ws.ID, model.UpdateInstallationSettingsRequest{PortalAnonymousIntakeEnabled: &enabled}); !errors.Is(err, ErrPortalReplyDeliveryUnavailable) {
		t.Fatalf("enabling intake without reply delivery: %v", err)
	}
}

func TestCustomerPortalSettingsRejectUnknownAccessMode(t *testing.T) {
	svc, _, _, ws := setupCustomerPortalService(t)
	mode := "everyone"
	if _, _, err := svc.inbox.UpdateInstallationSettings(context.Background(), ws.ID, model.UpdateInstallationSettingsRequest{PortalAccessMode: &mode}); err == nil {
		t.Fatal("unknown access mode accepted")
	}
}

func TestCustomerPortalAccessSummaryCountsRealConflicts(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	allowed, blocked := portalAccess(model.PortalAccessAllowed), portalAccess(model.PortalAccessBlocked)
	setPortalContact(t, db, ws.ID, "fine@example.com", allowed)
	setPortalContact(t, db, ws.ID, "fine@example.com", nil)
	setPortalContact(t, db, ws.ID, "double@example.com", allowed)
	setPortalContact(t, db, ws.ID, "Double@Example.com ", allowed)
	setPortalContact(t, db, ws.ID, "mixed@example.com", allowed)
	setPortalContact(t, db, ws.ID, "mixed@example.com", blocked)
	setPortalContact(t, db, ws.ID, "blocked@example.com", blocked)
	setPortalContact(t, db, ws.ID, "blocked@example.com", nil)
	summary, err := svc.AccessSummary(context.Background(), ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.AllowedContacts != 4 || summary.Conflicts != 2 || strings.Join(summary.ConflictEmails, ",") != "double@example.com,mixed@example.com" || !summary.AnonymousIntakeDeliveryAvailable {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestCustomerPortalSetContactAccessValidates(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	contactID := setPortalContact(t, db, ws.ID, "customer@example.com", nil)
	setPortalContact(t, db, ws.ID, "customer@example.com", nil)
	if _, err := svc.SetContactAccess(ctx, ws.ID, contactID, "", portalAccess("owner")); !errors.Is(err, ErrPortalAccessInvalid) {
		t.Fatalf("invalid access accepted: %v", err)
	}
	if _, err := svc.SetContactAccess(ctx, "other-workspace", contactID, "", portalAccess(model.PortalAccessAllowed)); !errors.Is(err, ErrPortalContactNotFound) {
		t.Fatalf("cross-workspace contact updated: %v", err)
	}
	access, err := svc.SetContactAccess(ctx, ws.ID, contactID, "", portalAccess(model.PortalAccessAllowed))
	if err != nil || derefString(access.PortalAccess) != model.PortalAccessAllowed || access.SharedEmailContacts != 1 {
		t.Fatalf("set access: %+v %v", access, err)
	}
	access, err = svc.SetContactAccess(ctx, ws.ID, contactID, "", portalAccess(""))
	if err != nil || access.PortalAccess != nil {
		t.Fatalf("clear access: %+v %v", access, err)
	}
}

// Identities are shared per email. A later sign-in bound to contact B must
// not let a session issued for contact A continue as B, even if A's session
// is checked only afterwards.
func TestCustomerPortalOldSessionCannotContinueAsReboundContact(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	contactA := setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	sessionA, _, err := svc.Exchange(ctx, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DELETE FROM crm_contacts WHERE id = ?`, contactA).Error; err != nil {
		t.Fatal(err)
	}
	contactB := setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	sessionB, identity, err := svc.Exchange(ctx, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if derefString(identity.CRMContactID) != contactB {
		t.Fatalf("identity bound to %q, want %q", derefString(identity.CRMContactID), contactB)
	}
	// Rebinding revokes the identity's older sessions.
	assertPortalSessionRevoked(t, db, sessionA)
	// The session's own contact rejects it even if that revocation were missed.
	if err := db.Model(&model.PortalSession{}).Where("token_hash = ?", portalHash(sessionA)).Update("revoked_at", nil).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, ws, sessionA); !errors.Is(err, ErrPortalAuthInvalid) {
		t.Fatalf("session issued for contact A continued as contact B: %v", err)
	}
	if _, err := svc.Authenticate(ctx, ws, sessionB); err != nil {
		t.Fatalf("new session for contact B rejected: %v", err)
	}
}

// Lookup failures are not proof of lost access: they must not look like an
// invalid session or link, and must not revoke or consume anything.
func TestCustomerPortalLookupFailureIsNotInvalidCredentials(t *testing.T) {
	svc, db, sender, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	sessionSecret, _, err := svc.Exchange(ctx, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	svc.RequestLink(ctx, ws, "customer@example.com")
	if len(sender.sent) != 1 {
		t.Fatalf("link not sent: %+v", sender.sent)
	}
	if err := db.Exec(`ALTER TABLE crm_contacts RENAME TO crm_contacts_unavailable`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, ws, sessionSecret); err == nil || errors.Is(err, ErrPortalAuthInvalid) {
		t.Fatalf("lookup failure reported as %v", err)
	}
	var revoked int64
	if err := db.Model(&model.PortalSession{}).Where("revoked_at IS NOT NULL").Count(&revoked).Error; err != nil || revoked != 0 {
		t.Fatalf("lookup failure revoked the session: %d %v", revoked, err)
	}
	token := portalLinkToken(t, sender.sent[0])
	if _, _, err := svc.Exchange(ctx, ws, token); err == nil || errors.Is(err, ErrPortalAuthInvalid) {
		t.Fatalf("exchange lookup failure reported as %v", err)
	}
	if err := db.Exec(`ALTER TABLE crm_contacts_unavailable RENAME TO crm_contacts`).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Exchange(ctx, ws, token); err != nil {
		t.Fatalf("link was consumed by a failed exchange: %v", err)
	}
}

type unconfiguredPortalSender struct{ recordingPortalSender }

func (unconfiguredPortalSender) Configured() bool { return false }

// Anonymous intake emails a confirmation or receipt, so it needs a working
// application sender as well as reply delivery.
func TestCustomerPortalAnonymousIntakeNeedsApplicationEmail(t *testing.T) {
	for _, tt := range []struct {
		name   string
		sender portalEmailSender
	}{
		{"no sender", nil},
		{"unconfigured sender", &unconfiguredPortalSender{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			base, _, _, ws := setupCustomerPortalService(t)
			ctx := context.Background()
			svc := NewCustomerPortalService(base.repo, base.inbox, tt.sender, base.baseURL)
			if svc.Configuration(context.Background(), ws).AnonymousIntakeEnabled {
				t.Fatal("portal offers intake without application email")
			}
			if _, err := svc.StartAnonymousIntake(ctx, ws, "customer@example.com"); !errors.Is(err, ErrPortalAnonymousIntakeDisabled) {
				t.Fatalf("intake started without application email: %v", err)
			}
			summary, err := svc.AccessSummary(ctx, ws.ID)
			if err != nil || summary.AnonymousIntakeDeliveryAvailable {
				t.Fatalf("summary reports intake email available: %+v %v", summary, err)
			}
			enabled := true
			if _, _, err := svc.inbox.UpdateInstallationSettings(ctx, ws.ID, model.UpdateInstallationSettingsRequest{PortalAnonymousIntakeEnabled: &enabled}); !errors.Is(err, ErrPortalReplyDeliveryUnavailable) {
				t.Fatalf("enabling intake without application email: %v", err)
			}
		})
	}
}
