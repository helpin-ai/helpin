package service

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const supportEmailSenderStatusPendingDNS = "pending_dns"
const supportEmailSenderStatusVerified = "verified"

func (s *SupportInboxService) ListEmailSenderDomains(ctx context.Context, workspaceID string) ([]model.SupportEmailSenderDomain, error) {
	if s.emailSenderDomainRepo == nil {
		return nil, fmt.Errorf("support email sender domain repository is unavailable")
	}
	return s.emailSenderDomainRepo.ListByWorkspace(ctx, workspaceID)
}

func (s *SupportInboxService) CreateEmailSenderDomain(ctx context.Context, workspaceID string, req model.CreateSupportEmailSenderDomainRequest, actorID string) (*model.SupportEmailSenderDomain, error) {
	if s.emailSenderDomainRepo == nil {
		return nil, fmt.Errorf("support email sender domain repository is unavailable")
	}
	if s.postmarkDomainClient == nil {
		return nil, fmt.Errorf("postmark account token is not configured")
	}
	domainName, err := normalizeSupportSenderDomain(req.Domain)
	if err != nil {
		return nil, err
	}
	localPart, err := normalizeSupportSenderLocalPart(req.FromLocalPart)
	if err != nil {
		return nil, err
	}
	if actorID = strings.TrimSpace(actorID); actorID == "" {
		return nil, fmt.Errorf("actor is required")
	}

	existing, err := s.emailSenderDomainRepo.GetByDomain(ctx, workspaceID, domainName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	postmarkDomain, err := s.postmarkDomainClient.CreateDomain(domainName, "pm-bounces."+domainName)
	if err != nil {
		if email.IsDomainAlreadyExistsError(err) {
			existingDomain, lookupErr := s.postmarkDomainClient.FindDomainByName(domainName)
			if lookupErr != nil {
				return nil, fmt.Errorf("create postmark sender domain: %w; lookup existing domain: %w", err, lookupErr)
			}
			postmarkDomain = existingDomain
		} else {
			return nil, fmt.Errorf("create postmark sender domain: %w", err)
		}
	}
	senderDomain := supportEmailSenderDomainFromPostmark(postmarkDomain)
	senderDomain.WorkspaceID = workspaceID
	senderDomain.Domain = domainName
	senderDomain.FromLocalPart = localPart
	senderDomain.CreatedByID = actorID

	if err := s.emailSenderDomainRepo.Create(ctx, senderDomain); err != nil {
		return nil, err
	}
	return senderDomain, nil
}

func (s *SupportInboxService) VerifyEmailSenderDomain(ctx context.Context, workspaceID, domainID string) (*model.SupportEmailSenderDomain, error) {
	if s.emailSenderDomainRepo == nil {
		return nil, fmt.Errorf("support email sender domain repository is unavailable")
	}
	if s.postmarkDomainClient == nil {
		return nil, fmt.Errorf("postmark account token is not configured")
	}
	senderDomain, err := s.emailSenderDomainRepo.GetByID(ctx, workspaceID, domainID)
	if err != nil {
		return nil, err
	}
	if senderDomain == nil {
		return nil, fmt.Errorf("sender domain not found")
	}
	if senderDomain.PostmarkDomainID == nil || *senderDomain.PostmarkDomainID == 0 {
		return nil, fmt.Errorf("sender domain is missing postmark domain id")
	}

	lastErr := ""
	if dkim, err := s.postmarkDomainClient.VerifyDKIM(*senderDomain.PostmarkDomainID); err != nil {
		lastErr = err.Error()
	} else if dkim != nil {
		mergePostmarkSenderDomain(senderDomain, dkim)
	}
	if rp, err := s.postmarkDomainClient.VerifyReturnPath(*senderDomain.PostmarkDomainID); err != nil {
		if lastErr != "" {
			lastErr += "; "
		}
		lastErr += err.Error()
	} else if rp != nil {
		mergePostmarkSenderDomain(senderDomain, rp)
	}

	now := time.Now().UTC()
	senderDomain.LastCheckedAt = &now
	senderDomain.Status = supportEmailSenderStatus(senderDomain.DKIMVerified, senderDomain.ReturnPathDomainVerified)
	if lastErr == "" {
		senderDomain.LastError = nil
	} else {
		senderDomain.LastError = &lastErr
	}
	if err := s.emailSenderDomainRepo.Update(ctx, senderDomain); err != nil {
		return nil, err
	}
	return senderDomain, nil
}

func (s *SupportInboxService) ActivateEmailSenderDomain(ctx context.Context, workspaceID, domainID string) (*model.SupportEmailSenderDomain, error) {
	if s.emailSenderDomainRepo == nil {
		return nil, fmt.Errorf("support email sender domain repository is unavailable")
	}
	senderDomain, err := s.emailSenderDomainRepo.GetByID(ctx, workspaceID, domainID)
	if err != nil {
		return nil, err
	}
	if senderDomain == nil {
		return nil, fmt.Errorf("sender domain not found")
	}
	if !senderDomain.DKIMVerified || !senderDomain.ReturnPathDomainVerified {
		return nil, fmt.Errorf("sender domain must have verified DKIM and Return-Path records before activation")
	}
	if err := s.emailSenderDomainRepo.Activate(ctx, workspaceID, domainID); err != nil {
		return nil, err
	}
	return s.emailSenderDomainRepo.GetByID(ctx, workspaceID, domainID)
}

func (s *SupportInboxService) DeactivateEmailSenderDomain(ctx context.Context, workspaceID, domainID string) error {
	if s.emailSenderDomainRepo == nil {
		return fmt.Errorf("support email sender domain repository is unavailable")
	}
	senderDomain, err := s.emailSenderDomainRepo.GetByID(ctx, workspaceID, domainID)
	if err != nil {
		return err
	}
	if senderDomain == nil {
		return fmt.Errorf("sender domain not found")
	}
	return s.emailSenderDomainRepo.Deactivate(ctx, workspaceID, domainID)
}

func supportEmailSenderDomainFromPostmark(postmarkDomain *email.PostmarkDomain) *model.SupportEmailSenderDomain {
	senderDomain := &model.SupportEmailSenderDomain{
		Status: supportEmailSenderStatusPendingDNS,
	}
	mergePostmarkSenderDomain(senderDomain, postmarkDomain)
	return senderDomain
}

func mergePostmarkSenderDomain(senderDomain *model.SupportEmailSenderDomain, postmarkDomain *email.PostmarkDomain) {
	if senderDomain == nil || postmarkDomain == nil {
		return
	}
	if postmarkDomain.ID != 0 {
		senderDomain.PostmarkDomainID = &postmarkDomain.ID
	}
	if strings.TrimSpace(postmarkDomain.ReturnPathDomain) != "" {
		senderDomain.ReturnPathDomain = strings.TrimSpace(postmarkDomain.ReturnPathDomain)
	}
	if strings.TrimSpace(postmarkDomain.ReturnPathDomainCNAMEValue) != "" {
		senderDomain.ReturnPathDomainCNAMEValue = strings.TrimSpace(postmarkDomain.ReturnPathDomainCNAMEValue)
	}
	senderDomain.ReturnPathDomainVerified = postmarkDomain.ReturnPathDomainVerified
	if strings.TrimSpace(postmarkDomain.DKIMHost) != "" {
		senderDomain.DKIMHost = strings.TrimSpace(postmarkDomain.DKIMHost)
	}
	if strings.TrimSpace(postmarkDomain.DKIMTextValue) != "" {
		senderDomain.DKIMTextValue = strings.TrimSpace(postmarkDomain.DKIMTextValue)
	}
	if strings.TrimSpace(postmarkDomain.DKIMPendingHost) != "" {
		senderDomain.DKIMPendingHost = strings.TrimSpace(postmarkDomain.DKIMPendingHost)
	}
	if strings.TrimSpace(postmarkDomain.DKIMPendingTextValue) != "" {
		senderDomain.DKIMPendingTextValue = strings.TrimSpace(postmarkDomain.DKIMPendingTextValue)
	}
	senderDomain.DKIMVerified = postmarkDomain.DKIMVerified
	senderDomain.DKIMUpdateStatus = strings.TrimSpace(postmarkDomain.DKIMUpdateStatus)
	senderDomain.Status = supportEmailSenderStatus(senderDomain.DKIMVerified, senderDomain.ReturnPathDomainVerified)
}

func supportEmailSenderStatus(dkimVerified, returnPathVerified bool) string {
	if dkimVerified && returnPathVerified {
		return supportEmailSenderStatusVerified
	}
	return supportEmailSenderStatusPendingDNS
}

func normalizeSupportSenderDomain(value string) (string, error) {
	domain := strings.TrimSpace(strings.ToLower(value))
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimSuffix(domain, ".")
	if idx := strings.Index(domain, "/"); idx >= 0 {
		domain = domain[:idx]
	}
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	if strings.Contains(domain, "@") || strings.Contains(domain, "_") {
		return "", fmt.Errorf("enter a valid domain, for example example.com")
	}
	if strings.Contains(domain, "..") || !strings.Contains(domain, ".") {
		return "", fmt.Errorf("enter a valid domain, for example example.com")
	}
	if ip := net.ParseIP(domain); ip != nil {
		return "", fmt.Errorf("sender domain cannot be an IP address")
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return "", fmt.Errorf("enter a valid domain, for example example.com")
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("domain labels cannot start or end with a hyphen")
		}
		for _, r := range label {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-') {
				return "", fmt.Errorf("domain can only contain letters, numbers, dots, and hyphens")
			}
		}
	}
	return domain, nil
}

func normalizeSupportSenderLocalPart(value string) (string, error) {
	local := strings.TrimSpace(strings.ToLower(value))
	if local == "" {
		local = "support"
	}
	if len(local) > 64 {
		return "", fmt.Errorf("sender local part is too long")
	}
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, "..") {
		return "", fmt.Errorf("sender local part cannot start or end with a dot")
	}
	for _, r := range local {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("sender local part can only contain letters, numbers, dots, hyphens, and underscores")
	}
	return local, nil
}
