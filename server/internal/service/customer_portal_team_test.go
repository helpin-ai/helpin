package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func teamEmails(team *PortalTeam) string {
	emails := make([]string, 0, len(team.Members))
	for _, member := range team.Members {
		emails = append(emails, member.Email+":"+member.Role)
	}
	return strings.Join(emails, ",")
}

func TestPortalCompanyAdminManagesTeam(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	sender := p.svc.sender.(*recordingPortalSender)
	p.exec(t, `UPDATE crm_companies SET domain = 'acme.test' WHERE id = ?`, p.acme)
	p.exec(t, `UPDATE crm_contacts SET portal_role = 'admin' WHERE id = ?`, p.alice)

	if _, err := p.svc.Team(ctx, p.bobIn, p.acme); !errors.Is(err, ErrPortalTeamDenied) {
		t.Fatalf("member reading the team = %v", err)
	}
	team, err := p.svc.Team(ctx, p.aliceIn, p.acme)
	if err != nil || teamEmails(team) != "alice@acme.test:admin,bob@acme.test:member" || !team.Members[0].You || team.CanInvite {
		t.Fatalf("team = %+v, %v", team, err)
	}
	if _, err := p.svc.Invite(ctx, p.aliceIn, p.acme, PortalInvite{Email: "dan@acme.test"}); !errors.Is(err, ErrPortalInvitesOff) {
		t.Fatalf("invite with invites off = %v", err)
	}
	if err := p.svc.SetCompanyTeamSettings(ctx, p.ws.ID, p.acme, true, false); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"eve@other.test", "not an email"} {
		if _, err := p.svc.Invite(ctx, p.aliceIn, p.acme, PortalInvite{Email: email}); !errors.Is(err, ErrPortalInviteInvalid) {
			t.Errorf("invite %q = %v", email, err)
		}
	}
	if _, err := p.svc.Invite(ctx, p.aliceIn, p.acme, PortalInvite{Email: "dan@acme.test", Role: model.PortalRoleAdmin}); !errors.Is(err, ErrPortalRoleInvalid) {
		t.Fatalf("invite as admin = %v", err)
	}
	sentBefore := len(sender.sent)
	team, err = p.svc.Invite(ctx, p.aliceIn, p.acme, PortalInvite{Email: "Dan@Acme.test", Name: "Dan Daly", Role: model.PortalRoleRequester})
	if err != nil || !strings.Contains(teamEmails(team), "dan@acme.test:requester") {
		t.Fatalf("invite = %+v, %v", team, err)
	}
	if len(sender.sent) != sentBefore+1 || sender.sent[sentBefore].to != "dan@acme.test" {
		t.Fatalf("invite email = %#v", sender.sent[sentBefore:])
	}
	// Inviting again within the hour sends no second email.
	if _, err := p.svc.Invite(ctx, p.aliceIn, p.acme, PortalInvite{Email: "dan@acme.test"}); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != sentBefore+1 {
		t.Fatalf("emails after re-inviting = %d", len(sender.sent)-sentBefore)
	}

	// The last admin cannot step down or leave; with another admin they can.
	var aliceID string
	for _, member := range team.Members {
		if member.Email == "alice@acme.test" {
			aliceID = member.ContactID
		}
	}
	if _, err := p.svc.SetMemberRole(ctx, p.aliceIn, p.acme, aliceID, model.PortalRoleMember); !errors.Is(err, ErrPortalLastAdmin) {
		t.Fatalf("last admin stepping down = %v", err)
	}
	if _, err := p.svc.RemoveMember(ctx, p.aliceIn, p.acme, aliceID); !errors.Is(err, ErrPortalLastAdmin) {
		t.Fatalf("last admin leaving = %v", err)
	}
	if _, err := p.svc.SetMemberRole(ctx, p.aliceIn, p.acme, p.bob, model.PortalRoleAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := p.svc.SetMemberRole(ctx, p.aliceIn, p.acme, aliceID, model.PortalRoleMember); err != nil {
		t.Fatalf("stepping down with another admin: %v", err)
	}
	// Alice is no longer an admin.
	if _, err := p.svc.Team(ctx, p.aliceIn, p.acme); !errors.Is(err, ErrPortalTeamDenied) {
		t.Fatalf("former admin reading the team = %v", err)
	}
	team, err = p.svc.Team(ctx, p.bobIn, p.acme)
	if err != nil {
		t.Fatal(err)
	}
	var danID string
	for _, member := range team.Members {
		if member.Email == "dan@acme.test" {
			danID = member.ContactID
		}
	}
	if team, err = p.svc.RemoveMember(ctx, p.bobIn, p.acme, danID); err != nil || strings.Contains(teamEmails(team), "dan@") {
		t.Fatalf("remove = %+v, %v", team, err)
	}
	if _, err := p.svc.RemoveMember(ctx, p.bobIn, p.acme, danID); !errors.Is(err, ErrPortalTeamMemberMissing) {
		t.Fatalf("remove twice = %v", err)
	}

	_, activity, err := p.svc.CompanyTeamActivity(ctx, p.ws.ID, p.acme)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, event := range activity {
		kinds = append(kinds, event.EventType+":"+event.SubjectEmail)
	}
	if strings.Join(kinds, ",") != "removed:dan@acme.test,role_changed:alice@acme.test,role_changed:bob@acme.test,invited:dan@acme.test,invited:dan@acme.test" {
		t.Fatalf("activity = %v", kinds)
	}
}

func TestPortalCompanyInviteRespectsBlocks(t *testing.T) {
	p := setupCompanyPortal(t)
	ctx := context.Background()
	p.exec(t, `UPDATE crm_contacts SET portal_role = 'admin' WHERE id = ?`, p.alice)
	if err := p.svc.SetCompanyTeamSettings(ctx, p.ws.ID, p.acme, true, true); err != nil {
		t.Fatal(err)
	}
	createNamedPortalContact(t, p.db, p.ws.ID, "Mallory", "mallory@else.test", portalAccess(model.PortalAccessBlocked))
	if _, err := p.svc.Invite(ctx, p.aliceIn, p.acme, PortalInvite{Email: "mallory@else.test"}); !errors.Is(err, ErrPortalInviteInvalid) {
		t.Fatalf("invite blocked person = %v", err)
	}
	// Any address is fine when support widened invitations.
	if _, err := p.svc.Invite(ctx, p.aliceIn, p.acme, PortalInvite{Email: "partner@else.test"}); err != nil {
		t.Fatalf("invite with any domain: %v", err)
	}
	if _, err := p.svc.SetCompanyDefaultRole(ctx, p.ws.ID, p.acme, "", portalAccess(model.PortalRoleAdmin)); !errors.Is(err, ErrPortalRoleInvalid) {
		t.Fatalf("admin as company default = %v", err)
	}
}
