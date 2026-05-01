package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const sharedSupportEmailRouteLocalPart = "inbox"

func (s *SupportInboxService) inboundEmailDomain() string {
	if s != nil && strings.TrimSpace(s.routeDomain) != "" {
		return strings.TrimSpace(s.routeDomain)
	}
	if s != nil && s.emailFallbackService != nil {
		return s.emailFallbackService.InboundDomain()
	}
	return "replies.helpin.email"
}

func (s *SupportInboxService) ListEmailRoutes(ctx context.Context, workspaceID string) ([]model.SupportEmailRoute, error) {
	if s.emailRouteRepo == nil {
		return nil, fmt.Errorf("support email route repository is unavailable")
	}
	return s.emailRouteRepo.ListByWorkspace(ctx, workspaceID)
}

func (s *SupportInboxService) CreateEmailRoute(ctx context.Context, workspaceID string, req model.CreateSupportEmailRouteRequest, actorID string) (*model.SupportEmailRoute, error) {
	if s.emailRouteRepo == nil {
		return nil, fmt.Errorf("support email route repository is unavailable")
	}

	mailboxID, mailbox, err := s.sanitizeMailboxSelection(ctx, workspaceID, req.MailboxID)
	if err != nil {
		return nil, err
	}

	existing, err := s.emailRouteRepo.GetActiveByMailbox(ctx, workspaceID, mailboxID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	sourceAddress := normalizeSupportEmailRouteSource(req.SourceAddress)
	routeKey, err := s.generateSupportEmailRouteKey(ctx)
	if err != nil {
		return nil, err
	}
	inboundAddress := fmt.Sprintf("%s@%s", routeKey, s.inboundEmailDomain())
	if brandedAddress, brandedErr := s.buildSupportEmailRouteAddress(ctx, workspaceID, mailbox); brandedErr == nil && brandedAddress != "" {
		inboundAddress = brandedAddress
	}

	route := &model.SupportEmailRoute{
		WorkspaceID:    workspaceID,
		MailboxID:      mailboxID,
		RouteKey:       routeKey,
		InboundAddress: inboundAddress,
		SourceAddress:  sourceAddress,
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    actorID,
	}

	if err := s.emailRouteRepo.Create(ctx, route); err != nil {
		return nil, err
	}
	if mailbox != nil {
		route.MailboxName = &mailbox.Name
		route.MailboxHandle = &mailbox.Handle
		route.MailboxIcon = &mailbox.Icon
	}
	return route, nil
}

func (s *SupportInboxService) DisableEmailRoute(ctx context.Context, workspaceID, routeID string) error {
	if s.emailRouteRepo == nil {
		return fmt.Errorf("support email route repository is unavailable")
	}
	route, err := s.emailRouteRepo.GetByID(ctx, workspaceID, routeID)
	if err != nil {
		return err
	}
	if route == nil {
		return fmt.Errorf("email route not found")
	}
	return s.emailRouteRepo.Disable(ctx, workspaceID, routeID)
}

func normalizeSupportEmailRouteSource(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *SupportInboxService) generateSupportEmailRouteKey(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		token, err := generateSecureToken(8)
		if err != nil {
			return "", fmt.Errorf("generate support email route key: %w", err)
		}
		routeKey := "route-" + token
		if s.emailRouteRepo == nil {
			return routeKey, nil
		}
		existing, lookupErr := s.emailRouteRepo.GetByRouteKey(ctx, routeKey)
		if lookupErr != nil {
			return "", lookupErr
		}
		if existing == nil {
			return routeKey, nil
		}
	}
	return "", fmt.Errorf("failed to allocate a unique support email route key")
}

// BuildOutboundFromAddress returns the mailbox-branded sender address used for
// outbound support emails, formatted as "<handle>@<workspace_slug>.<route_domain>".
// The workspace slug and the route domain must both be configured; otherwise an
// error is returned so callers can fall back to a legacy global sender.
func (s *SupportInboxService) BuildOutboundFromAddress(ctx context.Context, workspaceID string, mailboxID *string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("support inbox service is unavailable")
	}
	if address, ok := s.activeCustomSenderAddress(ctx, workspaceID); ok {
		return address, nil
	}
	mailbox := s.loadMailboxFromConversation(ctx, workspaceID, mailboxID)
	return s.buildSupportEmailRouteAddress(ctx, workspaceID, mailbox)
}

func (s *SupportInboxService) activeCustomSenderAddress(ctx context.Context, workspaceID string) (string, bool) {
	if s == nil || s.emailSenderDomainRepo == nil {
		return "", false
	}
	senderDomain, err := s.emailSenderDomainRepo.GetActiveVerifiedByWorkspace(ctx, workspaceID)
	if err != nil || senderDomain == nil {
		return "", false
	}
	localPart := strings.TrimSpace(senderDomain.FromLocalPart)
	domain := strings.TrimSpace(senderDomain.Domain)
	if localPart == "" || domain == "" {
		return "", false
	}
	return fmt.Sprintf("%s@%s", localPart, domain), true
}

func (s *SupportInboxService) buildSupportEmailRouteAddress(ctx context.Context, workspaceID string, mailbox *model.SupportMailbox) (string, error) {
	namespace, err := s.supportEmailRouteNamespace(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	domain := strings.TrimSpace(s.inboundEmailDomain())
	if namespace == "" || domain == "" {
		return "", fmt.Errorf("support email route namespace or domain is empty")
	}
	return fmt.Sprintf("%s@%s.%s", supportEmailRouteLocalPart(mailbox), namespace, domain), nil
}

func (s *SupportInboxService) supportEmailRouteNamespace(ctx context.Context, workspaceID string) (string, error) {
	if s == nil || s.workspaceRepo == nil {
		return "", fmt.Errorf("workspace repository is unavailable")
	}
	workspace, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("load workspace for support email route namespace: %w", err)
	}
	if workspace == nil {
		return "", fmt.Errorf("workspace not found")
	}
	namespace := normalizeSupportMailboxHandle(workspace.Slug)
	if namespace == "" {
		return "", fmt.Errorf("workspace slug is required for support email namespace")
	}
	return namespace, nil
}

func supportEmailRouteLocalPart(mailbox *model.SupportMailbox) string {
	if mailbox == nil {
		return sharedSupportEmailRouteLocalPart
	}
	handle := normalizeSupportMailboxHandle(mailbox.Handle)
	if handle == "" {
		return sharedSupportEmailRouteLocalPart
	}
	return handle
}
