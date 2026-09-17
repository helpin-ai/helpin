package service

import (
	"context"
	"fmt"
	"github.com/helpin-ai/helpin/server/internal/deployment"
	"html"
	"net/mail"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const sharedSupportEmailRouteLocalPart = "inbox"

const supportEmailRouteVerificationSubject = "Helpin forwarding test"

const (
	supportOutboundSenderSourceMailboxDefault   = "mailbox_default_sender"
	supportOutboundSenderSourceWorkspaceDefault = "workspace_default_sender"
	supportOutboundSenderSourceActiveDomain     = "active_sender_domain"
	supportOutboundSenderSourceGeneratedRoute   = "generated_route"
)

type SupportOutboundFromAddressResult struct {
	Email     string
	Source    string
	SenderID  string
	MailboxID *string
}

func (s *SupportInboxService) inboundEmailDomain() string {
	if s != nil && strings.TrimSpace(s.routeDomain) != "" {
		return strings.TrimSpace(s.routeDomain)
	}
	if s != nil && s.emailFallbackService != nil {
		return s.emailFallbackService.InboundDomain()
	}
	return deployment.DefaultReplyDomain
}

func (s *SupportInboxService) ListEmailRoutes(ctx context.Context, workspaceID string) ([]model.SupportEmailRoute, error) {
	if s.emailRouteRepo == nil {
		return nil, fmt.Errorf("support email route repository is unavailable")
	}
	return s.emailRouteRepo.ListByWorkspace(ctx, workspaceID)
}

func (s *SupportInboxService) CreateEmailRoute(ctx context.Context, workspaceID string, req model.CreateSupportEmailRouteRequest, actorID string) (*model.SupportEmailRoute, error) {
	if s.inboundEmailDomain() == "" {
		return nil, fmt.Errorf("support email is not configured: set SUPPORT_EMAIL_ROUTE_DOMAIN and a support email provider")
	}
	if s.emailRouteRepo == nil {
		return nil, fmt.Errorf("support email route repository is unavailable")
	}

	mailboxID, mailbox, err := s.sanitizeMailboxSelection(ctx, workspaceID, req.MailboxID)
	if err != nil {
		return nil, err
	}

	existing, err := s.emailRouteRepo.GetByMailbox(ctx, workspaceID, mailboxID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if !existing.Active {
			if err := s.emailRouteRepo.Reactivate(ctx, workspaceID, existing.ID, normalizeSupportEmailRouteSource(req.SourceAddress)); err != nil {
				return nil, fmt.Errorf("restore support email route: %w", err)
			}
			return s.emailRouteRepo.GetByID(ctx, workspaceID, existing.ID)
		}
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

// SendEmailRouteTest sends a tagged message through the customer's source address so
// Helpin can verify that forwarding works end to end when the message returns.
func (s *SupportInboxService) SendEmailRouteTest(ctx context.Context, workspaceID, routeID, sourceAddress string) (*model.SupportEmailRoute, error) {
	if s == nil || s.emailRouteRepo == nil {
		return nil, fmt.Errorf("support email routes are unavailable")
	}
	if s.emailFallbackService == nil || s.emailFallbackService.emailClient == nil {
		return nil, fmt.Errorf("support email delivery is unavailable")
	}
	route, err := s.emailRouteRepo.GetByID(ctx, workspaceID, routeID)
	if err != nil {
		return nil, err
	}
	if route == nil || !route.Active {
		return nil, fmt.Errorf("email forwarding route not found")
	}
	parsed, err := mail.ParseAddress(strings.TrimSpace(sourceAddress))
	if err != nil || strings.TrimSpace(parsed.Address) == "" {
		return nil, fmt.Errorf("enter a valid source email address")
	}
	sourceAddress = strings.ToLower(strings.TrimSpace(parsed.Address))
	if strings.EqualFold(sourceAddress, route.InboundAddress) {
		return nil, fmt.Errorf("source address must be different from the Helpin forwarding address")
	}

	token, err := generateSecureToken(12)
	if err != nil {
		return nil, fmt.Errorf("generate forwarding test token: %w", err)
	}
	now := time.Now().UTC()
	route.SourceAddress = &sourceAddress
	route.VerificationSentAt = &now
	route.ForwardingVerificationToken = token
	route.ForwardingLastError = nil
	if err := s.emailRouteRepo.Update(ctx, route); err != nil {
		return nil, err
	}

	subject := fmt.Sprintf("%s [%s]", supportEmailRouteVerificationSubject, token)
	textBody := "This message verifies that email sent to " + sourceAddress + " is forwarded into Helpin. No action is required."
	htmlBody := "<p>This message verifies that email sent to <strong>" + html.EscapeString(sourceAddress) + "</strong> is forwarded into Helpin.</p><p>No action is required.</p>"
	if err := s.emailFallbackService.emailClient.SendEmail(sourceAddress, subject, htmlBody, textBody); err != nil {
		message := err.Error()
		route.ForwardingLastError = &message
		if updateErr := s.emailRouteRepo.Update(ctx, route); updateErr != nil {
			return nil, updateErr
		}
		return nil, fmt.Errorf("send forwarding test: %w", err)
	}
	return route, nil
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
	result, err := s.ResolveOutboundFromAddress(ctx, workspaceID, mailboxID)
	if err != nil {
		return "", err
	}
	return result.Email, nil
}

func (s *SupportInboxService) ResolveOutboundFromAddress(ctx context.Context, workspaceID string, mailboxID *string) (SupportOutboundFromAddressResult, error) {
	if s == nil {
		return SupportOutboundFromAddressResult{}, fmt.Errorf("support inbox service is unavailable")
	}
	if result, ok := s.defaultEmailSenderAddress(ctx, workspaceID, mailboxID); ok {
		return result, nil
	}
	if address, ok := s.activeCustomSenderAddress(ctx, workspaceID); ok {
		return SupportOutboundFromAddressResult{Email: address, Source: supportOutboundSenderSourceActiveDomain, MailboxID: mailboxID}, nil
	}
	mailbox := s.loadMailboxFromConversation(ctx, workspaceID, mailboxID)
	address, err := s.buildSupportEmailRouteAddress(ctx, workspaceID, mailbox)
	if err != nil {
		return SupportOutboundFromAddressResult{}, err
	}
	return SupportOutboundFromAddressResult{Email: address, Source: supportOutboundSenderSourceGeneratedRoute, MailboxID: mailboxID}, nil
}

func (s *SupportInboxService) defaultEmailSenderAddress(ctx context.Context, workspaceID string, mailboxID *string) (SupportOutboundFromAddressResult, bool) {
	if s == nil || s.emailSenderRepo == nil {
		return SupportOutboundFromAddressResult{}, false
	}
	if sender, err := s.emailSenderRepo.GetMailboxDefaultVerified(ctx, workspaceID, mailboxID); err == nil && sender != nil {
		if address := strings.TrimSpace(sender.Email); address != "" {
			return SupportOutboundFromAddressResult{Email: address, Source: supportOutboundSenderSourceMailboxDefault, SenderID: sender.ID, MailboxID: sender.MailboxID}, true
		}
	}
	if sender, err := s.emailSenderRepo.GetWorkspaceDefaultVerified(ctx, workspaceID); err == nil && sender != nil {
		if address := strings.TrimSpace(sender.Email); address != "" {
			return SupportOutboundFromAddressResult{Email: address, Source: supportOutboundSenderSourceWorkspaceDefault, SenderID: sender.ID, MailboxID: sender.MailboxID}, true
		}
	}
	return SupportOutboundFromAddressResult{}, false
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
