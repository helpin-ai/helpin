package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// conversationCCContainsSQL is true when the conversation's Cc list holds
// the bound lowercase address.
func conversationCCContainsSQL(dialect string) string {
	if dialect == "postgres" {
		return "EXISTS (SELECT 1 FROM unnest(conv.email_cc) AS cc(address) WHERE lower(cc.address) = ?)"
	}
	// SQLite stores the quoted array literal as text; instr matches exactly,
	// where LIKE would treat _ and % in addresses as wildcards.
	return "instr(',' || replace(lower(trim(COALESCE(conv.email_cc, ''), '{}')), '\"', '') || ',', ',' || ? || ',') > 0"
}

// copiedRequests selects the published requests that copy email, other than
// the identity's own. Being copied shows a request; it never grants sign-in.
// Erased conversations and requests of blocked requesters are never shown.
func (r *CustomerPortalRepository) copiedRequests(ctx context.Context, workspaceID, identityID, email string) *gorm.DB {
	return r.db.WithContext(ctx).Table("support_portal_request_references AS refs").
		Joins("JOIN support_conversations AS conv ON conv.id = refs.conversation_id AND conv.workspace_id = refs.workspace_id").
		Joins("JOIN support_portal_identities AS identity ON identity.id = refs.portal_identity_id AND identity.workspace_id = refs.workspace_id").
		Joins("LEFT JOIN crm_contacts AS requester ON requester.workspace_id = conv.workspace_id AND requester.id = conv.crm_contact_id").
		Where("refs.workspace_id = ? AND refs.portal_identity_id <> ?", workspaceID, identityID).
		Where(conversationCCContainsSQL(r.db.Dialector.Name()), strings.ToLower(strings.TrimSpace(email))).
		Where("conv.anonymized_at IS NULL AND (requester.portal_access IS NULL OR requester.portal_access <> ?)", model.PortalAccessBlocked).
		Where(portalVisibleConversation).
		Where(portalContinuityGuard, model.IdentityTrustVerified, model.IdentityTrustVerified)
}

// ListCopiedRequests returns the requests that copy email, newest activity
// first, with who asked.
func (r *CustomerPortalRepository) ListCopiedRequests(ctx context.Context, workspaceID, identityID, email, status string) ([]model.SupportPortalRequest, error) {
	if strings.TrimSpace(email) == "" {
		return []model.SupportPortalRequest{}, nil
	}
	return r.listRequests(r.copiedRequests(ctx, workspaceID, identityID, email), `,
		NULLIF(trim(COALESCE(requester.first_name, '') || ' ' || COALESCE(requester.last_name, '')), '') AS requester_name,
		lower(conv.customer_email) AS requester_email`, status)
}

// FindCopiedRequest resolves a reference among the requests that copy email,
// or nil.
func (r *CustomerPortalRepository) FindCopiedRequest(ctx context.Context, workspaceID, identityID, email, reference string) (*model.SupportConversation, error) {
	if strings.TrimSpace(email) == "" {
		return nil, nil
	}
	var conversation model.SupportConversation
	err := r.copiedRequests(ctx, workspaceID, identityID, email).Select("conv.*").
		Where("refs.reference = ?", reference).Take(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// PortalColleague is someone a requester may copy in "colleagues" mode.
type PortalColleague struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// PortalColleagues returns the people a contact may copy in colleagues
// mode: other allowed members of the companies where the contact has the
// member role, and the email domains of all its portal companies. A contact
// without explicit portal access has none, as for company visibility.
func (r *CustomerPortalRepository) PortalColleagues(ctx context.Context, workspaceID, contactID string) ([]PortalColleague, []string, error) {
	memberships, err := r.PortalCompanyMemberships(ctx, workspaceID, contactID)
	if err != nil || len(memberships) == 0 {
		return []PortalColleague{}, nil, err
	}
	all := make([]string, 0, len(memberships))
	var memberOf []string
	for _, membership := range memberships {
		all = append(all, membership.ID)
		if membership.Role == model.PortalRoleMember {
			memberOf = append(memberOf, membership.ID)
		}
	}
	var rawDomains []string
	if err := r.db.WithContext(ctx).Table("crm_companies").Where("workspace_id = ? AND id IN ? AND domain IS NOT NULL", workspaceID, all).
		Pluck("domain", &rawDomains).Error; err != nil {
		return nil, nil, err
	}
	var domains []string
	for _, raw := range rawDomains {
		if domain := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "www.")); domain != "" {
			domains = append(domains, domain)
		}
	}
	colleagues := []PortalColleague{}
	if len(memberOf) == 0 {
		return colleagues, domains, nil
	}
	err = r.db.WithContext(ctx).Table("support_portal_company_members AS member").
		Joins("JOIN crm_contacts AS contact ON contact.id = member.crm_contact_id AND contact.workspace_id = member.workspace_id").
		Where("member.workspace_id = ? AND member.crm_company_id IN ? AND member.crm_contact_id <> ?", workspaceID, memberOf, contactID).
		Where("contact.email IS NOT NULL AND contact.email <> '' AND contact.portal_access = ?", model.PortalAccessAllowed).
		Select("DISTINCT lower(contact.email) AS email, trim(COALESCE(contact.first_name, '') || ' ' || COALESCE(contact.last_name, '')) AS name").
		Order("email").Scan(&colleagues).Error
	return colleagues, domains, err
}

// CopyNoticeCounts are recent copy notices, by scope.
type CopyNoticeCounts struct {
	ForRequest, ForAddress, ForRequester, ForWorkspace int64
}

// CopyNoticeCounts counts copy notices: to address about the conversation,
// to address at all, and sent for the requester, since daySince; and by the
// workspace since hourSince.
func (r *CustomerPortalRepository) CopyNoticeCounts(ctx context.Context, workspaceID, conversationID, requesterIdentityID, address string, daySince, hourSince time.Time) (CopyNoticeCounts, error) {
	var counts CopyNoticeCounts
	notices := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.SupportPortalAuditEvent{}).
			Where("workspace_id = ? AND event_type = ?", workspaceID, model.SupportPortalAuditCCNotified)
	}
	metadata, err := json.Marshal(map[string]string{"notified": address})
	if err != nil {
		return counts, err
	}
	for _, query := range []struct {
		target *int64
		scope  *gorm.DB
	}{
		{&counts.ForRequest, notices().Where("conversation_id = ? AND metadata = ? AND occurred_at >= ?", conversationID, string(metadata), daySince)},
		{&counts.ForAddress, notices().Where("metadata = ? AND occurred_at >= ?", string(metadata), daySince)},
		{&counts.ForRequester, notices().Where("portal_identity_id = ? AND occurred_at >= ?", requesterIdentityID, daySince)},
		{&counts.ForWorkspace, notices().Where("occurred_at >= ?", hourSince)},
	} {
		if err := query.scope.Count(query.target).Error; err != nil {
			return counts, err
		}
	}
	return counts, nil
}

// UpdateConversationCC applies one change to a conversation's Cc list under
// a row lock, keeping addresses others added meanwhile, and returns the
// stored list. Merged sources are read-only.
func (r *CustomerPortalRepository) UpdateConversationCC(ctx context.Context, workspaceID, conversationID string, added, removed []string) ([]string, error) {
	var stored []string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation model.SupportConversation
		query := tx.Select("id", "email_cc").Where("workspace_id = ? AND id = ? AND merged_into_id IS NULL", workspaceID, conversationID)
		if r.db.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Take(&conversation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSupportMerged
			}
			return err
		}
		drop := map[string]bool{}
		for _, address := range removed {
			drop[strings.ToLower(address)] = true
		}
		seen := map[string]bool{}
		for _, address := range append([]string(conversation.EmailCC), added...) {
			key := strings.ToLower(strings.TrimSpace(address))
			if key == "" || drop[key] || seen[key] {
				continue
			}
			seen[key] = true
			stored = append(stored, key)
		}
		return tx.Model(&model.SupportConversation{}).Where("workspace_id = ? AND id = ?", workspaceID, conversationID).
			Update("email_cc", model.DocsStringArray(stored)).Error
	})
	return stored, err
}
