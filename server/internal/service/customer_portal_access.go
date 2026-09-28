package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Portal access errors returned to support admins.
var (
	ErrPortalContactNotFound     = errors.New("contact not found")
	ErrPortalAccessInvalid       = errors.New("portal access must be allowed, blocked, or empty")
	ErrPortalCustomerNotEligible = errors.New("this customer does not have portal access")
	ErrPortalLinkNotSent         = errors.New("a confirmation email was sent recently; try again later")
)

// portalEligibility is the result of the portal access rule for one email.
type portalEligibility struct {
	Eligible  bool
	ContactID *string
}

// evaluatePortalEligibility applies the access rule to the contacts sharing
// one normalized email:
//   - any blocked contact denies access in either mode;
//   - approved-contacts mode needs exactly one allowed contact;
//   - any-verified-email mode allows the rest and binds a sole contact.
func evaluatePortalEligibility(mode string, contacts []repository.PortalContactAccess) portalEligibility {
	var allowed []string
	for _, contact := range contacts {
		switch derefString(contact.PortalAccess) {
		case model.PortalAccessBlocked:
			return portalEligibility{}
		case model.PortalAccessAllowed:
			allowed = append(allowed, contact.ID)
		}
	}
	if mode == model.SupportPortalAccessModeAnyVerifiedEmail {
		if len(contacts) == 1 {
			return portalEligibility{Eligible: true, ContactID: &contacts[0].ID}
		}
		return portalEligibility{Eligible: true}
	}
	if len(allowed) != 1 {
		return portalEligibility{}
	}
	return portalEligibility{Eligible: true, ContactID: &allowed[0]}
}

func (s *CustomerPortalService) eligibility(ctx context.Context, repo *repository.CustomerPortalRepository, ws *PortalWorkspace, email string) (portalEligibility, error) {
	contacts, err := repo.ContactsForEmail(ctx, ws.ID, email)
	if err != nil {
		return portalEligibility{}, err
	}
	return evaluatePortalEligibility(ws.Settings.EffectivePortalAccessMode(), contacts), nil
}

// sessionStillAuthorized rechecks eligibility for a signed-in identity. In
// approved-contacts mode the eligible contact must be the one this session
// was issued for; a different match requires a fresh sign-in. The session,
// not the shared per-email identity, carries that contact, so a later
// sign-in bound to another contact cannot revive an older session.
func (s *CustomerPortalService) sessionStillAuthorized(ctx context.Context, ws *PortalWorkspace, session *model.PortalSession, identity *model.SupportPortalIdentity) (bool, error) {
	result, err := s.eligibility(ctx, s.repo, ws, identity.Email)
	if err != nil || !result.Eligible {
		return false, err
	}
	if ws.Settings.EffectivePortalAccessMode() == model.SupportPortalAccessModeAnyVerifiedEmail {
		return true, nil
	}
	return session.CRMContactID != nil && result.ContactID != nil && *session.CRMContactID == *result.ContactID, nil
}

// PortalAccessSummary is the admin view of who may use the portal.
type PortalAccessSummary struct {
	AccessMode                       string   `json:"access_mode"`
	AllowedContacts                  int64    `json:"allowed_contacts"`
	Conflicts                        int64    `json:"conflicts"`
	ConflictEmails                   []string `json:"conflict_emails"`
	AnonymousIntakeDeliveryAvailable bool     `json:"anonymous_intake_delivery_available"`
}

// AccessSummary returns portal access counts for a workspace.
func (s *CustomerPortalService) AccessSummary(ctx context.Context, workspaceID string) (*PortalAccessSummary, error) {
	_, settings, err := s.inbox.GetInstallation(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	mode := model.DefaultSupportInboxSettings().EffectivePortalAccessMode()
	if settings != nil {
		mode = settings.EffectivePortalAccessMode()
	}
	counts, err := s.repo.AccessSummary(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return &PortalAccessSummary{
		AccessMode:                       mode,
		AllowedContacts:                  counts.AllowedContacts,
		Conflicts:                        counts.Conflicts,
		ConflictEmails:                   append([]string{}, counts.ConflictEmails...),
		AnonymousIntakeDeliveryAvailable: s.inbox.PortalIntakeEmailAvailable(),
	}, nil
}

// PortalContactAccess is a contact's portal decision for support admins.
type PortalContactAccess struct {
	PortalAccess        *string `json:"portal_access"`
	SharedEmailContacts int64   `json:"shared_email_contacts"`
}

// ContactAccess returns a contact's portal decision and how many other
// contacts share its email.
func (s *CustomerPortalService) ContactAccess(ctx context.Context, workspaceID, contactID string) (*PortalContactAccess, error) {
	detail, err := s.repo.ContactPortalAccess(ctx, workspaceID, contactID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPortalContactNotFound
	}
	if err != nil {
		return nil, err
	}
	return &PortalContactAccess{PortalAccess: detail.PortalAccess, SharedEmailContacts: detail.SharedEmailContacts}, nil
}

// SetContactAccess records a support admin's portal decision for a contact.
// Existing sessions are rechecked on their next request, so a block or a
// removed approval takes effect immediately.
func (s *CustomerPortalService) SetContactAccess(ctx context.Context, workspaceID, contactID, actorUserID string, access *string) (*PortalContactAccess, error) {
	if access != nil {
		value := strings.TrimSpace(*access)
		switch value {
		case "":
			access = nil
		case model.PortalAccessAllowed, model.PortalAccessBlocked:
			access = &value
		default:
			return nil, ErrPortalAccessInvalid
		}
	}
	if err := s.repo.SetContactPortalAccess(ctx, workspaceID, contactID, access); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPortalContactNotFound
		}
		return nil, err
	}
	slog.InfoContext(ctx, "portal contact access changed", "workspace_id", workspaceID, "contact_id", contactID, "user_id", actorUserID, "portal_access", derefString(access))
	return s.ContactAccess(ctx, workspaceID, contactID)
}

// ResendIntakeConfirmation emails a fresh confirmation link for one hidden
// anonymous request, for use after its submitter has been approved. Only that
// request is published when the link is used.
func (s *CustomerPortalService) ResendIntakeConfirmation(ctx context.Context, workspaceID, conversationID, actorUserID string) error {
	ws, err := s.resolveWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	conversation, err := s.repo.FindHiddenIntakeRequest(ctx, workspaceID, conversationID)
	if err != nil {
		return err
	}
	if conversation == nil {
		return ErrPortalRequestNotFound
	}
	address, ok := normalizePortalEmail(derefString(conversation.CustomerEmail))
	if !ok {
		return ErrPortalCustomerNotEligible
	}
	result, err := s.eligibility(ctx, s.repo, ws, address)
	if err != nil {
		return err
	}
	if !result.Eligible {
		return ErrPortalCustomerNotEligible
	}
	if !s.sendLink(ctx, ws, address, &conversation.ID) {
		return ErrPortalLinkNotSent
	}
	var actor *string
	if actorUserID != "" {
		actor = &actorUserID
	}
	if err := s.repo.CreateAuditEvent(ctx, &model.SupportPortalAuditEvent{
		ID: uuid.NewString(), WorkspaceID: workspaceID, ConversationID: conversation.ID,
		ActorType: model.SupportPortalActorUser, ActorUserID: actor,
		EventType: model.SupportPortalAuditConfirmationResent, Metadata: "{}", OccurredAt: time.Now().UTC(),
	}); err != nil {
		slog.ErrorContext(ctx, "portal confirmation audit failed", "workspace_id", workspaceID, "conversation_id", conversation.ID, "error", err)
	}
	return nil
}

func (s *CustomerPortalService) resolveWorkspaceByID(ctx context.Context, workspaceID string) (*PortalWorkspace, error) {
	ws, err := s.repo.WorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, ErrPortalUnavailable
	}
	return s.ResolveWorkspace(ctx, ws.Slug)
}
