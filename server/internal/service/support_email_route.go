package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *SupportInboxService) inboundEmailDomain() string {
	if s != nil && s.emailFallbackService != nil {
		return s.emailFallbackService.InboundDomain()
	}
	return "replies.helpin.ai"
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
