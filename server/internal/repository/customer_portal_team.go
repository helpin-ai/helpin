package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PortalCompanyTeamSettings are a company's switches for its portal admins.
type PortalCompanyTeamSettings struct {
	Name            string  `json:"name"`
	Domain          *string `json:"domain,omitempty"`
	AdminInvites    bool    `json:"admin_invites" gorm:"column:portal_admin_invites"`
	InviteAnyDomain bool    `json:"invite_any_domain" gorm:"column:portal_invite_any_domain"`
}

// CompanyTeamSettings returns a company's portal admin switches, or
// gorm.ErrRecordNotFound.
func (r *CustomerPortalRepository) CompanyTeamSettings(ctx context.Context, workspaceID, companyID string) (*PortalCompanyTeamSettings, error) {
	var settings PortalCompanyTeamSettings
	err := r.db.WithContext(ctx).Table("crm_companies").
		Select("name, domain, portal_admin_invites, portal_invite_any_domain").
		Where("workspace_id = ? AND id = ?", workspaceID, companyID).Take(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// SetCompanyTeamSettings saves a company's portal admin switches. It returns
// gorm.ErrRecordNotFound for an unknown company.
func (r *CustomerPortalRepository) SetCompanyTeamSettings(ctx context.Context, workspaceID, companyID string, adminInvites, inviteAnyDomain bool) error {
	result := r.db.WithContext(ctx).Table("crm_companies").Where("workspace_id = ? AND id = ?", workspaceID, companyID).
		Updates(map[string]any{"portal_admin_invites": adminInvites, "portal_invite_any_domain": inviteAnyDomain, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CreateCompanyEvent records a change to a company's people.
func (r *CustomerPortalRepository) CreateCompanyEvent(ctx context.Context, event *model.SupportPortalCompanyEvent) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	return r.db.WithContext(ctx).Create(event).Error
}

// ListCompanyEvents returns a company's latest people changes, newest first,
// with who acted.
func (r *CustomerPortalRepository) ListCompanyEvents(ctx context.Context, workspaceID, companyID string, limit int) ([]model.SupportPortalCompanyEvent, error) {
	var events []model.SupportPortalCompanyEvent
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND crm_company_id = ?", workspaceID, companyID).
		Order("occurred_at DESC, id DESC").Limit(limit).Find(&events).Error; err != nil {
		return nil, err
	}
	for i := range events {
		var name string
		switch {
		case events[i].ActorIdentityID != nil:
			r.db.WithContext(ctx).Table("support_portal_identities").Where("id = ?", *events[i].ActorIdentityID).
				Select("COALESCE(NULLIF(display_name, ''), email)").Scan(&name)
		case events[i].ActorUserID != nil:
			r.db.WithContext(ctx).Table("users").Where("id = ?", *events[i].ActorUserID).Select("full_name").Scan(&name)
		}
		events[i].ActorName = name
	}
	if events == nil {
		events = []model.SupportPortalCompanyEvent{}
	}
	return events, nil
}

// CompanyInviteCounts counts portal invitations for the company since
// since, and to address since addressSince.
func (r *CustomerPortalRepository) CompanyInviteCounts(ctx context.Context, workspaceID, companyID, address string, since, addressSince time.Time) (forCompany, forAddress int64, err error) {
	invites := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.SupportPortalCompanyEvent{}).
			Where("workspace_id = ? AND crm_company_id = ? AND event_type = ? AND actor_identity_id IS NOT NULL", workspaceID, companyID, model.PortalCompanyEventInvited)
	}
	if err = invites().Where("occurred_at >= ?", since).Count(&forCompany).Error; err != nil {
		return 0, 0, err
	}
	// The cooldown holds across companies, so one address gets one email.
	err = r.db.WithContext(ctx).Model(&model.SupportPortalCompanyEvent{}).
		Where("workspace_id = ? AND event_type = ? AND subject_email = ? AND occurred_at >= ?", workspaceID, model.PortalCompanyEventInvited, address, addressSince).
		Count(&forAddress).Error
	return forCompany, forAddress, err
}

// ErrPortalLastCompanyAdmin means a change would leave the company without
// an admin who can sign in.
var ErrPortalLastCompanyAdmin = errors.New("a company needs at least one admin")

// PortalCompanyMemberContact is a company member's contact as company admins
// manage it.
type PortalCompanyMemberContact struct {
	ID           string
	PortalAccess *string
	Email        *string
	// Role is the effective role in the company; MemberRole the membership's own.
	Role       *string
	MemberRole *string
}

// CompanyMemberContact returns a member of the company, or nil when the
// contact is not one.
func (r *CustomerPortalRepository) CompanyMemberContact(ctx context.Context, workspaceID, companyID, contactID string) (*PortalCompanyMemberContact, error) {
	return companyMemberContact(r.db.WithContext(ctx), workspaceID, companyID, contactID)
}

func companyMemberContact(db *gorm.DB, workspaceID, companyID, contactID string) (*PortalCompanyMemberContact, error) {
	var contact PortalCompanyMemberContact
	err := db.Table("crm_contacts AS contact").
		Joins("JOIN crm_companies AS company ON company.workspace_id = contact.workspace_id AND company.id = ?", companyID).
		Joins("JOIN support_portal_company_members AS membership ON membership.crm_company_id = company.id AND membership.crm_contact_id = contact.id").
		Where("contact.workspace_id = ? AND contact.id = ?", workspaceID, contactID).
		Select("contact.id, contact.portal_access, contact.email, membership.role AS member_role, " +
			"COALESCE(membership.role, contact.portal_role, company.portal_default_role) AS role").Take(&contact).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

// OtherCompanyMemberships counts the contact's memberships in companies
// other than companyID.
func (r *CustomerPortalRepository) OtherCompanyMemberships(ctx context.Context, workspaceID, companyID, contactID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("support_portal_company_members").
		Where("workspace_id = ? AND crm_contact_id = ? AND crm_company_id <> ?", workspaceID, contactID, companyID).Count(&count).Error
	return count, err
}

// lockCompanyAdmins locks the company row, so admin counts and changes for
// one company happen one at a time, and returns how many members other than
// exceptContactID are admins who can sign in.
func lockCompanyAdmins(tx *gorm.DB, dialect, workspaceID, companyID, exceptContactID string) (int64, error) {
	lock := tx.Table("crm_companies").Select("id").Where("workspace_id = ? AND id = ?", workspaceID, companyID)
	if dialect == "postgres" {
		lock = lock.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var company struct{ ID string }
	if err := lock.Take(&company).Error; err != nil {
		return 0, err
	}
	var admins int64
	err := tx.Table("crm_contacts AS contact").
		Joins("JOIN crm_companies AS company ON company.workspace_id = contact.workspace_id AND company.id = ?", companyID).
		Where("contact.workspace_id = ? AND contact.id <> ? AND contact.portal_access = ?", workspaceID, exceptContactID, model.PortalAccessAllowed).
		Where(portalMemberSQL("contact.id", "company.id")).
		Where(portalCompanyRoleSQL("contact.id", "company.id", "company")+" = ?", model.PortalRoleAdmin).
		Count(&admins).Error
	return admins, err
}

// changeKeepsAnAdmin reports whether changing the member keeps an admin
// who can sign in: true when they are not one, or when another is.
func changeKeepsAnAdmin(member *PortalCompanyMemberContact, otherAdmins int64) bool {
	isSigningInAdmin := crmContactStringValue(member.Role) == model.PortalRoleAdmin && crmContactStringValue(member.PortalAccess) == model.PortalAccessAllowed
	return !isSigningInAdmin || otherAdmins > 0
}

// SetCompanyMemberRole sets the role of a contact's membership in the
// company (nil follows the contact's role and the company default). A change
// that would leave no admin who can sign in fails with
// ErrPortalLastCompanyAdmin; a contact who is not a member yields nil, nil.
func (r *CustomerPortalRepository) SetCompanyMemberRole(ctx context.Context, workspaceID, companyID, contactID string, role *string) (*PortalCompanyMemberContact, error) {
	var before *PortalCompanyMemberContact
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		others, err := lockCompanyAdmins(tx, r.db.Dialector.Name(), workspaceID, companyID, contactID)
		if err != nil {
			return err
		}
		if before, err = companyMemberContact(tx, workspaceID, companyID, contactID); err != nil || before == nil {
			return err
		}
		if (role == nil || *role != model.PortalRoleAdmin) && !changeKeepsAnAdmin(before, others) {
			return ErrPortalLastCompanyAdmin
		}
		return tx.Table("support_portal_company_members").Where("crm_company_id = ? AND crm_contact_id = ?", companyID, contactID).
			Update("role", role).Error
	})
	return before, err
}

// RemoveCompanyMemberKeepingAdmin drops a membership unless it would leave no
// admin who can sign in. A contact who is not a member yields nil, nil.
func (r *CustomerPortalRepository) RemoveCompanyMemberKeepingAdmin(ctx context.Context, workspaceID, companyID, contactID string) (*PortalCompanyMemberContact, error) {
	var before *PortalCompanyMemberContact
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		others, err := lockCompanyAdmins(tx, r.db.Dialector.Name(), workspaceID, companyID, contactID)
		if err != nil {
			return err
		}
		if before, err = companyMemberContact(tx, workspaceID, companyID, contactID); err != nil || before == nil {
			return err
		}
		if !changeKeepsAnAdmin(before, others) {
			return ErrPortalLastCompanyAdmin
		}
		return tx.Exec(`DELETE FROM support_portal_company_members WHERE crm_company_id = ? AND crm_contact_id = ?`, companyID, contactID).Error
	})
	return before, err
}

// AddCompanyMembers makes contacts company members and allows sign-in for
// those without a portal decision, as a support admin adding them does.
func (r *CustomerPortalRepository) AddCompanyMembers(ctx context.Context, workspaceID, companyID string, contactIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return addPortalCompanyMembers(tx, workspaceID, companyID, "", contactIDs)
	})
}
