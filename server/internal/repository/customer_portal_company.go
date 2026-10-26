package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// contactCompanyLinkSQL is true when the contact and company columns are
// associated. CRM associations are stored in either direction.
func contactCompanyLinkSQL(contactColumn, companyColumn string) string {
	return `EXISTS (SELECT 1 FROM crm_associations AS link WHERE link.workspace_id = conv.workspace_id AND (
		(link.from_object_type = 'contact' AND link.from_object_id = ` + contactColumn + ` AND link.to_object_type = 'company' AND link.to_object_id = ` + companyColumn + `)
		OR (link.from_object_type = 'company' AND link.from_object_id = ` + companyColumn + ` AND link.to_object_type = 'contact' AND link.to_object_id = ` + contactColumn + `)))`
}

// PortalCompanyMembership is a company whose requests a contact may see in the
// portal, with the contact's effective role there.
type PortalCompanyMembership struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// PortalCompanyMemberships lists the portal companies of an allowed contact:
// companies with a portal default role that the contact is associated with.
// Contacts without an explicit allowed decision have none, so automatically
// created contacts and associations never widen what a session can read.
func (r *CustomerPortalRepository) PortalCompanyMemberships(ctx context.Context, workspaceID, contactID string) ([]PortalCompanyMembership, error) {
	var memberships []PortalCompanyMembership
	err := r.db.WithContext(ctx).Table("crm_companies AS company").
		Joins("JOIN crm_contacts AS viewer ON viewer.workspace_id = company.workspace_id AND viewer.id = ?", contactID).
		Where("company.workspace_id = ? AND company.portal_default_role IS NOT NULL AND viewer.portal_access = ?", workspaceID, model.PortalAccessAllowed).
		Where(`EXISTS (SELECT 1 FROM crm_associations AS link WHERE link.workspace_id = company.workspace_id AND (
			(link.from_object_type = 'contact' AND link.from_object_id = viewer.id AND link.to_object_type = 'company' AND link.to_object_id = company.id)
			OR (link.from_object_type = 'company' AND link.from_object_id = company.id AND link.to_object_type = 'contact' AND link.to_object_id = viewer.id)))`).
		Select("company.id, company.name, COALESCE(viewer.portal_role, company.portal_default_role) AS role").
		Order("company.name, company.id").Scan(&memberships).Error
	return memberships, err
}

// companyRequests selects the published requests of companyIDs. A request
// belongs to a company only when its requester is a known contact of that
// company who is not blocked; a company matched from an email domain alone
// never shares a request. Erased conversations are never shared.
func (r *CustomerPortalRepository) companyRequests(ctx context.Context, workspaceID string, companyIDs []string) *gorm.DB {
	return r.db.WithContext(ctx).Table("support_portal_request_references AS refs").
		Joins("JOIN support_conversations AS conv ON conv.id = refs.conversation_id AND conv.workspace_id = refs.workspace_id").
		Joins("JOIN support_portal_identities AS identity ON identity.id = refs.portal_identity_id AND identity.workspace_id = refs.workspace_id").
		Joins("JOIN crm_contacts AS requester ON requester.workspace_id = conv.workspace_id AND requester.id = conv.crm_contact_id").
		Where("refs.workspace_id = ? AND conv.crm_company_id IN ?", workspaceID, companyIDs).
		Where("conv.anonymized_at IS NULL AND (requester.portal_access IS NULL OR requester.portal_access <> ?)", model.PortalAccessBlocked).
		Where(contactCompanyLinkSQL("requester.id", "conv.crm_company_id")).
		Where(portalVisibleConversation).
		Where(portalContinuityGuard, model.IdentityTrustVerified, model.IdentityTrustVerified)
}

// ListCompanyRequests returns the published requests of companyIDs, newest
// activity first, with who asked and for which company.
func (r *CustomerPortalRepository) ListCompanyRequests(ctx context.Context, workspaceID string, companyIDs []string, status string) ([]model.SupportPortalRequest, error) {
	if len(companyIDs) == 0 {
		return []model.SupportPortalRequest{}, nil
	}
	query := r.companyRequests(ctx, workspaceID, companyIDs).
		Joins("JOIN crm_companies AS company ON company.workspace_id = conv.workspace_id AND company.id = conv.crm_company_id")
	return r.listRequests(query, `, conv.crm_company_id, company.name AS company_name,
		NULLIF(trim(COALESCE(requester.first_name, '') || ' ' || COALESCE(requester.last_name, '')), '') AS requester_name,
		lower(conv.customer_email) AS requester_email`, status)
}

// FindCompanyRequest resolves a reference among the published requests of
// companyIDs, or nil.
func (r *CustomerPortalRepository) FindCompanyRequest(ctx context.Context, workspaceID string, companyIDs []string, reference string) (*model.SupportConversation, error) {
	if len(companyIDs) == 0 {
		return nil, nil
	}
	var conversation model.SupportConversation
	err := r.companyRequests(ctx, workspaceID, companyIDs).Select("conv.*").
		Where("refs.reference = ?", reference).Take(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// CompanyRequestReferences maps the published conversations of companyIDs to
// their public references.
func (r *CustomerPortalRepository) CompanyRequestReferences(ctx context.Context, workspaceID string, companyIDs []string) (map[string]string, error) {
	references := map[string]string{}
	if len(companyIDs) == 0 {
		return references, nil
	}
	var rows []struct {
		ConversationID string
		Reference      string
	}
	if err := r.companyRequests(ctx, workspaceID, companyIDs).Select("conv.id AS conversation_id, refs.reference").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		references[row.ConversationID] = row.Reference
	}
	return references, nil
}

// PublishCompanyRequests publishes the company's conversations from
// colleagues who have not signed in to the portal yet, so members see the
// whole account. Each conversation passes the same continuity rule as a
// customer's own sign-in (AssociateVerifiedEmail), is owned by its requester's
// portal identity, and keeps any explicit visibility decision.
func (r *CustomerPortalRepository) PublishCompanyRequests(ctx context.Context, workspaceID string, companyIDs []string) error {
	if len(companyIDs) == 0 {
		return nil
	}
	var rows []struct {
		ID    string
		Email string
	}
	if err := r.companyCandidateRequests(ctx, workspaceID).
		Where("conv.crm_company_id IN ? AND conv.customer_email IS NOT NULL AND trim(conv.customer_email) <> ''", companyIDs).
		Where("NOT EXISTS (SELECT 1 FROM support_portal_request_references AS refs WHERE refs.workspace_id = conv.workspace_id AND refs.conversation_id = conv.id)").
		Select("conv.id, lower(trim(conv.customer_email)) AS email").Scan(&rows).Error; err != nil {
		return err
	}
	byEmail := map[string][]string{}
	for _, row := range rows {
		byEmail[row.Email] = append(byEmail[row.Email], row.ID)
	}
	for email, ids := range byEmail {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			identity := model.SupportPortalIdentity{ID: uuid.NewString(), WorkspaceID: workspaceID, Email: email}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "workspace_id"}, {Name: "email"}}, DoNothing: true}).Create(&identity).Error; err != nil {
				return err
			}
			if err := tx.Where("workspace_id = ? AND email = ?", workspaceID, email).First(&identity).Error; err != nil {
				return err
			}
			return r.publishCustomerConversations(tx, workspaceID, email, identity.ID, ids)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// PortalCompanyContact is a company contact's portal state for support admins.
type PortalCompanyContact struct {
	ID                  string     `json:"id"`
	FirstName           string     `json:"first_name"`
	LastName            *string    `json:"last_name,omitempty"`
	Email               *string    `json:"email,omitempty"`
	PortalAccess        *string    `json:"portal_access"`
	PortalRole          *string    `json:"portal_role"`
	SharedEmailContacts int64      `json:"shared_email_contacts"`
	LastSignedInAt      *time.Time `json:"last_signed_in_at,omitempty"`
}

// PortalCompanyAccess is a company's portal settings and contacts.
type PortalCompanyAccess struct {
	DefaultRole *string                `json:"default_role"`
	Contacts    []PortalCompanyContact `json:"contacts"`
}

// companyContactsSQL selects the contacts associated with the company @company.
const companyContactsSQL = `SELECT contact.id FROM crm_contacts AS contact
	WHERE contact.workspace_id = @workspace AND EXISTS (SELECT 1 FROM crm_associations AS link WHERE link.workspace_id = contact.workspace_id AND (
		(link.from_object_type = 'contact' AND link.from_object_id = contact.id AND link.to_object_type = 'company' AND link.to_object_id = @company)
		OR (link.from_object_type = 'company' AND link.from_object_id = @company AND link.to_object_type = 'contact' AND link.to_object_id = contact.id)))`

// CompanyPortalAccess returns a company's default role and the portal state of
// each associated contact. It returns gorm.ErrRecordNotFound for an unknown
// company.
func (r *CustomerPortalRepository) CompanyPortalAccess(ctx context.Context, workspaceID, companyID string) (*PortalCompanyAccess, error) {
	db := r.db.WithContext(ctx)
	var company struct{ PortalDefaultRole *string }
	if err := db.Table("crm_companies").Select("portal_default_role").
		Where("workspace_id = ? AND id = ?", workspaceID, companyID).Take(&company).Error; err != nil {
		return nil, err
	}
	access := &PortalCompanyAccess{DefaultRole: company.PortalDefaultRole, Contacts: []PortalCompanyContact{}}
	args := map[string]any{"workspace": workspaceID, "company": companyID}
	err := db.Raw(`SELECT contact.id, contact.first_name, contact.last_name, contact.email, contact.portal_access, contact.portal_role,
			(SELECT count(*) FROM crm_contacts AS other WHERE other.workspace_id = contact.workspace_id AND other.id <> contact.id
				AND contact.email IS NOT NULL AND trim(contact.email) <> '' AND lower(trim(other.email)) = lower(trim(contact.email))) AS shared_email_contacts
		FROM crm_contacts AS contact WHERE contact.id IN (`+companyContactsSQL+`)
		ORDER BY lower(contact.first_name), lower(COALESCE(contact.last_name, '')), contact.id`, args).Scan(&access.Contacts).Error
	if err != nil || len(access.Contacts) == 0 {
		return access, err
	}
	ids := make([]string, 0, len(access.Contacts))
	for _, contact := range access.Contacts {
		ids = append(ids, contact.ID)
	}
	var sessions []struct {
		CRMContactID string
		CreatedAt    time.Time
	}
	if err := db.Model(&model.PortalSession{}).Select("crm_contact_id, created_at").
		Where("workspace_id = ? AND crm_contact_id IN ?", workspaceID, ids).Scan(&sessions).Error; err != nil {
		return nil, err
	}
	latest := map[string]time.Time{}
	for _, session := range sessions {
		if session.CreatedAt.After(latest[session.CRMContactID]) {
			latest[session.CRMContactID] = session.CreatedAt
		}
	}
	for i := range access.Contacts {
		if signedIn, ok := latest[access.Contacts[i].ID]; ok {
			access.Contacts[i].LastSignedInAt = &signedIn
		}
	}
	return access, nil
}

// SetCompanyPortalDefaultRole sets or clears (nil) a company's portal default
// role. It returns gorm.ErrRecordNotFound for an unknown company.
func (r *CustomerPortalRepository) SetCompanyPortalDefaultRole(ctx context.Context, workspaceID, companyID string, role *string) error {
	result := r.db.WithContext(ctx).Table("crm_companies").
		Where("workspace_id = ? AND id = ?", workspaceID, companyID).
		Updates(map[string]any{"portal_default_role": role, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetContactPortalRole sets or clears (nil) a contact's portal role. It
// returns gorm.ErrRecordNotFound for an unknown contact.
func (r *CustomerPortalRepository) SetContactPortalRole(ctx context.Context, workspaceID, contactID string, role *string) error {
	result := r.db.WithContext(ctx).Table("crm_contacts").
		Where("workspace_id = ? AND id = ?", workspaceID, contactID).
		Updates(map[string]any{"portal_role": role, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// AllowCompanyContacts allows portal access for the company's contacts that
// have no decision yet and an email no other contact uses. Blocked contacts
// and shared emails are left for an admin to decide one by one, because an
// allowed duplicate would lock every contact with that email out.
func (r *CustomerPortalRepository) AllowCompanyContacts(ctx context.Context, workspaceID, companyID string) (int64, error) {
	args := map[string]any{"workspace": workspaceID, "company": companyID, "allowed": model.PortalAccessAllowed, "now": time.Now().UTC()}
	result := r.db.WithContext(ctx).Exec(`UPDATE crm_contacts SET portal_access = @allowed, updated_at = @now
		WHERE workspace_id = @workspace AND portal_access IS NULL AND email IS NOT NULL AND trim(email) <> ''
		AND id IN (`+companyContactsSQL+`)
		AND NOT EXISTS (SELECT 1 FROM crm_contacts AS other WHERE other.workspace_id = crm_contacts.workspace_id
			AND other.id <> crm_contacts.id AND lower(trim(other.email)) = lower(trim(crm_contacts.email)))`, args)
	return result.RowsAffected, result.Error
}

// companyCandidateRequests selects company conversations that members could
// see: from an unblocked contact of the conversation's company and not hidden
// from the portal. Continuity checks can still hold some back until published.
func (r *CustomerPortalRepository) companyCandidateRequests(ctx context.Context, workspaceID string) *gorm.DB {
	return r.db.WithContext(ctx).Table("support_conversations AS conv").
		Joins("JOIN crm_contacts AS requester ON requester.workspace_id = conv.workspace_id AND requester.id = conv.crm_contact_id").
		Where("conv.workspace_id = ? AND conv.crm_company_id IS NOT NULL", workspaceID).
		Where("conv.anonymized_at IS NULL AND (requester.portal_access IS NULL OR requester.portal_access <> ?)", model.PortalAccessBlocked).
		Where(contactCompanyLinkSQL("requester.id", "conv.crm_company_id")).
		Where(portalEligibleConversation).
		Where("(conv.portal_visible = true OR conv.portal_visibility_changed_at IS NULL)")
}

// CompanyRequestCount counts the company's requests members could see.
func (r *CustomerPortalRepository) CompanyRequestCount(ctx context.Context, workspaceID, companyID string) (int64, error) {
	var count int64
	err := r.companyCandidateRequests(ctx, workspaceID).Where("conv.crm_company_id = ?", companyID).Count(&count).Error
	return count, err
}

// PortalCompanyAudience is who at a company can read a conversation in the
// portal besides its requester.
type PortalCompanyAudience struct {
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
	// Members counts allowed company members other than the requester.
	Members int64 `json:"members"`
}

// ConversationCompanyAudience returns the company members other than the
// requester who can read the conversation in the portal, or nil when it is
// not shared with a company.
func (r *CustomerPortalRepository) ConversationCompanyAudience(ctx context.Context, workspaceID, conversationID string) (*PortalCompanyAudience, error) {
	var shared struct {
		ID          string
		Name        string
		RequesterID string
	}
	err := r.companyCandidateRequests(ctx, workspaceID).
		Joins("JOIN crm_companies AS company ON company.workspace_id = conv.workspace_id AND company.id = conv.crm_company_id").
		Where("conv.id = ? AND company.portal_default_role IS NOT NULL", conversationID).
		Select("company.id, company.name, conv.crm_contact_id AS requester_id").Take(&shared).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var members int64
	err = r.db.WithContext(ctx).Raw(`SELECT count(*) FROM crm_contacts AS member JOIN crm_companies AS company ON company.workspace_id = member.workspace_id AND company.id = @company
		WHERE member.id IN (`+companyContactsSQL+`) AND member.id <> @requester AND member.portal_access = @allowed
		AND COALESCE(member.portal_role, company.portal_default_role) = @member`,
		map[string]any{"workspace": workspaceID, "company": shared.ID, "requester": shared.RequesterID, "allowed": model.PortalAccessAllowed, "member": model.PortalRoleMember}).
		Scan(&members).Error
	if err != nil {
		return nil, err
	}
	return &PortalCompanyAudience{CompanyID: shared.ID, CompanyName: shared.Name, Members: members}, nil
}

// RecentAuditEventExists reports whether the identity recorded eventType for
// the conversation since the given time.
func (r *CustomerPortalRepository) RecentAuditEventExists(ctx context.Context, workspaceID, conversationID, identityID, eventType string, since time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SupportPortalAuditEvent{}).
		Where("workspace_id = ? AND conversation_id = ? AND portal_identity_id = ? AND event_type = ? AND occurred_at > ?", workspaceID, conversationID, identityID, eventType, since).
		Limit(1).Count(&count).Error
	return count > 0, err
}

// ContactDisplayName returns a contact's full name, or "" when it has none.
func (r *CustomerPortalRepository) ContactDisplayName(ctx context.Context, workspaceID, contactID string) (string, error) {
	var names []string
	err := r.db.WithContext(ctx).Table("crm_contacts").
		Where("workspace_id = ? AND id = ?", workspaceID, contactID).
		Pluck("trim(COALESCE(first_name, '') || ' ' || COALESCE(last_name, ''))", &names).Error
	if err != nil || len(names) == 0 {
		return "", err
	}
	return names[0], nil
}

// PortalIdentityNames returns a display name for each portal identity: its
// contact's full name, else its own display name, else its email.
func (r *CustomerPortalRepository) PortalIdentityNames(ctx context.Context, workspaceID string, identityIDs []string) (map[string]string, error) {
	names := map[string]string{}
	if len(identityIDs) == 0 {
		return names, nil
	}
	var rows []struct {
		ID   string
		Name string
	}
	err := r.db.WithContext(ctx).Table("support_portal_identities AS identity").
		Joins("LEFT JOIN crm_contacts AS contact ON contact.workspace_id = identity.workspace_id AND contact.id = identity.crm_contact_id").
		Where("identity.workspace_id = ? AND identity.id IN ?", workspaceID, identityIDs).
		Select(`identity.id, COALESCE(NULLIF(trim(COALESCE(contact.first_name, '') || ' ' || COALESCE(contact.last_name, '')), ''),
			NULLIF(trim(COALESCE(identity.display_name, '')), ''), identity.email) AS name`).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}
