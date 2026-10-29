package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// conversationCCContainsSQL is true when the conversation's Cc list holds
// the bound lowercase address.
func conversationCCContainsSQL(dialect string) string {
	if dialect == "postgres" {
		return "EXISTS (SELECT 1 FROM unnest(conv.email_cc) AS cc(address) WHERE lower(cc.address) = ?)"
	}
	// SQLite stores the quoted array literal as text.
	return "(',' || replace(lower(trim(COALESCE(conv.email_cc, ''), '{}')), '\"', '') || ',') LIKE '%,' || ? || ',%'"
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

// PortalColleagues returns the other members of the contact's portal
// companies, and those companies' email domains. Only explicit portal
// memberships count, as for company visibility.
func (r *CustomerPortalRepository) PortalColleagues(ctx context.Context, workspaceID, contactID string) ([]PortalColleague, []string, error) {
	var companies []struct {
		ID     string
		Domain *string
	}
	if err := r.db.WithContext(ctx).Table("crm_companies AS company").
		Where("company.workspace_id = ? AND company.portal_default_role IS NOT NULL", workspaceID).
		Where(portalMemberSQL("?", "company.id"), contactID).
		Select("company.id, company.domain").Scan(&companies).Error; err != nil {
		return nil, nil, err
	}
	if len(companies) == 0 {
		return []PortalColleague{}, nil, nil
	}
	ids := make([]string, 0, len(companies))
	var domains []string
	for _, company := range companies {
		ids = append(ids, company.ID)
		if company.Domain != nil {
			if domain := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(*company.Domain), "www.")); domain != "" {
				domains = append(domains, domain)
			}
		}
	}
	var colleagues []PortalColleague
	err := r.db.WithContext(ctx).Table("support_portal_company_members AS member").
		Joins("JOIN crm_contacts AS contact ON contact.id = member.crm_contact_id AND contact.workspace_id = member.workspace_id").
		Where("member.workspace_id = ? AND member.crm_company_id IN ? AND member.crm_contact_id <> ?", workspaceID, ids, contactID).
		Where("contact.email IS NOT NULL AND contact.email <> '' AND (contact.portal_access IS NULL OR contact.portal_access <> ?)", model.PortalAccessBlocked).
		Select("DISTINCT lower(contact.email) AS email, trim(COALESCE(contact.first_name, '') || ' ' || COALESCE(contact.last_name, '')) AS name").
		Order("email").Scan(&colleagues).Error
	if colleagues == nil {
		colleagues = []PortalColleague{}
	}
	return colleagues, domains, err
}

// CopyNoticeCounts counts copy notices sent to address about the
// conversation since addressSince, and by the workspace since
// workspaceSince.
func (r *CustomerPortalRepository) CopyNoticeCounts(ctx context.Context, workspaceID, conversationID, address string, addressSince, workspaceSince time.Time) (forAddress, forWorkspace int64, err error) {
	notices := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&model.SupportPortalAuditEvent{}).
			Where("workspace_id = ? AND event_type = ?", workspaceID, model.SupportPortalAuditCCNotified)
	}
	metadata, err := json.Marshal(map[string]string{"notified": address})
	if err != nil {
		return 0, 0, err
	}
	if err = notices().Where("conversation_id = ? AND metadata = ? AND occurred_at >= ?", conversationID, string(metadata), addressSince).Count(&forAddress).Error; err != nil {
		return 0, 0, err
	}
	err = notices().Where("occurred_at >= ?", workspaceSince).Count(&forWorkspace).Error
	return forAddress, forWorkspace, err
}
