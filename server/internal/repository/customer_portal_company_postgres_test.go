//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Company visibility on the real ledger: UUID columns, both association
// directions, the role constraints, and the PostgreSQL query plans.
func TestCustomerPortalCompanyPostgres(t *testing.T) {
	db := contactPrivacyDB(t)
	ctx := context.Background()
	repo := NewCustomerPortalRepository(db)
	ws, acme, other := uuid.NewString(), uuid.NewString(), uuid.NewString()
	alice, bob, carol, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	aliceID, bobID := uuid.NewString(), uuid.NewString()
	bobConv, carolConv, hiddenConv := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_companies (id,workspace_id,display_id,name,portal_default_role) VALUES (?,?,'CO-1','Acme','member'), (?,?,'CO-2','Other',NULL)`, acme, ws, other, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts (id,workspace_id,display_id,first_name,last_name,email,portal_access) VALUES
		(?,?,'C-1','Alice','Doe','alice@acme.test','allowed'), (?,?,'C-2','Bob','Roe','bob@acme.test','allowed'),
		(?,?,'C-3','Carol',NULL,'carol@acme.test',NULL), (?,?,'C-4','Outsider',NULL,'out@else.test',NULL)`, alice, ws, bob, ws, carol, ws, outsider, ws)
	privacyExec(t, db, `INSERT INTO crm_associations (workspace_id,from_object_type,from_object_id,to_object_type,to_object_id) VALUES
		(?,'contact',?,'company',?), (?,'company',?,'contact',?), (?,'contact',?,'company',?)`, ws, alice, acme, ws, acme, bob, ws, carol, acme)
	privacyExec(t, db, `INSERT INTO support_portal_identities (id,workspace_id,email,crm_contact_id) VALUES (?,?,'alice@acme.test',?), (?,?,'bob@acme.test',?)`, aliceID, ws, alice, bobID, ws, bob)
	privacyExec(t, db, `INSERT INTO support_portal_sessions (id,workspace_id,identity_id,crm_contact_id,token_hash,expires_at,created_at) VALUES (?,?,?,?,?,?,?)`, uuid.NewString(), ws, aliceID, alice, uuid.NewString(), time.Now().Add(time.Hour), time.Now())
	privacyExec(t, db, `INSERT INTO support_conversations (id,workspace_id,display_id,subject,status,channel,source,customer_email,crm_contact_id,crm_company_id,portal_visible,portal_visibility_changed_at) VALUES
		(?,?,1,'Bob invoices','open','portal','portal','bob@acme.test',?,?,true,NULL),
		(?,?,2,'Carol outage','open','email','email','carol@acme.test',?,?,false,NULL),
		(?,?,3,'Carol private','open','email','email','carol@acme.test',?,?,false,now())`,
		bobConv, ws, bob, acme, carolConv, ws, carol, acme, hiddenConv, ws, carol, acme)
	privacyExec(t, db, `INSERT INTO support_portal_request_references (workspace_id,conversation_id,portal_identity_id,reference) VALUES (?,?,?,'req_bob')`, ws, bobConv, bobID)

	if err := db.Exec(`UPDATE crm_companies SET portal_default_role = 'owner' WHERE id = ?`, acme).Error; err == nil {
		t.Fatal("company role constraint accepted an unknown role")
	}
	if err := db.Exec(`UPDATE crm_contacts SET portal_role = 'admin' WHERE id = ?`, alice).Error; err == nil {
		t.Fatal("contact role constraint accepted an unknown role")
	}

	memberships, err := repo.PortalCompanyMemberships(ctx, ws, alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 1 || memberships[0] != (PortalCompanyMembership{ID: acme, Name: "Acme", Role: model.PortalRoleMember}) {
		t.Fatalf("memberships = %+v", memberships)
	}

	if err := repo.PublishCompanyRequests(ctx, ws, []string{acme}); err != nil {
		t.Fatal(err)
	}
	requests, err := repo.ListCompanyRequests(ctx, ws, []string{acme}, "")
	if err != nil {
		t.Fatal(err)
	}
	subjects := map[string]model.SupportPortalRequest{}
	for _, request := range requests {
		subjects[request.Subject] = request
	}
	if len(requests) != 2 || derefPortalTestString(subjects["Bob invoices"].RequesterName) != "Bob Roe" || derefPortalTestString(subjects["Carol outage"].RequesterName) != "Carol" ||
		derefPortalTestString(subjects["Bob invoices"].CompanyName) != "Acme" || subjects["Bob invoices"].OwnerIdentityID != bobID {
		t.Fatalf("company requests = %+v", requests)
	}
	conv, err := repo.FindCompanyRequest(ctx, ws, []string{acme}, "req_bob")
	if err != nil || conv == nil || conv.ID != bobConv {
		t.Fatalf("find company request = %v, %v", conv, err)
	}
	privacyCheck(t, db, `SELECT portal_visible = false FROM support_conversations WHERE id = ?`, hiddenConv)
	references, err := repo.CompanyRequestReferences(ctx, ws, []string{acme})
	if err != nil || len(references) != 2 || references[bobConv] != "req_bob" {
		t.Fatalf("company references = %v, %v", references, err)
	}

	access, err := repo.CompanyPortalAccess(ctx, ws, acme)
	if err != nil {
		t.Fatal(err)
	}
	if len(access.Contacts) != 3 || access.Contacts[0].FirstName != "Alice" || access.Contacts[0].LastSignedInAt == nil || access.Contacts[1].LastSignedInAt != nil {
		t.Fatalf("company access = %+v", access)
	}
	count, err := repo.CompanyRequestCount(ctx, ws, acme)
	if err != nil || count != 2 {
		t.Fatalf("company request count = %d, %v", count, err)
	}
	allowed, err := repo.AllowCompanyContacts(ctx, ws, acme)
	if err != nil || allowed != 1 {
		t.Fatalf("allowed = %d, %v", allowed, err)
	}
	privacyCheck(t, db, `SELECT portal_access = 'allowed' FROM crm_contacts WHERE id = ?`, carol)
	privacyCheck(t, db, `SELECT portal_access IS NULL FROM crm_contacts WHERE id = ?`, outsider)

	audience, err := repo.ConversationCompanyAudience(ctx, ws, bobConv)
	// Alice and Carol, but not Bob, who asked.
	if err != nil || audience == nil || audience.CompanyID != acme || audience.Members != 2 {
		t.Fatalf("audience = %+v, %v", audience, err)
	}
	if err := repo.SetContactPortalRole(ctx, ws, carol, stringPointer(model.PortalRoleRequester)); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetCompanyPortalDefaultRole(ctx, ws, acme, nil); err != nil {
		t.Fatal(err)
	}
	if audience, err := repo.ConversationCompanyAudience(ctx, ws, bobConv); err != nil || audience != nil {
		t.Fatalf("audience with company portal off = %+v, %v", audience, err)
	}
	if memberships, err := repo.PortalCompanyMemberships(ctx, ws, alice); err != nil || len(memberships) != 0 {
		t.Fatalf("memberships with company portal off = %+v, %v", memberships, err)
	}
}

func derefPortalTestString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// Portal intake events and the deflection report on the real ledger.
func TestCustomerPortalIntakePostgres(t *testing.T) {
	db := contactPrivacyDB(t)
	ctx := context.Background()
	ws := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	portal := NewCustomerPortalRepository(db)
	reports := NewSupportConversationRepository(db)
	now := time.Now().UTC()
	deflected, asked := uuid.NewString(), uuid.NewString()
	article := uuid.NewString()
	for _, step := range []struct {
		intake, event string
		articles      []string
	}{
		{deflected, model.PortalIntakeEventShown, []string{article}},
		{deflected, model.PortalIntakeEventOpened, []string{article}},
		{asked, model.PortalIntakeEventShown, []string{article}},
		{asked, model.PortalIntakeEventOpened, []string{article}},
		{asked, model.PortalIntakeEventSubmitted, []string{article}},
	} {
		if err := portal.RecordIntakeEvent(ctx, ws, step.intake, step.event, step.articles, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO support_portal_intake_events (workspace_id, intake_id, event_type) VALUES (?, ?, 'clicked')`, ws, deflected).Error; err == nil {
		t.Fatal("unknown intake event accepted")
	}
	// Judged after the deflection window has passed.
	summary, err := reports.ReportPortalIntake(ctx, ws, now.Add(-time.Hour), now.Add(time.Minute), now.Add(PortalIntakeDeflectionWindow+time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	want := model.SupportReportPortalIntake{WithSuggestions: 2, Opened: 2, SubmittedAfterOpening: 1, LikelyDeflected: 1}
	if summary == nil || *summary != want {
		t.Fatalf("summary = %+v, want %+v", summary, want)
	}
	privacyCheck(t, db, `SELECT article_ids = jsonb_build_array(?::text) AND article_count = 1 FROM support_portal_intake_events WHERE intake_id = ? AND event_type = 'shown'`, article, deflected)
}
