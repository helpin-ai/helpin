package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Company portal errors returned to customers and support admins.
var (
	ErrPortalCompanyNotFound = errors.New("company not found")
	ErrPortalRoleInvalid     = errors.New("portal role must be requester, member, or empty")
	ErrPortalScopeInvalid    = errors.New("unknown request scope")
)

// Portal request list scopes.
const (
	PortalScopeMine    = "mine"
	PortalScopeCompany = "company"
)

// companyViewAuditInterval limits company view audit events to one per
// colleague and request in this window; live updates refetch a request often.
const companyViewAuditInterval = time.Hour

// PortalRequestAccess is a request the signed-in customer may open: their
// own, or a colleague's through a company they are a member of.
type PortalRequestAccess struct {
	Conversation *model.SupportConversation
	Own          bool
}

// Companies returns the portal companies of the session's contact. A
// session without a bound contact has none.
func (s *CustomerPortalService) Companies(ctx context.Context, access *PortalAccess) ([]repository.PortalCompanyMembership, error) {
	if access.Session.CRMContactID == nil {
		return []repository.PortalCompanyMembership{}, nil
	}
	memberships, err := s.repo.PortalCompanyMemberships(ctx, access.Workspace.ID, *access.Session.CRMContactID)
	if err != nil {
		return nil, err
	}
	if memberships == nil {
		memberships = []repository.PortalCompanyMembership{}
	}
	return memberships, nil
}

// memberCompanyIDs returns the companies whose requests the session may see.
func (s *CustomerPortalService) memberCompanyIDs(ctx context.Context, access *PortalAccess) ([]string, error) {
	memberships, err := s.Companies(ctx, access)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, membership := range memberships {
		if membership.Role == model.PortalRoleMember {
			ids = append(ids, membership.ID)
		}
	}
	return ids, nil
}

// CompanyRequests lists the requests of the companies where the session is a
// member, optionally narrowed to one of them. A company the session cannot
// see yields no requests rather than an error, so ids cannot be probed.
func (s *CustomerPortalService) CompanyRequests(ctx context.Context, access *PortalAccess, companyID, status string) ([]model.SupportPortalRequest, error) {
	companyIDs, err := s.memberCompanyIDs(ctx, access)
	if err != nil {
		return nil, err
	}
	if companyID != "" {
		if !slices.Contains(companyIDs, companyID) {
			return []model.SupportPortalRequest{}, nil
		}
		companyIDs = []string{companyID}
	}
	// Publishing is idempotent and finds nothing once a company is caught up,
	// so it runs on every company list instead of waiting for a reconcile slot.
	if err := s.repo.PublishCompanyRequests(ctx, access.Workspace.ID, companyIDs); err != nil {
		slog.ErrorContext(ctx, "portal company request publish failed", "workspace_id", access.Workspace.ID, "portal_identity_id", access.Identity.ID, "error", err)
	}
	requests, err := s.repo.ListCompanyRequests(ctx, access.Workspace.ID, companyIDs, status)
	if err != nil {
		return nil, err
	}
	for i := range requests {
		// Read state belongs to the requester; colleagues get no unread marks.
		if requests[i].OwnerIdentityID != access.Identity.ID {
			requests[i].Unread = false
		}
	}
	return requests, nil
}

// findRequest resolves a reference the session may open: its own request
// first, then a request of a company where it is a member. It returns nil
// when neither applies.
func (s *CustomerPortalService) findRequest(ctx context.Context, access *PortalAccess, reference string) (*PortalRequestAccess, error) {
	conv, err := s.repo.FindRequest(ctx, access.Workspace.ID, access.Identity.ID, reference)
	if err != nil {
		return nil, err
	}
	if conv != nil {
		return &PortalRequestAccess{Conversation: conv, Own: true}, nil
	}
	companyIDs, err := s.memberCompanyIDs(ctx, access)
	if err != nil {
		return nil, err
	}
	conv, err = s.repo.FindCompanyRequest(ctx, access.Workspace.ID, companyIDs, reference)
	if err != nil || conv == nil {
		return nil, err
	}
	return &PortalRequestAccess{Conversation: conv}, nil
}

// auditCompanyView records a colleague opening a request through company
// visibility, at most once per companyViewAuditInterval.
func (s *CustomerPortalService) auditCompanyView(ctx context.Context, access *PortalAccess, conv *model.SupportConversation) {
	now := time.Now().UTC()
	recent, err := s.repo.RecentAuditEventExists(ctx, access.Workspace.ID, conv.ID, access.Identity.ID, model.SupportPortalAuditCompanyRequestViewed, now.Add(-companyViewAuditInterval))
	if err == nil && !recent {
		identityID := access.Identity.ID
		err = s.repo.CreateAuditEvent(ctx, &model.SupportPortalAuditEvent{
			ID: uuid.NewString(), WorkspaceID: access.Workspace.ID, ConversationID: conv.ID,
			PortalIdentityID: &identityID, ActorType: model.SupportPortalActorCustomer,
			EventType: model.SupportPortalAuditCompanyRequestViewed, Metadata: `{"via":"company"}`, OccurredAt: now,
		})
	}
	if err != nil {
		slog.WarnContext(ctx, "portal company view audit failed", "workspace_id", access.Workspace.ID, "conversation_id", conv.ID, "error", err)
	}
}

// PortalCompanyAccess is a company's portal settings for support admins.
type PortalCompanyAccess struct {
	repository.PortalCompanyAccess
	// AccessMode is the workspace portal access mode; company visibility
	// needs contacts that are explicitly allowed in either mode.
	AccessMode    string `json:"access_mode"`
	PortalEnabled bool   `json:"portal_enabled"`
	// RequestCount is how many of the company's requests members could see.
	RequestCount int64 `json:"request_count"`
}

// CompanyAccess returns a company's portal settings and contacts.
func (s *CustomerPortalService) CompanyAccess(ctx context.Context, workspaceID, companyID string) (*PortalCompanyAccess, error) {
	access, err := s.repo.CompanyPortalAccess(ctx, workspaceID, companyID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPortalCompanyNotFound
	}
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CompanyRequestCount(ctx, workspaceID, companyID)
	if err != nil {
		return nil, err
	}
	result := &PortalCompanyAccess{PortalCompanyAccess: *access, RequestCount: count, AccessMode: model.DefaultSupportInboxSettings().EffectivePortalAccessMode()}
	_, settings, err := s.inbox.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if settings != nil {
		result.AccessMode, result.PortalEnabled = settings.EffectivePortalAccessMode(), settings.PortalEnabled
	}
	return result, nil
}

// SetCompanyDefaultRole sets or clears a company's portal default role and
// returns the refreshed settings. Sessions pick the change up on their next
// request.
func (s *CustomerPortalService) SetCompanyDefaultRole(ctx context.Context, workspaceID, companyID, actorUserID string, role *string) (*PortalCompanyAccess, error) {
	role, err := normalizePortalRole(role)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetCompanyPortalDefaultRole(ctx, workspaceID, companyID, role); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPortalCompanyNotFound
		}
		return nil, err
	}
	slog.InfoContext(ctx, "portal company default role changed", "workspace_id", workspaceID, "company_id", companyID, "user_id", actorUserID, "portal_role", derefString(role))
	return s.CompanyAccess(ctx, workspaceID, companyID)
}

// SetContactRole sets or clears a contact's portal role and returns the
// contact's refreshed portal decision.
func (s *CustomerPortalService) SetContactRole(ctx context.Context, workspaceID, contactID, actorUserID string, role *string) (*PortalContactAccess, error) {
	role, err := normalizePortalRole(role)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetContactPortalRole(ctx, workspaceID, contactID, role); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPortalContactNotFound
		}
		return nil, err
	}
	slog.InfoContext(ctx, "portal contact role changed", "workspace_id", workspaceID, "contact_id", contactID, "user_id", actorUserID, "portal_role", derefString(role))
	return s.ContactAccess(ctx, workspaceID, contactID)
}

// AllowCompanyContacts allows every company contact that has no portal
// decision and an email of its own, and returns the refreshed settings with
// how many contacts changed.
func (s *CustomerPortalService) AllowCompanyContacts(ctx context.Context, workspaceID, companyID, actorUserID string) (*PortalCompanyAccess, int64, error) {
	if _, err := s.CompanyAccess(ctx, workspaceID, companyID); err != nil {
		return nil, 0, err
	}
	allowed, err := s.repo.AllowCompanyContacts(ctx, workspaceID, companyID)
	if err != nil {
		return nil, 0, err
	}
	slog.InfoContext(ctx, "portal company contacts allowed", "workspace_id", workspaceID, "company_id", companyID, "user_id", actorUserID, "allowed", allowed)
	access, err := s.CompanyAccess(ctx, workspaceID, companyID)
	return access, allowed, err
}

// normalizePortalRole trims a role, maps "" to nil, and rejects unknown roles.
func normalizePortalRole(role *string) (*string, error) {
	if role == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*role)
	if value == "" {
		return nil, nil
	}
	if !model.ValidPortalRole(value) {
		return nil, ErrPortalRoleInvalid
	}
	return &value, nil
}

// withSupportMessageMetadataValue sets one key in a message's JSON metadata.
// Unreadable metadata is replaced rather than dropping the value.
func withSupportMessageMetadataValue(metadata, key string, value any) string {
	values := map[string]any{}
	if strings.TrimSpace(metadata) != "" {
		if err := json.Unmarshal([]byte(metadata), &values); err != nil || values == nil {
			values = map[string]any{}
		}
	}
	values[key] = value
	encoded, err := json.Marshal(values)
	if err != nil {
		return metadata
	}
	return string(encoded)
}

// ConversationCompanyAudience tells a teammate who can read a conversation
// in the portal through company visibility, or nil when nobody besides its
// requester can. It enforces the teammate's mailbox access.
func (s *CustomerPortalService) ConversationCompanyAudience(ctx context.Context, workspaceID, conversationID string) (*repository.PortalCompanyAudience, error) {
	conv, err := s.inbox.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, ErrPortalRequestNotFound
	}
	return s.repo.ConversationCompanyAudience(ctx, workspaceID, conv.ID)
}
