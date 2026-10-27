package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// companyPortal is a workspace where Alice and Bob are allowed contacts of
// Acme, whose portal default role is member, and Bob has one request.
type companyPortal struct {
	svc        *CustomerPortalService
	db         *gorm.DB
	ws         *PortalWorkspace
	acme       string
	alice, bob string
	aliceIn    *PortalAccess
	bobIn      *PortalAccess
	bobRequest *model.SupportPortalRequest
}

func setupCompanyPortal(t *testing.T) *companyPortal {
	t.Helper()
	svc, db, _, ws := setupCustomerPortalService(t)
	p := &companyPortal{svc: svc, db: db, ws: ws}
	p.acme = createPortalCompany(t, db, ws.ID, "Acme", portalAccess(model.PortalRoleMember))
	p.alice = createNamedPortalContact(t, db, ws.ID, "Alice", "alice@acme.test", portalAccess(model.PortalAccessAllowed))
	p.bob = createNamedPortalContact(t, db, ws.ID, "Bob", "bob@acme.test", portalAccess(model.PortalAccessAllowed))
	linkContactCompany(t, db, ws.ID, p.alice, p.acme)
	// Stored from the company side to cover both association directions.
	linkCompanyContact(t, db, ws.ID, p.acme, p.bob)
	p.bobIn = signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "bob@acme.test", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(context.Background(), p.bobIn, "Invoices are missing", "We cannot find March invoices", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.bobRequest = request
	p.aliceIn = signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "alice@acme.test", time.Now().Add(-time.Hour)))
	return p
}

func createPortalCompany(t *testing.T, db *gorm.DB, workspaceID, name string, defaultRole *string) string {
	t.Helper()
	id := uuid.NewString()
	if err := db.Exec(`INSERT INTO crm_companies (id, workspace_id, display_id, name, portal_default_role) VALUES (?, ?, ?, ?, ?)`, id, workspaceID, "CO-"+id[:8], name, defaultRole).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func createNamedPortalContact(t *testing.T, db *gorm.DB, workspaceID, name, email string, access *string) string {
	t.Helper()
	id := uuid.NewString()
	if err := db.Exec(`INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, last_name, email, portal_access) VALUES (?, ?, ?, ?, 'Example', ?, ?)`, id, workspaceID, "C-"+id[:8], name, email, access).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func linkContactCompany(t *testing.T, db *gorm.DB, workspaceID, contactID, companyID string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES (?, ?, 'contact', ?, 'company', ?)`, uuid.NewString(), workspaceID, contactID, companyID).Error; err != nil {
		t.Fatal(err)
	}
}

func linkCompanyContact(t *testing.T, db *gorm.DB, workspaceID, companyID, contactID string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO crm_associations (id, workspace_id, from_object_type, from_object_id, to_object_type, to_object_id) VALUES (?, ?, 'company', ?, 'contact', ?)`, uuid.NewString(), workspaceID, companyID, contactID).Error; err != nil {
		t.Fatal(err)
	}
}

func (p *companyPortal) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if err := p.db.Exec(sql, args...).Error; err != nil {
		t.Fatal(err)
	}
}

func (p *companyPortal) bobConversationID(t *testing.T) string {
	t.Helper()
	var id string
	if err := p.db.Table("support_portal_request_references").Where("reference = ?", p.bobRequest.Reference).Pluck("conversation_id", &id).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func requestReferences(requests []model.SupportPortalRequest) []string {
	references := make([]string, 0, len(requests))
	for _, request := range requests {
		references = append(references, request.Reference)
	}
	return references
}

func TestCompanyPortalRequestCompanyIsSnapshotted(t *testing.T) {
	p := setupCompanyPortal(t)
	var companyIDs []string
	if err := p.db.Table("support_conversations").Where("id = ? AND crm_company_id IS NOT NULL", p.bobConversationID(t)).Pluck("crm_company_id", &companyIDs).Error; err != nil {
		t.Fatal(err)
	}
	if len(companyIDs) != 1 || companyIDs[0] != p.acme {
		t.Fatalf("portal request company = %v, want Acme", companyIDs)
	}

	// A contact of two companies leaves the company for a teammate to choose.
	other := createPortalCompany(t, p.db, p.ws.ID, "Other", nil)
	linkContactCompany(t, p.db, p.ws.ID, p.alice, other)
	request, err := p.svc.CreateRequest(context.Background(), p.aliceIn, "Two companies", "Which account?", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ambiguous int64
	if err := p.db.Table("support_conversations AS conv").Joins("JOIN support_portal_request_references AS refs ON refs.conversation_id = conv.id").
		Where("refs.reference = ? AND conv.crm_company_id IS NOT NULL", request.Reference).Count(&ambiguous).Error; err != nil {
		t.Fatal(err)
	}
	if ambiguous != 0 {
		t.Fatal("ambiguous membership chose a company")
	}
}

func TestCompanyPortalSessionCompanies(t *testing.T) {
	p := setupCompanyPortal(t)
	createPortalCompany(t, p.db, p.ws.ID, "Unlinked", portalAccess(model.PortalRoleMember))
	off := createPortalCompany(t, p.db, p.ws.ID, "Portal off", nil)
	linkContactCompany(t, p.db, p.ws.ID, p.alice, off)
	p.exec(t, `UPDATE crm_contacts SET portal_role = ? WHERE id = ?`, model.PortalRoleRequester, p.alice)

	companies, err := p.svc.Companies(context.Background(), p.aliceIn)
	if err != nil {
		t.Fatal(err)
	}
	if len(companies) != 1 || companies[0].ID != p.acme || companies[0].Name != "Acme" || companies[0].Role != model.PortalRoleRequester {
		t.Fatalf("companies = %+v, want Acme as requester", companies)
	}
}

func TestCompanyPortalMemberReadsColleagueRequest(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()

	mine, err := p.svc.Requests(ctx, p.aliceIn, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 0 {
		t.Fatalf("colleague request listed as Alice's own: %v", requestReferences(mine))
	}
	company, err := p.svc.CompanyRequests(ctx, p.aliceIn, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(company) != 1 || company[0].Reference != p.bobRequest.Reference {
		t.Fatalf("company requests = %v, want Bob's", requestReferences(company))
	}
	row := company[0]
	if row.OwnerIdentityID == p.aliceIn.Identity.ID || derefString(row.RequesterName) != "Bob Example" || derefString(row.CompanyName) != "Acme" || row.Unread {
		t.Fatalf("company row = %+v", row)
	}

	detail, err := p.svc.RequestDetail(ctx, p.aliceIn, p.bobRequest.Reference)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Own || derefString(detail.RequesterName) != "Bob Example" {
		t.Fatalf("colleague detail own=%v requester=%q", detail.Own, derefString(detail.RequesterName))
	}
	if len(detail.Messages) != 1 || detail.Messages[0].FromYou {
		t.Fatalf("Bob's message shown as Alice's: %+v", detail.Messages)
	}
	if derefString(detail.Messages[0].SenderName) != "Bob Example" {
		t.Fatalf("Bob's first message is named %q in Alice's view", derefString(detail.Messages[0].SenderName))
	}
	if detail.CanRate {
		t.Fatal("a colleague may rate the request")
	}

	// Repeated views (live updates refetch) are audited once per interval.
	if _, err := p.svc.RequestDetail(ctx, p.aliceIn, p.bobRequest.Reference); err != nil {
		t.Fatal(err)
	}
	var views int64
	if err := p.db.Table("support_portal_audit_events").Where("event_type = ? AND portal_identity_id = ?", model.SupportPortalAuditCompanyRequestViewed, p.aliceIn.Identity.ID).Count(&views).Error; err != nil {
		t.Fatal(err)
	}
	if views != 1 {
		t.Fatalf("company view audit events = %d, want 1", views)
	}

	// The requester's own read state is not touched by a colleague.
	var read int64
	if err := p.db.Table("support_portal_request_references").Where("reference = ? AND customer_last_read_at IS NOT NULL", p.bobRequest.Reference).Count(&read).Error; err != nil {
		t.Fatal(err)
	}
	if read != 0 {
		t.Fatal("colleague view marked the request read for Bob")
	}
}

func TestCompanyPortalVisibilityRules(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(t *testing.T, p *companyPortal)
		visible bool
	}{
		{"member by company default", func(*testing.T, *companyPortal) {}, true},
		{"company portal off", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE crm_companies SET portal_default_role = NULL WHERE id = ?`, p.acme)
		}, false},
		{"company default requester", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE crm_companies SET portal_default_role = ? WHERE id = ?`, model.PortalRoleRequester, p.acme)
		}, false},
		{"member override on a requester company", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE crm_companies SET portal_default_role = ? WHERE id = ?`, model.PortalRoleRequester, p.acme)
			p.exec(t, `UPDATE crm_contacts SET portal_role = ? WHERE id = ?`, model.PortalRoleMember, p.alice)
		}, true},
		{"requester override on a member company", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE crm_contacts SET portal_role = ? WHERE id = ?`, model.PortalRoleRequester, p.alice)
		}, false},
		{"member override with company portal off", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE crm_companies SET portal_default_role = NULL WHERE id = ?`, p.acme)
			p.exec(t, `UPDATE crm_contacts SET portal_role = ? WHERE id = ?`, model.PortalRoleMember, p.alice)
		}, false},
		{"viewer removed from the company", func(t *testing.T, p *companyPortal) {
			p.exec(t, `DELETE FROM crm_associations WHERE from_object_id = ? AND to_object_id = ?`, p.alice, p.acme)
		}, false},
		{"requester removed from the company", func(t *testing.T, p *companyPortal) {
			p.exec(t, `DELETE FROM crm_associations WHERE from_object_id = ? AND to_object_id = ?`, p.acme, p.bob)
		}, false},
		{"requester blocked", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE crm_contacts SET portal_access = ? WHERE id = ?`, model.PortalAccessBlocked, p.bob)
		}, false},
		{"request moved to another company", func(t *testing.T, p *companyPortal) {
			other := createPortalCompany(t, p.db, p.ws.ID, "Other", portalAccess(model.PortalRoleMember))
			p.exec(t, `UPDATE support_conversations SET crm_company_id = ? WHERE id = ?`, other, p.bobConversationID(t))
		}, false},
		{"request erased", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE support_conversations SET anonymized_at = ? WHERE id = ?`, time.Now(), p.bobConversationID(t))
		}, false},
		{"request hidden from the portal", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE support_conversations SET portal_visible = false, portal_visibility_changed_at = ? WHERE id = ?`, time.Now(), p.bobConversationID(t))
		}, false},
		{"request marked spam", func(t *testing.T, p *companyPortal) {
			p.exec(t, `UPDATE support_conversations SET status = 'spam' WHERE id = ?`, p.bobConversationID(t))
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := setupCompanyPortal(t)
			tt.arrange(t, p)
			ctx := context.Background()
			listed, err := p.svc.CompanyRequests(ctx, p.aliceIn, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if got := len(listed) == 1; got != tt.visible {
				t.Fatalf("listed = %v, want visible %v", requestReferences(listed), tt.visible)
			}
			_, err = p.svc.RequestDetail(ctx, p.aliceIn, p.bobRequest.Reference)
			if tt.visible && err != nil {
				t.Fatalf("detail: %v", err)
			}
			if !tt.visible && !errors.Is(err, ErrPortalRequestNotFound) {
				t.Fatalf("detail err = %v, want not found", err)
			}
			_, err = p.svc.Reply(ctx, p.aliceIn, p.bobRequest.Reference, "Following up", nil)
			if !tt.visible && !errors.Is(err, ErrPortalRequestNotFound) {
				t.Fatalf("reply err = %v, want not found", err)
			}
			grant, err := p.svc.AuthenticatePortalSocket(ctx, p.ws.Slug, signInSecret(t, p, "alice@acme.test"))
			if err != nil {
				t.Fatal(err)
			}
			if got := len(grant.References) == 1; got != tt.visible {
				t.Fatalf("socket grant = %v, want visible %v", grant.References, tt.visible)
			}
		})
	}
}

// signInSecret exchanges a fresh link and returns the session secret.
func signInSecret(t *testing.T, p *companyPortal, email string) string {
	t.Helper()
	secret, _, err := p.svc.Exchange(context.Background(), p.ws, issuePortalLink(t, p.db, p.ws.ID, email, time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

// Company visibility needs an explicit allowed decision, even where any
// verified email may sign in.
func TestCompanyPortalNeedsExplicitAccess(t *testing.T) {
	p := setupCompanyPortal(t)
	ws := setPortalSettings(t, p.svc, p.db, p.ws, portalSettingsAnyEmail)
	p.exec(t, `UPDATE crm_contacts SET portal_access = NULL WHERE id = ?`, p.alice)
	access := signInPortal(t, p.svc, ws, issuePortalLink(t, p.db, ws.ID, "alice@acme.test", time.Now().Add(-time.Hour)))
	if derefString(access.Session.CRMContactID) != p.alice {
		t.Fatalf("session contact = %q, want Alice", derefString(access.Session.CRMContactID))
	}
	listed, err := p.svc.CompanyRequests(context.Background(), access, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("unapproved contact sees company requests: %v", requestReferences(listed))
	}
}

func TestCompanyPortalCompanyFilter(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	for _, tt := range []struct {
		name, companyID string
		want            int
	}{
		{"member company", p.acme, 1},
		{"unknown company", uuid.NewString(), 0},
	} {
		listed, err := p.svc.CompanyRequests(ctx, p.aliceIn, tt.companyID, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(listed) != tt.want {
			t.Errorf("%s: listed %v, want %d", tt.name, requestReferences(listed), tt.want)
		}
	}
	waiting, err := p.svc.CompanyRequests(ctx, p.aliceIn, "", model.SupportConversationStatusResolved)
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 0 {
		t.Fatalf("status filter ignored: %v", requestReferences(waiting))
	}
}

func TestCompanyPortalColleagueReply(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	detail, err := p.svc.Reply(ctx, p.aliceIn, p.bobRequest.Reference, "I can see this too", nil)
	if err != nil {
		t.Fatal(err)
	}
	last := detail.Messages[len(detail.Messages)-1]
	if !last.FromYou || last.Content != "I can see this too" {
		t.Fatalf("Alice's reply in her view = %+v", last)
	}

	var stored model.SupportMessage
	if err := p.db.Where("conversation_id = ? AND content = ?", p.bobConversationID(t), "I can see this too").Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(stored.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["portal_identity_id"] != p.aliceIn.Identity.ID || derefString(stored.SenderDisplayName) != "Alice Example" {
		t.Fatalf("colleague reply attribution: metadata=%v sender=%q", metadata, derefString(stored.SenderDisplayName))
	}

	bobView, err := p.svc.RequestDetail(ctx, p.bobIn, p.bobRequest.Reference)
	if err != nil {
		t.Fatal(err)
	}
	if len(bobView.Messages) != 2 || !bobView.Messages[0].FromYou || bobView.Messages[1].FromYou {
		t.Fatalf("Bob's view of authorship = %+v", bobView.Messages)
	}
	if !bobView.Own {
		t.Fatal("requester's view is not marked own")
	}

	if _, err := p.svc.RateRequest(ctx, p.aliceIn, p.bobRequest.Reference, 5, ""); !errors.Is(err, ErrSupportCSATUnavailable) {
		t.Fatalf("colleague rating err = %v, want unavailable", err)
	}
}

func TestPortalCustomerAuthorName(t *testing.T) {
	requester := func() *string { name := "Bob Example"; return &name }
	authors := map[string]string{"alice-id": "Alice Example"}
	tests := []struct {
		name     string
		metadata string
		want     string
	}{
		{"portal author", `{"portal_identity_id":"alice-id"}`, "Alice Example"},
		{"unknown portal author stays unnamed", `{"portal_identity_id":"gone"}`, ""},
		{"email sender", `{"email_sender":" carol@acme.test "}`, "carol@acme.test"},
		{"unattributed message is the requester's", "", "Bob Example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &model.SupportMessage{SenderType: "customer", Metadata: tt.metadata}
			if got := derefString(portalCustomerAuthorName(msg, authors, requester)); got != tt.want {
				t.Fatalf("author name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPortalMessageFromViewer(t *testing.T) {
	viewer := &model.SupportPortalIdentity{ID: "me", Email: "me@acme.test"}
	tests := []struct {
		name     string
		sender   string
		metadata string
		own      bool
		want     bool
	}{
		{"my portal reply", "customer", `{"portal_identity_id":"me"}`, false, true},
		{"colleague portal reply on my request", "customer", `{"portal_identity_id":"them"}`, true, false},
		{"my email", "customer", `{"email_sender":"ME@acme.test"}`, false, true},
		{"colleague email on my request", "customer", `{"email_sender":"them@acme.test"}`, true, false},
		{"unattributed message on my request", "customer", "", true, true},
		{"unattributed message on a colleague request", "customer", "{}", false, false},
		{"unreadable metadata keeps ownership", "customer", "{", true, true},
		{"teammate reply", "user", "", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &model.SupportMessage{SenderType: tt.sender, Metadata: tt.metadata}
			if got := portalMessageFromViewer(msg, viewer, tt.own); got != tt.want {
				t.Fatalf("portalMessageFromViewer = %v, want %v", got, tt.want)
			}
		})
	}
}

// A colleague who never used the portal still has their email conversations
// shared with members, under the same continuity rule as their own sign-in.
func TestCompanyPortalPublishesColleagueConversations(t *testing.T) {
	p := setupCompanyPortal(t)
	carol := createNamedPortalContact(t, p.db, p.ws.ID, "Carol", "carol@acme.test", nil)
	linkContactCompany(t, p.db, p.ws.ID, carol, p.acme)
	insert := func(subject, companyID string, hidden bool) string {
		id := uuid.NewString()
		p.exec(t, `INSERT INTO support_conversations (id, workspace_id, subject, status, priority, channel, source, customer_email, crm_contact_id, crm_company_id, portal_visible, display_id, created_at, updated_at)
			VALUES (?, ?, ?, 'open', 'medium', 'email', 'email', 'carol@acme.test', ?, ?, false, ?, ?, ?)`,
			id, p.ws.ID, subject, carol, companyID, time.Now().UnixNano()%100000, time.Now(), time.Now())
		if hidden {
			p.exec(t, `UPDATE support_conversations SET portal_visibility_changed_at = ? WHERE id = ?`, time.Now(), id)
		}
		return id
	}
	shared := insert("Shared outage", p.acme, false)
	insert("Hidden by a teammate", p.acme, true)
	personal := insert("Personal account", createPortalCompany(t, p.db, p.ws.ID, "Personal", nil), false)

	listed, err := p.svc.CompanyRequests(context.Background(), p.aliceIn, "", "")
	if err != nil {
		t.Fatal(err)
	}
	subjects := map[string]bool{}
	for _, request := range listed {
		subjects[request.Subject] = true
	}
	if len(listed) != 2 || !subjects["Shared outage"] || !subjects["Invoices are missing"] {
		t.Fatalf("company requests = %v", subjects)
	}
	var visible []string
	if err := p.db.Table("support_conversations").Where("id IN ? AND portal_visible = true", []string{shared, personal}).Pluck("id", &visible).Error; err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0] != shared {
		t.Fatalf("published conversations = %v, want only the company one", visible)
	}
	var owner string
	if err := p.db.Table("support_portal_request_references AS refs").Joins("JOIN support_portal_identities AS identity ON identity.id = refs.portal_identity_id").
		Where("refs.conversation_id = ?", shared).Pluck("identity.email", &owner).Error; err != nil {
		t.Fatal(err)
	}
	if owner != "carol@acme.test" {
		t.Fatalf("published request owned by %q, want Carol", owner)
	}
}

func TestCompanyPortalAdminAccess(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	blocked := createNamedPortalContact(t, p.db, p.ws.ID, "Blocked", "blocked@acme.test", portalAccess(model.PortalAccessBlocked))
	shared := createNamedPortalContact(t, p.db, p.ws.ID, "Shared", "shared@acme.test", nil)
	createNamedPortalContact(t, p.db, p.ws.ID, "Duplicate", "SHARED@acme.test ", nil)
	noEmail := createNamedPortalContact(t, p.db, p.ws.ID, "No email", "", nil)
	fresh := createNamedPortalContact(t, p.db, p.ws.ID, "Fresh", "fresh@acme.test", nil)
	outside := createNamedPortalContact(t, p.db, p.ws.ID, "Outside", "outside@else.test", nil)
	for _, id := range []string{blocked, shared, noEmail, fresh} {
		linkContactCompany(t, p.db, p.ws.ID, id, p.acme)
	}

	access, err := p.svc.CompanyAccess(ctx, p.ws.ID, p.acme)
	if err != nil {
		t.Fatal(err)
	}
	if derefString(access.DefaultRole) != model.PortalRoleMember || len(access.Contacts) != 6 || !access.PortalEnabled || access.RequestCount != 1 {
		t.Fatalf("company access = %+v", access)
	}
	byName := map[string]int{}
	for i, contact := range access.Contacts {
		byName[contact.FirstName] = i
	}
	if access.Contacts[byName["Shared"]].SharedEmailContacts != 1 || access.Contacts[byName["Bob"]].LastSignedInAt == nil || access.Contacts[byName["Fresh"]].LastSignedInAt != nil {
		t.Fatalf("contact details = %+v", access.Contacts)
	}

	updated, allowed, err := p.svc.AllowCompanyContacts(ctx, p.ws.ID, p.acme, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if allowed != 1 || derefString(updated.Contacts[byName["Fresh"]].PortalAccess) != model.PortalAccessAllowed {
		t.Fatalf("allowed %d contacts: %+v", allowed, updated.Contacts)
	}
	for _, id := range []string{blocked, shared, noEmail, outside} {
		var row struct{ PortalAccess *string }
		if err := p.db.Table("crm_contacts").Select("portal_access").Where("id = ?", id).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		want := ""
		if id == blocked {
			want = model.PortalAccessBlocked
		}
		if derefString(row.PortalAccess) != want {
			t.Errorf("contact %s access changed to %q", id, derefString(row.PortalAccess))
		}
	}

	if _, err := p.svc.SetCompanyDefaultRole(ctx, p.ws.ID, p.acme, "admin", portalAccess("owner")); !errors.Is(err, ErrPortalRoleInvalid) {
		t.Fatalf("invalid role err = %v", err)
	}
	cleared, err := p.svc.SetCompanyDefaultRole(ctx, p.ws.ID, p.acme, "admin", portalAccess(" "))
	if err != nil || cleared.DefaultRole != nil {
		t.Fatalf("clear default role = %+v, %v", cleared, err)
	}
	if _, err := p.svc.SetCompanyDefaultRole(ctx, p.ws.ID, uuid.NewString(), "admin", nil); !errors.Is(err, ErrPortalCompanyNotFound) {
		t.Fatalf("unknown company err = %v", err)
	}
	if _, err := p.svc.CompanyAccess(ctx, p.ws.ID, uuid.NewString()); !errors.Is(err, ErrPortalCompanyNotFound) {
		t.Fatalf("unknown company access err = %v", err)
	}
	if _, _, err := p.svc.AllowCompanyContacts(ctx, p.ws.ID, uuid.NewString(), "admin"); !errors.Is(err, ErrPortalCompanyNotFound) {
		t.Fatalf("unknown company allow err = %v", err)
	}

	contact, err := p.svc.SetContactRole(ctx, p.ws.ID, p.alice, "admin", portalAccess(model.PortalRoleRequester))
	if err != nil || derefString(contact.PortalRole) != model.PortalRoleRequester {
		t.Fatalf("set contact role = %+v, %v", contact, err)
	}
	if _, err := p.svc.SetContactRole(ctx, p.ws.ID, p.alice, "admin", portalAccess("admin")); !errors.Is(err, ErrPortalRoleInvalid) {
		t.Fatalf("invalid contact role err = %v", err)
	}
	if _, err := p.svc.SetContactRole(ctx, p.ws.ID, uuid.NewString(), "admin", nil); !errors.Is(err, ErrPortalContactNotFound) {
		t.Fatalf("unknown contact err = %v", err)
	}
}

func TestCompanyPortalConversationAudience(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	conversationID := p.bobConversationID(t)
	audience, err := portalAudience(t, p, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	// Alice can read Bob's request; Bob, the requester, is not counted.
	if audience == nil || audience.CompanyID != p.acme || audience.CompanyName != "Acme" || audience.Members != 1 {
		t.Fatalf("audience = %+v", audience)
	}

	p.exec(t, `UPDATE crm_companies SET portal_default_role = NULL WHERE id = ?`, p.acme)
	audience, err = portalAudience(t, p, conversationID)
	if err != nil || audience != nil {
		t.Fatalf("audience with company portal off = %+v, %v", audience, err)
	}
	if _, err := p.svc.ConversationPortalContext(ctx, p.ws.ID, uuid.NewString()); !errors.Is(err, ErrPortalRequestNotFound) {
		t.Fatalf("unknown conversation err = %v", err)
	}
}

// portalAudience returns the company part of a conversation's portal context.
func portalAudience(t *testing.T, p *companyPortal, conversationID string) (*repository.PortalCompanyAudience, error) {
	t.Helper()
	portalContext, err := p.svc.ConversationPortalContext(context.Background(), p.ws.ID, conversationID)
	if err != nil {
		return nil, err
	}
	return portalContext.Company, nil
}
