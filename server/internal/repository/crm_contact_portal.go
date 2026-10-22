package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrPortalProtectedContact rejects email changes and deletion of a contact
// with a customer portal decision by callers without support admin authority.
// Changing such a contact's email could grant portal access to another
// address, and deleting a blocked contact would lift its block.
var ErrPortalProtectedContact = errors.New("this contact has customer portal access settings; a support admin must change its email or delete it")

type portalAccessAuthorityKey struct{}

// WithPortalAccessAuthority marks ctx as acting for a support admin, who may
// change the email of or delete a contact with a portal decision.
func WithPortalAccessAuthority(ctx context.Context) context.Context {
	return context.WithValue(ctx, portalAccessAuthorityKey{}, true)
}

// HasPortalAccessAuthority reports whether ctx acts for a support admin.
func HasPortalAccessAuthority(ctx context.Context) bool {
	authorized, _ := ctx.Value(portalAccessAuthorityKey{}).(bool)
	return authorized
}

// NormalizePortalEmail is the single email normalization for portal
// eligibility. SQL comparisons use lower(trim(email)) to match it and the
// idx_crm_contacts_workspace_normalized_email index.
func NormalizePortalEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// guardPortalProtectedContact locks the stored contact and refuses an email
// change or deletion (newEmail nil with deleting true) without authority.
func guardPortalProtectedContact(ctx context.Context, tx *gorm.DB, workspaceID, contactID string, newEmail *string, deleting bool) error {
	var current struct {
		Email        *string
		PortalAccess *string
	}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("crm_contacts").
		Select("email, portal_access").
		Where("workspace_id = ? AND id = ?", workspaceID, contactID).
		Take(&current).Error
	if err != nil {
		return err
	}
	if current.PortalAccess == nil || HasPortalAccessAuthority(ctx) {
		return nil
	}
	if deleting || NormalizePortalEmail(crmContactStringValue(current.Email)) != NormalizePortalEmail(crmContactStringValue(newEmail)) {
		return ErrPortalProtectedContact
	}
	return nil
}
