package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Company admin errors returned to customers.
var (
	ErrPortalTeamDenied        = errors.New("only company admins manage the company's people")
	ErrPortalInvitesOff        = errors.New("inviting colleagues is turned off for this company")
	ErrPortalInviteInvalid     = errors.New("invalid invitation")
	ErrPortalLastAdmin         = errors.New("a company needs at least one admin; make someone else admin first")
	ErrPortalInviteLimit       = errors.New("this company sent too many invitations today; try again tomorrow")
	ErrPortalTeamMemberMissing = errors.New("this person is not a member of the company")
)

// Invitation limits: per company a day, and one per address an hour.
const (
	portalInvitesPerCompanyDay = 50
	portalInviteCooldown       = time.Hour
	portalCompanyEventsShown   = 50
)

// PortalTeamMember is a company member as company admins see them.
type PortalTeamMember struct {
	ContactID      string     `json:"contact_id"`
	Name           string     `json:"name"`
	Email          string     `json:"email"`
	Role           string     `json:"role"`
	CanSignIn      bool       `json:"can_sign_in"`
	LastSignedInAt *time.Time `json:"last_signed_in_at,omitempty"`
	You            bool       `json:"you"`
}

// PortalTeam is a company's people and invitation rules for its admins.
type PortalTeam struct {
	CompanyID   string             `json:"company_id"`
	CompanyName string             `json:"company_name"`
	CanInvite   bool               `json:"can_invite"`
	Domains     []string           `json:"domains"`
	AnyDomain   bool               `json:"any_domain"`
	Members     []PortalTeamMember `json:"members"`
}

// requireCompanyAdmin checks, on every request, that the session is an admin
// of the company.
func (s *CustomerPortalService) requireCompanyAdmin(ctx context.Context, access *PortalAccess, companyID string) error {
	memberships, err := s.Companies(ctx, access)
	if err != nil {
		return err
	}
	for _, membership := range memberships {
		if membership.ID == companyID && membership.Role == model.PortalRoleAdmin {
			return nil
		}
	}
	return ErrPortalTeamDenied
}

// Team returns a company's members for one of its admins.
func (s *CustomerPortalService) Team(ctx context.Context, access *PortalAccess, companyID string) (*PortalTeam, error) {
	if err := s.requireCompanyAdmin(ctx, access, companyID); err != nil {
		return nil, err
	}
	return s.team(ctx, access, companyID)
}

// team builds the team view; callers check the admin role first. After an
// admin steps down the view still returns, showing their new role.
func (s *CustomerPortalService) team(ctx context.Context, access *PortalAccess, companyID string) (*PortalTeam, error) {
	settings, err := s.repo.CompanyTeamSettings(ctx, access.Workspace.ID, companyID)
	if err != nil {
		return nil, err
	}
	companyAccess, err := s.repo.CompanyPortalAccess(ctx, access.Workspace.ID, companyID)
	if err != nil {
		return nil, err
	}
	team := &PortalTeam{CompanyID: companyID, CompanyName: settings.Name, CanInvite: settings.AdminInvites, AnyDomain: settings.InviteAnyDomain, Domains: []string{}, Members: []PortalTeamMember{}}
	if domain := companyDomain(settings.Domain); domain != "" {
		team.Domains = append(team.Domains, domain)
	}
	for _, contact := range companyAccess.Contacts {
		if !contact.Member {
			continue
		}
		role := derefString(contact.PortalRole)
		if role == "" {
			role = derefString(companyAccess.DefaultRole)
		}
		name := strings.TrimSpace(contact.FirstName + " " + derefString(contact.LastName))
		team.Members = append(team.Members, PortalTeamMember{
			ContactID: contact.ID, Name: name, Email: derefString(contact.Email), Role: role,
			CanSignIn: derefString(contact.PortalAccess) == model.PortalAccessAllowed, LastSignedInAt: contact.LastSignedInAt,
			You: access.Session.CRMContactID != nil && *access.Session.CRMContactID == contact.ID,
		})
	}
	return team, nil
}

func companyDomain(domain *string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(derefString(domain)), "www."))
}

// PortalInvite is a company admin's invitation of a colleague.
type PortalInvite struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// Invite adds a colleague to the company: it finds or creates their CRM
// contact, makes them a member (allowing sign-in when they had no portal
// decision), and emails them a sign-in link. Blocked people stay blocked.
func (s *CustomerPortalService) Invite(ctx context.Context, access *PortalAccess, companyID string, invite PortalInvite) (*PortalTeam, error) {
	if err := s.requireCompanyAdmin(ctx, access, companyID); err != nil {
		return nil, err
	}
	settings, err := s.repo.CompanyTeamSettings(ctx, access.Workspace.ID, companyID)
	if err != nil {
		return nil, err
	}
	if !settings.AdminInvites {
		return nil, ErrPortalInvitesOff
	}
	address, ok := normalizePortalEmail(invite.Email)
	if !ok {
		return nil, fmt.Errorf("%w: enter an email address", ErrPortalInviteInvalid)
	}
	if !settings.InviteAnyDomain {
		domain := companyDomain(settings.Domain)
		if domain == "" {
			return nil, fmt.Errorf("%w: ask support to set your company's domain first", ErrPortalInviteInvalid)
		}
		if !strings.HasSuffix(address, "@"+domain) {
			return nil, fmt.Errorf("%w: invite addresses ending in @%s", ErrPortalInviteInvalid, domain)
		}
	}
	role := invite.Role
	if role == "" {
		role = model.PortalRoleMember
	}
	if role != model.PortalRoleMember && role != model.PortalRoleRequester {
		return nil, ErrPortalRoleInvalid
	}
	name := strings.Join(strings.Fields(invite.Name), " ")
	if utf8.RuneCountInString(name) > 120 {
		return nil, fmt.Errorf("%w: names can be at most 120 characters", ErrPortalInviteInvalid)
	}
	now := time.Now().UTC()
	forCompany, forAddress, err := s.repo.CompanyInviteCounts(ctx, access.Workspace.ID, companyID, address, now.Add(-24*time.Hour), now.Add(-portalInviteCooldown))
	if err != nil {
		return nil, err
	}
	if forCompany >= portalInvitesPerCompanyDay {
		return nil, ErrPortalInviteLimit
	}
	contacts, err := s.repo.ContactsForEmail(ctx, access.Workspace.ID, address)
	if err != nil {
		return nil, err
	}
	var contactID string
	for _, contact := range contacts {
		if derefString(contact.PortalAccess) == model.PortalAccessBlocked {
			// Support decided this person may not use the portal.
			return nil, fmt.Errorf("%w: this person can't be invited; contact support", ErrPortalInviteInvalid)
		}
		if contactID == "" || derefString(contact.PortalAccess) == model.PortalAccessAllowed {
			contactID = contact.ID
		}
	}
	if contactID == "" {
		created, err := s.createInvitedContact(ctx, access.Workspace.ID, address, name)
		if err != nil {
			return nil, err
		}
		contactID = created
	}
	existing, err := s.repo.CompanyMemberContact(ctx, access.Workspace.ID, companyID, contactID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		if err := s.repo.AddCompanyMembers(ctx, access.Workspace.ID, companyID, []string{contactID}); err != nil {
			return nil, err
		}
		// A role set elsewhere stays; the admin's choice applies to new people.
		if err := s.setRoleIfUnset(ctx, access.Workspace.ID, companyID, contactID, role); err != nil {
			return nil, err
		}
	}
	s.recordCompanyEvent(ctx, access, companyID, contactID, address, model.PortalCompanyEventInvited, nil, &role)
	if forAddress == 0 {
		s.sendLink(ctx, &access.Workspace, address, nil)
	}
	return s.team(ctx, access, companyID)
}

func (s *CustomerPortalService) setRoleIfUnset(ctx context.Context, workspaceID, companyID, contactID, role string) error {
	member, err := s.repo.CompanyMemberContact(ctx, workspaceID, companyID, contactID)
	if err != nil || member == nil || derefString(member.PortalRole) != "" {
		return err
	}
	return s.repo.SetContactPortalRole(ctx, workspaceID, contactID, &role)
}

// createInvitedContact creates the CRM contact for an invited address.
func (s *CustomerPortalService) createInvitedContact(ctx context.Context, workspaceID, address, name string) (string, error) {
	if s.inbox.contactRepo == nil {
		return "", errors.New("contacts are not configured")
	}
	first, last := name, ""
	if index := strings.LastIndex(name, " "); index > 0 {
		first, last = name[:index], name[index+1:]
	}
	if first == "" {
		first = deriveWidgetNameFromEmail(address)
	}
	if first == "" {
		first = address
	}
	source := "portal_invite"
	contact := &model.CRMContact{
		WorkspaceID: workspaceID, FirstName: first, Email: &address,
		LifecycleStage: model.CRMLifecycleLead, LeadStatus: model.CRMLeadStatusNew, Source: &source,
	}
	if last != "" {
		contact.LastName = &last
	}
	displayID, err := s.inbox.contactRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	contact.DisplayID = displayID
	if err := s.inbox.contactRepo.Create(ctx, contact); err != nil {
		return "", err
	}
	return contact.ID, nil
}

// SetMemberRole changes a member's role. The last admin cannot step down.
func (s *CustomerPortalService) SetMemberRole(ctx context.Context, access *PortalAccess, companyID, contactID, role string) (*PortalTeam, error) {
	if err := s.requireCompanyAdmin(ctx, access, companyID); err != nil {
		return nil, err
	}
	if !model.ValidPortalRole(role) {
		return nil, ErrPortalRoleInvalid
	}
	member, err := s.memberForChange(ctx, access, companyID, contactID)
	if err != nil {
		return nil, err
	}
	previous := derefString(member.PortalRole)
	if previous == role {
		return s.team(ctx, access, companyID)
	}
	if previous == model.PortalRoleAdmin {
		if err := s.keepAnAdmin(ctx, access.Workspace.ID, companyID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.SetContactPortalRole(ctx, access.Workspace.ID, contactID, &role); err != nil {
		return nil, err
	}
	var before *string
	if previous != "" {
		before = &previous
	}
	s.recordCompanyEvent(ctx, access, companyID, contactID, derefString(member.Email), model.PortalCompanyEventRoleChanged, before, &role)
	return s.team(ctx, access, companyID)
}

// RemoveMember drops a member from the company. Their own requests stay
// theirs; the last admin cannot be removed.
func (s *CustomerPortalService) RemoveMember(ctx context.Context, access *PortalAccess, companyID, contactID string) (*PortalTeam, error) {
	if err := s.requireCompanyAdmin(ctx, access, companyID); err != nil {
		return nil, err
	}
	member, err := s.memberForChange(ctx, access, companyID, contactID)
	if err != nil {
		return nil, err
	}
	if derefString(member.PortalRole) == model.PortalRoleAdmin {
		if err := s.keepAnAdmin(ctx, access.Workspace.ID, companyID); err != nil {
			return nil, err
		}
	}
	if err := s.repo.SetCompanyMember(ctx, access.Workspace.ID, companyID, contactID, "", false); err != nil {
		return nil, err
	}
	s.recordCompanyEvent(ctx, access, companyID, contactID, derefString(member.Email), model.PortalCompanyEventRemoved, member.PortalRole, nil)
	return s.team(ctx, access, companyID)
}

func (s *CustomerPortalService) memberForChange(ctx context.Context, access *PortalAccess, companyID, contactID string) (*repository.PortalCompanyMemberContact, error) {
	if !validPortalID(contactID) {
		return nil, ErrPortalTeamMemberMissing
	}
	member, err := s.repo.CompanyMemberContact(ctx, access.Workspace.ID, companyID, contactID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrPortalTeamMemberMissing
	}
	return member, nil
}

// keepAnAdmin refuses a change that would leave the company without an admin
// who can sign in.
func (s *CustomerPortalService) keepAnAdmin(ctx context.Context, workspaceID, companyID string) error {
	admins, err := s.repo.CompanyAdminCount(ctx, workspaceID, companyID)
	if err != nil {
		return err
	}
	if admins <= 1 {
		return ErrPortalLastAdmin
	}
	return nil
}

func (s *CustomerPortalService) recordCompanyEvent(ctx context.Context, access *PortalAccess, companyID, contactID, email, eventType string, before, after *string) {
	identityID := access.Identity.ID
	event := &model.SupportPortalCompanyEvent{
		WorkspaceID: access.Workspace.ID, CRMCompanyID: companyID, ActorIdentityID: &identityID,
		SubjectEmail: strings.ToLower(email), EventType: eventType, BeforeValue: before, AfterValue: after,
	}
	if contactID != "" {
		event.SubjectContactID = &contactID
	}
	if err := s.repo.CreateCompanyEvent(ctx, event); err != nil {
		slog.ErrorContext(ctx, "portal company event failed", "workspace_id", access.Workspace.ID, "company_id", companyID, "error", err)
	}
}

// CompanyTeamSettings returns a company's admin switches and recent people
// changes, for support admins.
func (s *CustomerPortalService) CompanyTeamActivity(ctx context.Context, workspaceID, companyID string) (*repository.PortalCompanyTeamSettings, []model.SupportPortalCompanyEvent, error) {
	if !validPortalID(companyID) {
		return nil, nil, ErrPortalCompanyNotFound
	}
	settings, err := s.repo.CompanyTeamSettings(ctx, workspaceID, companyID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, ErrPortalCompanyNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	events, err := s.repo.ListCompanyEvents(ctx, workspaceID, companyID, portalCompanyEventsShown)
	return settings, events, err
}

// SetCompanyTeamSettings saves whether the company's admins may invite
// colleagues, and from which addresses.
func (s *CustomerPortalService) SetCompanyTeamSettings(ctx context.Context, workspaceID, companyID string, adminInvites, anyDomain bool) error {
	if !validPortalID(companyID) {
		return ErrPortalCompanyNotFound
	}
	if err := s.repo.SetCompanyTeamSettings(ctx, workspaceID, companyID, adminInvites, anyDomain); errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPortalCompanyNotFound
	} else if err != nil {
		return err
	}
	return nil
}
