package service

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	supportEmailSenderDefaultScopeNone      = "none"
	supportEmailSenderDefaultScopeWorkspace = "workspace"
	supportEmailSenderDefaultScopeMailbox   = "mailbox"
	supportEmailSenderForwardingNotStarted  = "not_started"
	supportEmailSenderForwardingPending     = "pending"
	supportEmailSenderForwardingVerified    = "verified"
	supportEmailSenderForwardingFailed      = "failed"
)

func (s *SupportInboxService) ListEmailSenders(ctx context.Context, workspaceID string) ([]model.SupportEmailSender, error) {
	if s.emailSenderRepo == nil {
		return nil, fmt.Errorf("support email sender repository is unavailable")
	}
	senders, err := s.emailSenderRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range senders {
		if err := s.ensureEmailSenderForwardingVerification(ctx, &senders[i]); err != nil {
			return nil, err
		}
	}
	return senders, nil
}

func (s *SupportInboxService) CreateEmailSender(ctx context.Context, workspaceID string, req model.CreateSupportEmailSenderRequest, actorID string) (*model.SupportEmailSender, error) {
	if s.emailSenderRepo == nil {
		return nil, fmt.Errorf("support email sender repository is unavailable")
	}
	if s.postmarkDomainClient == nil {
		return nil, fmt.Errorf("postmark account token is not configured")
	}
	if actorID = strings.TrimSpace(actorID); actorID == "" {
		return nil, fmt.Errorf("actor is required")
	}

	emailAddress, localPart, domainName, err := normalizeSupportSenderEmail(req.Email)
	if err != nil {
		return nil, err
	}
	mailboxID, mailbox, err := s.sanitizeMailboxSelection(ctx, workspaceID, req.MailboxID)
	if err != nil {
		return nil, err
	}

	existing, err := s.emailSenderRepo.GetByEmail(ctx, workspaceID, emailAddress)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	postmarkDomain, err := s.postmarkDomainForSender(ctx, workspaceID, domainName)
	if err != nil {
		return nil, err
	}
	forwardingToken, forwardingAddress, err := s.generateEmailSenderForwardingVerification(ctx)
	if err != nil {
		return nil, err
	}
	sender := supportEmailSenderFromPostmark(postmarkDomain)
	sender.WorkspaceID = workspaceID
	sender.MailboxID = mailboxID
	sender.Email = emailAddress
	sender.LocalPart = localPart
	sender.Domain = domainName
	sender.DisplayName = strings.TrimSpace(req.DisplayName)
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	sender.ForwardingVerificationToken = forwardingToken
	sender.ForwardingAddress = forwardingAddress
	sender.DefaultScope = supportEmailSenderDefaultScopeNone
	sender.Active = false
	sender.CreatedByID = actorID
	if mailbox != nil {
		sender.MailboxName = &mailbox.Name
		sender.MailboxHandle = &mailbox.Handle
		sender.MailboxIcon = &mailbox.Icon
	}

	refreshSupportEmailSenderDMARC(ctx, sender)
	if err := s.emailSenderRepo.Create(ctx, sender); err != nil {
		return nil, err
	}
	return sender, nil
}

func (s *SupportInboxService) CompleteEmailSenderForwardingVerification(ctx context.Context, token string, payload model.PostmarkInboundPayload) (*model.SupportEmailSender, bool, error) {
	if s == nil || s.emailSenderRepo == nil {
		return nil, false, nil
	}
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "verify-")
	if token == "" {
		return nil, false, nil
	}
	sender, err := s.emailSenderRepo.GetByForwardingVerificationToken(ctx, token)
	if err != nil {
		return nil, false, err
	}
	if sender == nil {
		return nil, false, nil
	}

	now := time.Now().UTC()
	sender.ForwardingLastCheckedAt = &now
	if !inboundPayloadMentionsAddress(payload, sender.Email) {
		msg := "verification email did not reference sender address"
		sender.ForwardingStatus = supportEmailSenderForwardingFailed
		sender.ForwardingLastError = &msg
		if updateErr := s.emailSenderRepo.Update(ctx, sender); updateErr != nil {
			return nil, false, updateErr
		}
		return sender, false, nil
	}

	sender.ForwardingStatus = supportEmailSenderForwardingVerified
	sender.ForwardingVerifiedAt = &now
	sender.ForwardingLastError = nil
	if sender.EmailRouteID == nil && s.emailRouteRepo != nil {
		if route, err := s.emailRouteRepo.GetActiveByMailbox(ctx, sender.WorkspaceID, sender.MailboxID); err == nil && route != nil {
			sender.EmailRouteID = &route.ID
		}
	}
	if err := s.emailSenderRepo.Update(ctx, sender); err != nil {
		return nil, false, err
	}
	return sender, true, nil
}

func (s *SupportInboxService) VerifyEmailSender(ctx context.Context, workspaceID, senderID string) (*model.SupportEmailSender, error) {
	if s.emailSenderRepo == nil {
		return nil, fmt.Errorf("support email sender repository is unavailable")
	}
	if s.postmarkDomainClient == nil {
		return nil, fmt.Errorf("postmark account token is not configured")
	}
	sender, err := s.emailSenderRepo.GetByID(ctx, workspaceID, senderID)
	if err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, fmt.Errorf("email sender not found")
	}
	if sender.PostmarkDomainID == nil || *sender.PostmarkDomainID == 0 {
		return nil, fmt.Errorf("email sender is missing postmark domain id")
	}
	if err := s.ensureEmailSenderForwardingVerification(ctx, sender); err != nil {
		return nil, err
	}

	lastErr := ""
	if dkim, err := s.postmarkDomainClient.VerifyDKIM(*sender.PostmarkDomainID); err != nil {
		lastErr = err.Error()
	} else if dkim != nil {
		mergePostmarkEmailSender(sender, dkim)
	}
	if rp, err := s.postmarkDomainClient.VerifyReturnPath(*sender.PostmarkDomainID); err != nil {
		if lastErr != "" {
			lastErr += "; "
		}
		lastErr += err.Error()
	} else if rp != nil {
		mergePostmarkEmailSender(sender, rp)
	}
	refreshSupportEmailSenderDMARC(ctx, sender)

	now := time.Now().UTC()
	sender.LastCheckedAt = &now
	sender.VerificationStatus = supportEmailSenderStatus(sender.DKIMVerified, sender.ReturnPathDomainVerified)
	sender.DomainStatus = sender.VerificationStatus
	if lastErr == "" {
		sender.LastError = nil
	} else {
		sender.LastError = &lastErr
	}

	if err := s.emailSenderRepo.Update(ctx, sender); err != nil {
		return nil, err
	}
	return sender, nil
}

func (s *SupportInboxService) SetDefaultEmailSender(ctx context.Context, workspaceID, senderID string, req model.SetSupportEmailSenderDefaultRequest) (*model.SupportEmailSender, error) {
	if s.emailSenderRepo == nil {
		return nil, fmt.Errorf("support email sender repository is unavailable")
	}
	sender, err := s.emailSenderRepo.GetByID(ctx, workspaceID, senderID)
	if err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, fmt.Errorf("email sender not found")
	}
	scope, err := normalizeSupportEmailSenderDefaultScope(req.DefaultScope)
	if err != nil {
		return nil, err
	}
	if scope != supportEmailSenderDefaultScopeNone && (!sender.DKIMVerified || !sender.ReturnPathDomainVerified) {
		return nil, fmt.Errorf("email sender must have verified DKIM and Return-Path records before it can be used")
	}
	if err := s.ensureEmailSenderForwardingVerification(ctx, sender); err != nil {
		return nil, err
	}

	mailboxID := req.MailboxID
	if scope == supportEmailSenderDefaultScopeMailbox {
		if sender.ForwardingStatus != supportEmailSenderForwardingVerified {
			return nil, fmt.Errorf("email sender forwarding must be verified before it can be used as an inbox default")
		}
		if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
			mailboxID = sender.MailboxID
		}
		normalizedMailboxID, _, err := s.sanitizeMailboxSelection(ctx, workspaceID, mailboxID)
		if err != nil {
			return nil, err
		}
		if normalizedMailboxID == nil || strings.TrimSpace(*normalizedMailboxID) == "" {
			return nil, fmt.Errorf("mailbox_id is required for mailbox sender default")
		}
		mailboxID = normalizedMailboxID
	} else {
		mailboxID = nil
	}

	if err := s.emailSenderRepo.SetDefault(ctx, workspaceID, senderID, scope, mailboxID); err != nil {
		return nil, err
	}
	return s.emailSenderRepo.GetByID(ctx, workspaceID, senderID)
}

func (s *SupportInboxService) DisableEmailSender(ctx context.Context, workspaceID, senderID string) error {
	if s.emailSenderRepo == nil {
		return fmt.Errorf("support email sender repository is unavailable")
	}
	sender, err := s.emailSenderRepo.GetByID(ctx, workspaceID, senderID)
	if err != nil {
		return err
	}
	if sender == nil {
		return fmt.Errorf("email sender not found")
	}
	return s.emailSenderRepo.Disable(ctx, workspaceID, senderID)
}

func (s *SupportInboxService) postmarkDomainForSender(ctx context.Context, workspaceID, domainName string) (*email.PostmarkDomain, error) {
	if s.emailSenderRepo != nil {
		existingSender, err := s.emailSenderRepo.GetByDomain(ctx, workspaceID, domainName)
		if err != nil {
			return nil, err
		}
		if existingSender != nil {
			return postmarkDomainFromEmailSender(existingSender), nil
		}
	}
	if s.emailSenderDomainRepo != nil {
		existingDomain, err := s.emailSenderDomainRepo.GetByDomain(ctx, workspaceID, domainName)
		if err != nil {
			return nil, err
		}
		if existingDomain != nil {
			return postmarkDomainFromSenderDomain(existingDomain), nil
		}
	}
	postmarkDomain, err := s.postmarkDomainClient.CreateDomain(domainName, "pm-bounces."+domainName)
	if err != nil {
		if email.IsDomainAlreadyExistsError(err) {
			existingDomain, lookupErr := s.postmarkDomainClient.FindDomainByName(domainName)
			if lookupErr != nil {
				return nil, fmt.Errorf("find existing postmark sender domain: %w", lookupErr)
			}
			if existingDomain != nil {
				return existingDomain, nil
			}
		}
		return nil, fmt.Errorf("create postmark sender domain: %w", err)
	}
	return postmarkDomain, nil
}

func (s *SupportInboxService) generateEmailSenderForwardingVerification(ctx context.Context) (string, string, error) {
	if s == nil {
		return "", "", fmt.Errorf("support inbox service is unavailable")
	}
	for attempt := 0; attempt < 8; attempt++ {
		token, err := generateSecureToken(10)
		if err != nil {
			return "", "", fmt.Errorf("generate forwarding verification token: %w", err)
		}
		if s.emailSenderRepo != nil {
			existing, err := s.emailSenderRepo.GetByForwardingVerificationToken(ctx, token)
			if err != nil {
				return "", "", err
			}
			if existing != nil {
				continue
			}
		}
		return token, "verify-" + token + "@" + s.inboundEmailDomain(), nil
	}
	return "", "", fmt.Errorf("failed to allocate forwarding verification address")
}

func (s *SupportInboxService) ensureEmailSenderForwardingVerification(ctx context.Context, sender *model.SupportEmailSender) error {
	if sender == nil {
		return nil
	}
	if strings.TrimSpace(sender.ForwardingVerificationToken) != "" && strings.TrimSpace(sender.ForwardingAddress) != "" {
		return nil
	}
	token, address, err := s.generateEmailSenderForwardingVerification(ctx)
	if err != nil {
		return err
	}
	sender.ForwardingVerificationToken = token
	sender.ForwardingAddress = address
	if strings.TrimSpace(sender.ForwardingStatus) == "" || sender.ForwardingStatus == supportEmailSenderForwardingNotStarted {
		sender.ForwardingStatus = supportEmailSenderForwardingPending
	}
	if s.emailSenderRepo != nil && strings.TrimSpace(sender.ID) != "" {
		if err := s.emailSenderRepo.Update(ctx, sender); err != nil {
			return err
		}
	}
	return nil
}

func supportEmailSenderFromPostmark(postmarkDomain *email.PostmarkDomain) *model.SupportEmailSender {
	sender := &model.SupportEmailSender{
		DomainStatus:       supportEmailSenderStatusPendingDNS,
		VerificationStatus: supportEmailSenderStatusPendingDNS,
		ForwardingStatus:   supportEmailSenderForwardingNotStarted,
		DefaultScope:       supportEmailSenderDefaultScopeNone,
	}
	mergePostmarkEmailSender(sender, postmarkDomain)
	return sender
}

func mergePostmarkEmailSender(sender *model.SupportEmailSender, postmarkDomain *email.PostmarkDomain) {
	if sender == nil || postmarkDomain == nil {
		return
	}
	if postmarkDomain.ID != 0 {
		sender.PostmarkDomainID = &postmarkDomain.ID
	}
	if strings.TrimSpace(postmarkDomain.ReturnPathDomain) != "" {
		sender.ReturnPathDomain = strings.TrimSpace(postmarkDomain.ReturnPathDomain)
	}
	if strings.TrimSpace(postmarkDomain.ReturnPathDomainCNAMEValue) != "" {
		sender.ReturnPathDomainCNAMEValue = strings.TrimSpace(postmarkDomain.ReturnPathDomainCNAMEValue)
	}
	sender.ReturnPathDomainVerified = postmarkDomain.ReturnPathDomainVerified
	if strings.TrimSpace(postmarkDomain.DKIMHost) != "" {
		sender.DKIMHost = strings.TrimSpace(postmarkDomain.DKIMHost)
	}
	if strings.TrimSpace(postmarkDomain.DKIMTextValue) != "" {
		sender.DKIMTextValue = strings.TrimSpace(postmarkDomain.DKIMTextValue)
	}
	if strings.TrimSpace(postmarkDomain.DKIMPendingHost) != "" {
		sender.DKIMPendingHost = strings.TrimSpace(postmarkDomain.DKIMPendingHost)
	}
	if strings.TrimSpace(postmarkDomain.DKIMPendingTextValue) != "" {
		sender.DKIMPendingTextValue = strings.TrimSpace(postmarkDomain.DKIMPendingTextValue)
	}
	sender.DKIMVerified = postmarkDomain.DKIMVerified
	sender.DKIMUpdateStatus = strings.TrimSpace(postmarkDomain.DKIMUpdateStatus)
	sender.DomainStatus = supportEmailSenderStatus(sender.DKIMVerified, sender.ReturnPathDomainVerified)
	sender.VerificationStatus = sender.DomainStatus
}

func postmarkDomainFromEmailSender(sender *model.SupportEmailSender) *email.PostmarkDomain {
	if sender == nil {
		return nil
	}
	postmarkDomain := &email.PostmarkDomain{
		ReturnPathDomain:           sender.ReturnPathDomain,
		ReturnPathDomainCNAMEValue: sender.ReturnPathDomainCNAMEValue,
		ReturnPathDomainVerified:   sender.ReturnPathDomainVerified,
		DKIMHost:                   sender.DKIMHost,
		DKIMTextValue:              sender.DKIMTextValue,
		DKIMPendingHost:            sender.DKIMPendingHost,
		DKIMPendingTextValue:       sender.DKIMPendingTextValue,
		DKIMVerified:               sender.DKIMVerified,
		DKIMUpdateStatus:           sender.DKIMUpdateStatus,
	}
	if sender.PostmarkDomainID != nil {
		postmarkDomain.ID = *sender.PostmarkDomainID
	}
	return postmarkDomain
}

func postmarkDomainFromSenderDomain(senderDomain *model.SupportEmailSenderDomain) *email.PostmarkDomain {
	if senderDomain == nil {
		return nil
	}
	postmarkDomain := &email.PostmarkDomain{
		ReturnPathDomain:           senderDomain.ReturnPathDomain,
		ReturnPathDomainCNAMEValue: senderDomain.ReturnPathDomainCNAMEValue,
		ReturnPathDomainVerified:   senderDomain.ReturnPathDomainVerified,
		DKIMHost:                   senderDomain.DKIMHost,
		DKIMTextValue:              senderDomain.DKIMTextValue,
		DKIMPendingHost:            senderDomain.DKIMPendingHost,
		DKIMPendingTextValue:       senderDomain.DKIMPendingTextValue,
		DKIMVerified:               senderDomain.DKIMVerified,
		DKIMUpdateStatus:           senderDomain.DKIMUpdateStatus,
	}
	if senderDomain.PostmarkDomainID != nil {
		postmarkDomain.ID = *senderDomain.PostmarkDomainID
	}
	return postmarkDomain
}

func normalizeSupportSenderEmail(value string) (emailAddress, localPart, domainName string, err error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil {
		return "", "", "", fmt.Errorf("enter a valid sender email address")
	}
	parts := strings.Split(parsed.Address, "@")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("enter a valid sender email address")
	}
	localPart, err = normalizeSupportSenderLocalPart(parts[0])
	if err != nil {
		return "", "", "", err
	}
	domainName, err = normalizeSupportSenderDomain(parts[1])
	if err != nil {
		return "", "", "", err
	}
	return localPart + "@" + domainName, localPart, domainName, nil
}

func normalizeSupportEmailSenderDefaultScope(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", supportEmailSenderDefaultScopeNone:
		return supportEmailSenderDefaultScopeNone, nil
	case supportEmailSenderDefaultScopeWorkspace:
		return supportEmailSenderDefaultScopeWorkspace, nil
	case supportEmailSenderDefaultScopeMailbox:
		return supportEmailSenderDefaultScopeMailbox, nil
	default:
		return "", fmt.Errorf("unsupported sender default scope")
	}
}

func inboundPayloadMentionsAddress(payload model.PostmarkInboundPayload, address string) bool {
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" {
		return false
	}
	candidates := []string{
		payload.From,
		payload.FromFull.Email,
		payload.To,
		payload.OriginalRecipient,
		payload.Subject,
	}
	for _, addr := range payload.ToFull {
		candidates = append(candidates, addr.Email)
	}
	for _, header := range payload.Headers {
		name := strings.ToLower(strings.TrimSpace(header.Name))
		switch name {
		case "to", "delivered-to", "x-original-to", "x-forwarded-to", "x-envelope-to", "original-recipient", "resent-to", "forwarded-to":
			candidates = append(candidates, header.Value)
		}
	}
	for _, candidate := range candidates {
		if emailTextMentionsAddress(candidate, address) {
			return true
		}
	}
	return false
}

func emailTextMentionsAddress(value, address string) bool {
	value = strings.TrimSpace(value)
	address = strings.ToLower(strings.TrimSpace(address))
	if value == "" || address == "" {
		return false
	}
	if parsed, err := mail.ParseAddressList(value); err == nil {
		for _, addr := range parsed {
			if strings.EqualFold(strings.TrimSpace(addr.Address), address) {
				return true
			}
		}
	}
	lower := strings.ToLower(value)
	start := 0
	for {
		idx := strings.Index(lower[start:], address)
		if idx < 0 {
			return false
		}
		idx += start
		beforeOK := idx == 0 || !isEmailAddressRune(rune(lower[idx-1]))
		after := idx + len(address)
		afterOK := after >= len(lower) || !isEmailAddressRune(rune(lower[after]))
		if beforeOK && afterOK {
			return true
		}
		start = idx + len(address)
	}
}

func isEmailAddressRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') ||
		r == '.' ||
		r == '_' ||
		r == '%' ||
		r == '+' ||
		r == '-'
}

func refreshSupportEmailSenderDMARC(ctx context.Context, sender *model.SupportEmailSender) {
	if sender == nil || strings.TrimSpace(sender.Domain) == "" {
		return
	}
	now := time.Now().UTC()
	sender.DMARCHost = "_dmarc." + strings.TrimSpace(sender.Domain)
	sender.DMARCLastCheckedAt = &now
	records, err := net.DefaultResolver.LookupTXT(ctx, sender.DMARCHost)
	if err != nil {
		sender.DMARCRecordPresent = false
		sender.DMARCPolicy = ""
		return
	}
	sender.DMARCRecordPresent = len(records) > 0
	sender.DMARCPolicy = parseDMARCPolicy(records)
}

func parseDMARCPolicy(records []string) string {
	for _, record := range records {
		fields := strings.Split(record, ";")
		for _, field := range fields {
			field = strings.TrimSpace(strings.ToLower(field))
			if strings.HasPrefix(field, "p=") {
				policy := strings.TrimSpace(strings.TrimPrefix(field, "p="))
				if policy == "none" || policy == "quarantine" || policy == "reject" {
					return policy
				}
			}
		}
	}
	return ""
}
