package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PortalContactAccess is a CRM contact's portal decision for one email.
type PortalContactAccess struct {
	ID           string
	PortalAccess *string
}

// PortalAccessSummary counts contact portal decisions in a workspace.
type PortalAccessSummary struct {
	AllowedContacts int64
	// Conflicts counts emails with two or more allowed contacts, or with an
	// allowed and a blocked contact. Such emails are never eligible.
	Conflicts int64
	// ConflictEmails lists up to portalConflictEmailLimit conflicting emails.
	ConflictEmails []string
}

const portalConflictEmailLimit = 20

// PortalContactAccessDetail is one contact's decision and how many other
// contacts share its email.
type PortalContactAccessDetail struct {
	PortalAccess        *string
	Email               string
	SharedEmailContacts int64
}

// ContactsForEmail returns every contact in the workspace with the email,
// compared as lower(trim(email)).
func (r *CustomerPortalRepository) ContactsForEmail(ctx context.Context, workspaceID, email string) ([]PortalContactAccess, error) {
	var contacts []PortalContactAccess
	err := r.db.WithContext(ctx).Table("crm_contacts").Select("id, portal_access").
		Where("workspace_id = ? AND lower(trim(email)) = ?", workspaceID, NormalizePortalEmail(email)).
		Order("id").Scan(&contacts).Error
	return contacts, err
}

// BindIdentityContact records the contact a portal identity signed in as.
func (r *CustomerPortalRepository) BindIdentityContact(ctx context.Context, workspaceID, identityID string, contactID *string) error {
	return r.db.WithContext(ctx).Model(&model.SupportPortalIdentity{}).
		Where("workspace_id = ? AND id = ?", workspaceID, identityID).
		Update("crm_contact_id", contactID).Error
}

// RevokeIdentitySessions revokes every active session of an identity.
func (r *CustomerPortalRepository) RevokeIdentitySessions(ctx context.Context, workspaceID, identityID string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.PortalSession{}).
		Where("workspace_id = ? AND identity_id = ? AND revoked_at IS NULL", workspaceID, identityID).
		Update("revoked_at", now).Error
}

// RevokeSessionByID revokes one session.
func (r *CustomerPortalRepository) RevokeSessionByID(ctx context.Context, sessionID string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.PortalSession{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).Update("revoked_at", now).Error
}

// SetContactPortalAccess records a support admin's decision; nil clears it.
// It is the only writer of crm_contacts.portal_access.
func (r *CustomerPortalRepository) SetContactPortalAccess(ctx context.Context, workspaceID, contactID string, access *string) error {
	result := r.db.WithContext(ctx).Table("crm_contacts").
		Where("workspace_id = ? AND id = ?", workspaceID, contactID).
		Updates(map[string]any{"portal_access": access, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ContactPortalAccess returns a contact's decision and email sharing count.
func (r *CustomerPortalRepository) ContactPortalAccess(ctx context.Context, workspaceID, contactID string) (*PortalContactAccessDetail, error) {
	var contact struct {
		Email        *string
		PortalAccess *string
	}
	err := r.db.WithContext(ctx).Table("crm_contacts").Select("email, portal_access").
		Where("workspace_id = ? AND id = ?", workspaceID, contactID).Take(&contact).Error
	if err != nil {
		return nil, err
	}
	detail := &PortalContactAccessDetail{PortalAccess: contact.PortalAccess, Email: NormalizePortalEmail(crmContactStringValue(contact.Email))}
	if detail.Email == "" {
		return detail, nil
	}
	err = r.db.WithContext(ctx).Table("crm_contacts").
		Where("workspace_id = ? AND lower(trim(email)) = ? AND id <> ?", workspaceID, detail.Email, contactID).
		Count(&detail.SharedEmailContacts).Error
	return detail, err
}

// AccessSummary counts allowed contacts and conflicting emails.
func (r *CustomerPortalRepository) AccessSummary(ctx context.Context, workspaceID string) (PortalAccessSummary, error) {
	var summary PortalAccessSummary
	db := r.db.WithContext(ctx)
	if err := db.Table("crm_contacts").
		Where("workspace_id = ? AND portal_access = ?", workspaceID, model.PortalAccessAllowed).
		Count(&summary.AllowedContacts).Error; err != nil {
		return summary, err
	}
	conflicts := db.Table("crm_contacts").
		Select("lower(trim(email))").
		Where("workspace_id = ? AND portal_access IS NOT NULL AND email IS NOT NULL AND trim(email) <> ''", workspaceID).
		Group("lower(trim(email))").
		Having("SUM(CASE WHEN portal_access = ? THEN 1 ELSE 0 END) >= 2 OR (SUM(CASE WHEN portal_access = ? THEN 1 ELSE 0 END) >= 1 AND SUM(CASE WHEN portal_access = ? THEN 1 ELSE 0 END) >= 1)",
			model.PortalAccessAllowed, model.PortalAccessAllowed, model.PortalAccessBlocked)
	if err := db.Table("(?) AS conflicts", conflicts).Count(&summary.Conflicts).Error; err != nil {
		return summary, err
	}
	err := conflicts.Session(&gorm.Session{}).Order("lower(trim(email))").Limit(portalConflictEmailLimit).
		Pluck("lower(trim(email))", &summary.ConflictEmails).Error
	return summary, err
}

// FindHiddenIntakeRequest returns an anonymous intake request that has not
// been published to a portal, or nil.
func (r *CustomerPortalRepository) FindHiddenIntakeRequest(ctx context.Context, workspaceID, conversationID string) (*model.SupportConversation, error) {
	var conversation model.SupportConversation
	err := r.db.WithContext(ctx).Table("support_conversations AS conv").Select("conv.*").
		Where("conv.workspace_id = ? AND conv.id = ? AND conv.channel = 'portal' AND conv.source = 'portal'", workspaceID, conversationID).
		Where("conv.portal_visible = false AND conv.portal_visibility_changed_at IS NULL AND conv.anonymized_at IS NULL").
		Where(portalEligibleConversation).Take(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// IntakeRequestCountsSince counts portal requests created since the given
// time, for the address and for the workspace. It throttles intake notices,
// which have no sign-in link row to count.
func (r *CustomerPortalRepository) IntakeRequestCountsSince(ctx context.Context, workspaceID, email string, since time.Time) (forEmail, forWorkspace int64, err error) {
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("support_conversations").
			Where("workspace_id = ? AND channel = 'portal' AND source = 'portal' AND created_at > ?", workspaceID, since)
	}
	if err = base().Where("lower(customer_email) = ?", NormalizePortalEmail(email)).Count(&forEmail).Error; err != nil {
		return 0, 0, err
	}
	err = base().Count(&forWorkspace).Error
	return forEmail, forWorkspace, err
}

// WorkspaceByID finds a workspace by ID.
func (r *CustomerPortalRepository) WorkspaceByID(ctx context.Context, id string) (*model.Workspace, error) {
	var ws model.Workspace
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&ws).Error; err != nil {
		return nil, err
	}
	return &ws, nil
}

// CreateAuditEvent stores a portal audit event.
func (r *CustomerPortalRepository) CreateAuditEvent(ctx context.Context, event *model.SupportPortalAuditEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}
