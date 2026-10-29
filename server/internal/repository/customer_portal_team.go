package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

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

// CompanyAdminCount counts the company's members whose effective role is
// admin and who may still sign in.
func (r *CustomerPortalRepository) CompanyAdminCount(ctx context.Context, workspaceID, companyID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("crm_contacts AS contact").
		Joins("JOIN crm_companies AS company ON company.workspace_id = contact.workspace_id AND company.id = ?", companyID).
		Where("contact.workspace_id = ? AND contact.portal_access = ?", workspaceID, model.PortalAccessAllowed).
		Where(portalMemberSQL("contact.id", "company.id")).
		Where("COALESCE(contact.portal_role, company.portal_default_role) = ?", model.PortalRoleAdmin).
		Count(&count).Error
	return count, err
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
	err = invites().Where("subject_email = ? AND occurred_at >= ?", address, addressSince).Count(&forAddress).Error
	return forCompany, forAddress, err
}

// PortalCompanyMemberContact is a company member's contact as company admins
// manage it.
type PortalCompanyMemberContact struct {
	ID           string
	PortalRole   *string
	PortalAccess *string
	Email        *string
}

// CompanyMemberContact returns a member of the company, or nil when the
// contact is not one.
func (r *CustomerPortalRepository) CompanyMemberContact(ctx context.Context, workspaceID, companyID, contactID string) (*PortalCompanyMemberContact, error) {
	var contact PortalCompanyMemberContact
	err := r.db.WithContext(ctx).Table("crm_contacts AS contact").
		Where("contact.workspace_id = ? AND contact.id = ?", workspaceID, contactID).
		Where(portalMemberSQL("contact.id", "?"), companyID).
		Select("contact.id, contact.portal_role, contact.portal_access, contact.email").Take(&contact).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

// AddCompanyMembers makes contacts company members and allows sign-in for
// those without a portal decision, as a support admin adding them does.
func (r *CustomerPortalRepository) AddCompanyMembers(ctx context.Context, workspaceID, companyID string, contactIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return addPortalCompanyMembers(tx, workspaceID, companyID, "", contactIDs)
	})
}
