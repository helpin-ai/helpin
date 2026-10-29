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
	ErrPortalRoleInvalid     = errors.New("portal role must be requester, member or admin (admin only for people), or empty")
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
	// Copied is true when the request is visible only because it copies
	// the customer.
	Copied bool
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
		if model.PortalRoleSeesCompany(membership.Role) {
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
	s.attachReplyExpectedBy(ctx, &access.Workspace, requests)
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
	if err != nil {
		return nil, err
	}
	if conv != nil {
		return &PortalRequestAccess{Conversation: conv}, nil
	}
	// Copied requests show only while requesters may copy people.
	if access.Workspace.Settings.PortalCCMode == "" {
		return nil, nil
	}
	conv, err = s.repo.FindCopiedRequest(ctx, access.Workspace.ID, access.Identity.ID, access.Identity.Email, reference)
	if err != nil || conv == nil {
		return nil, err
	}
	return &PortalRequestAccess{Conversation: conv, Copied: true}, nil
}

// auditCompanyView records someone other than the requester opening a
// request, through company visibility or a copy, at most once per
// companyViewAuditInterval.
func (s *CustomerPortalService) auditCompanyView(ctx context.Context, access *PortalAccess, conv *model.SupportConversation, copied bool) {
	now := time.Now().UTC()
	eventType, metadata := model.SupportPortalAuditCompanyRequestViewed, `{"via":"company"}`
	if copied {
		eventType, metadata = model.SupportPortalAuditCopiedRequestViewed, `{"via":"cc"}`
	}
	recent, err := s.repo.RecentAuditEventExists(ctx, access.Workspace.ID, conv.ID, access.Identity.ID, eventType, now.Add(-companyViewAuditInterval))
	if err == nil && !recent {
		identityID := access.Identity.ID
		err = s.repo.CreateAuditEvent(ctx, &model.SupportPortalAuditEvent{
			ID: uuid.NewString(), WorkspaceID: access.Workspace.ID, ConversationID: conv.ID,
			PortalIdentityID: &identityID, ActorType: model.SupportPortalActorCustomer,
			EventType: eventType, Metadata: metadata, OccurredAt: now,
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
	// AdminInvites lets the company's portal admins invite colleagues;
	// InviteAnyDomain lifts the company domain rule. Activity lists recent
	// changes company admins made to the company's people.
	AdminInvites    bool                              `json:"admin_invites"`
	InviteAnyDomain bool                              `json:"invite_any_domain"`
	Activity        []model.SupportPortalCompanyEvent `json:"activity"`
}

// CompanyAccess returns a company's portal settings and contacts.
func (s *CustomerPortalService) CompanyAccess(ctx context.Context, workspaceID, companyID string) (*PortalCompanyAccess, error) {
	if !validPortalID(companyID) {
		return nil, ErrPortalCompanyNotFound
	}
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
	team, activity, err := s.CompanyTeamActivity(ctx, workspaceID, companyID)
	if err != nil {
		return nil, err
	}
	result.AdminInvites, result.InviteAnyDomain, result.Activity = team.AdminInvites, team.InviteAnyDomain, activity
	return result, nil
}

// SetCompanyDefaultRole sets or clears a company's portal default role and
// returns the refreshed settings. Sessions pick the change up on their next
// request.
func (s *CustomerPortalService) SetCompanyDefaultRole(ctx context.Context, workspaceID, companyID, actorUserID string, role *string) (*PortalCompanyAccess, error) {
	if !validPortalID(companyID) {
		return nil, ErrPortalCompanyNotFound
	}
	role, err := normalizePortalRole(role)
	if err != nil {
		return nil, err
	}
	// Admin is given to people, never to a whole company.
	if role != nil && *role == model.PortalRoleAdmin {
		return nil, ErrPortalRoleInvalid
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
	if !validPortalID(contactID) {
		return nil, ErrPortalContactNotFound
	}
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

// SetCompanyMember gives a contact company access, or removes it, and
// returns the refreshed company settings. Giving access also allows sign-in
// for a contact without a portal decision; a blocked contact stays blocked.
func (s *CustomerPortalService) SetCompanyMember(ctx context.Context, workspaceID, companyID, contactID, actorUserID string, member bool) (*PortalCompanyAccess, error) {
	if !validPortalID(companyID) {
		return nil, ErrPortalCompanyNotFound
	}
	if !validPortalID(contactID) {
		return nil, ErrPortalContactNotFound
	}
	if err := s.repo.SetCompanyMember(ctx, workspaceID, companyID, contactID, actorUserID, member); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPortalCompanyNotFound
		}
		return nil, err
	}
	slog.InfoContext(ctx, "portal company membership changed", "workspace_id", workspaceID, "company_id", companyID, "contact_id", contactID, "user_id", actorUserID, "member", member)
	return s.CompanyAccess(ctx, workspaceID, companyID)
}

// AddCompanyDomainMembers gives company access to every CRM contact of the
// company whose email is on the company's domain and used by no other
// contact, and returns the refreshed settings with how many were added.
func (s *CustomerPortalService) AddCompanyDomainMembers(ctx context.Context, workspaceID, companyID, actorUserID string) (*PortalCompanyAccess, int, error) {
	if _, err := s.CompanyAccess(ctx, workspaceID, companyID); err != nil {
		return nil, 0, err
	}
	added, err := s.repo.AddCompanyDomainMembers(ctx, workspaceID, companyID, actorUserID)
	if err != nil {
		return nil, 0, err
	}
	slog.InfoContext(ctx, "portal company domain members added", "workspace_id", workspaceID, "company_id", companyID, "user_id", actorUserID, "added", added)
	access, err := s.CompanyAccess(ctx, workspaceID, companyID)
	return access, added, err
}

// validPortalID reports whether an id from a URL can name a row; anything
// else is simply not found.
func validPortalID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
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

// PortalConversationContext is what a teammate should know about a
// conversation's portal side before replying.
type PortalConversationContext struct {
	// Company is who besides the requester reads it in the portal, or nil.
	Company *repository.PortalCompanyAudience `json:"company"`
	// Intake is what the customer saw while writing the request, or nil.
	Intake *PortalIntakeContext `json:"intake"`
}

// PortalIntakeContext lists the help center articles a customer opened
// before writing in, so teammates don't send them again.
type PortalIntakeContext struct {
	OpenedArticles []PortalArticleSuggestion `json:"opened_articles"`
	SuggestedCount int                       `json:"suggested_count"`
	// Assistant is the help assistant conversation the customer had before
	// writing in, when the assistant handed them off.
	Assistant []HelpAssistantTranscriptTurn `json:"assistant,omitempty"`
}

// ConversationPortalContext returns the portal context of a conversation the
// teammate can open: its company audience and the requester's intake.
func (s *CustomerPortalService) ConversationPortalContext(ctx context.Context, workspaceID, conversationID string) (*PortalConversationContext, error) {
	conv, err := s.inbox.loadConversationAccessible(ctx, workspaceID, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, ErrPortalRequestNotFound
	}
	audience, err := s.repo.ConversationCompanyAudience(ctx, workspaceID, conv.ID)
	if err != nil {
		return nil, err
	}
	result := &PortalConversationContext{Company: audience}
	intake, err := s.repo.ConversationPortalIntake(ctx, workspaceID, conv.ID)
	if err != nil || intake == nil {
		return result, err
	}
	opened, err := s.portalArticles(ctx, workspaceID, intake.OpenedArticleIDs)
	if err != nil {
		return nil, err
	}
	result.Intake = &PortalIntakeContext{OpenedArticles: opened, SuggestedCount: len(intake.SuggestedArticleIDs)}
	if intake.AssistantSessionID != "" && s.assistant != nil {
		transcript, err := s.assistant.Transcript(ctx, workspaceID, intake.AssistantSessionID)
		if err != nil {
			return nil, err
		}
		result.Intake.Assistant = transcript
	}
	return result, nil
}

// portalArticles resolves article ids to published articles with their
// public addresses, in the given order; unpublished articles are skipped.
func (s *CustomerPortalService) portalArticles(ctx context.Context, workspaceID string, ids []string) ([]PortalArticleSuggestion, error) {
	articles := []PortalArticleSuggestion{}
	if len(ids) == 0 || s.knowledge == nil || s.inbox == nil || s.inbox.docsHelpcenterRepo == nil {
		return articles, nil
	}
	cfg, err := s.inbox.docsHelpcenterRepo.GetConfig(ctx, workspaceID)
	if err != nil || cfg == nil {
		return articles, err
	}
	refs, err := s.knowledge.PublicArticles(ctx, workspaceID, defaultHelpcenterLocale(cfg), ids)
	if err != nil {
		return nil, err
	}
	urls, err := supportPublicArticleURLs(ctx, s.inbox.docsHelpcenterRepo, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	// Articles for signed-in customers have no public address; teammates
	// open them in Docs.
	private, err := s.inbox.docsHelpcenterRepo.ListCustomerHelpArticles(ctx, workspaceID, defaultHelpcenterLocale(cfg), "")
	if err != nil {
		return nil, err
	}
	privateByID := make(map[string]repository.CustomerHelpArticle, len(private))
	for _, article := range private {
		privateByID[article.DocumentID] = article
	}
	for _, id := range ids {
		if ref, ok := refs[id]; ok && urls[id] != "" {
			articles = append(articles, PortalArticleSuggestion{ID: id, Title: ref.Title, SpaceName: ref.SpaceName, URL: urls[id]})
			continue
		}
		if article, ok := privateByID[id]; ok {
			articles = append(articles, PortalArticleSuggestion{ID: id, Title: article.Title, SpaceName: article.SpaceName, PortalArticleID: article.PublicID})
		}
	}
	return articles, nil
}
