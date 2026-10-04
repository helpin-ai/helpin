package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// A custom domain goes live when the workspace proves it owns it with a TXT
// record and the domain points at Helpin with a CNAME. A CNAME alone only
// shows the domain points at Helpin, not which workspace it belongs to, so a
// dangling record could otherwise be claimed by anyone.
const (
	helpcenterChallengePrefix = "_helpin-challenge."
	helpcenterChallengeValue  = "helpin-verify="

	helpcenterPendingCheckEvery = time.Hour
	helpcenterLiveCheckEvery    = 24 * time.Hour
	helpcenterCheckBatch        = 50
)

// ErrHelpcenterDomainNotSet means there is no custom domain to check.
var ErrHelpcenterDomainNotSet = errors.New("no custom domain is set")

// HelpcenterDNSResolver looks up the records a custom domain needs.
type HelpcenterDNSResolver interface {
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// helpcenterDomainNotifier sends admin alerts about a custom domain.
type helpcenterDomainNotifier interface {
	Emit(ctx context.Context, event model.NotificationEventInput) error
}

// helpcenterAdminLister lists who is told about custom domain changes.
type helpcenterAdminLister interface {
	ListAdminOwnerUserIDs(ctx context.Context, workspaceID string) ([]string, error)
}

type helpcenterDomainVerification struct {
	target   string
	resolver HelpcenterDNSResolver
	notifier helpcenterDomainNotifier
	admins   helpcenterAdminLister
	now      func() time.Time
}

// SetCustomDomainVerification requires custom domains to point at target
// (such as helpin.center) and carry the workspace's TXT token before they
// serve. Without a target (Community), custom domains serve as entered.
func (s *DocsHelpcenterService) SetCustomDomainVerification(target string, resolver HelpcenterDNSResolver, notifier helpcenterDomainNotifier, admins helpcenterAdminLister) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	s.domainVerification = &helpcenterDomainVerification{
		target:   strings.ToLower(strings.Trim(strings.TrimSpace(target), ".")),
		resolver: resolver, notifier: notifier, admins: admins, now: time.Now,
	}
}

// CustomDomainTarget is the host custom domains CNAME to, or "" when custom
// domains are not verified.
func (s *DocsHelpcenterService) CustomDomainTarget() string {
	if s.domainVerification == nil {
		return ""
	}
	return s.domainVerification.target
}

// CustomDomainChallenge is the TXT record that proves the workspace owns its
// custom domain.
func CustomDomainChallenge(cfg *model.DocsHelpcenterConfig) (name, value string) {
	if cfg == nil || cfg.CustomDomain == nil || cfg.CustomDomainToken == nil {
		return "", ""
	}
	return helpcenterChallengePrefix + *cfg.CustomDomain, helpcenterChallengeValue + *cfg.CustomDomainToken
}

// customDomainUpdates prepares a changed custom domain: another workspace's
// unproven claim gives way, and the new domain waits for verification.
func (s *DocsHelpcenterService) customDomainUpdates(ctx context.Context, workspaceID string, existing *model.DocsHelpcenterConfig, domain *string, updates map[string]any) error {
	if domain == nil {
		for _, column := range []string{"custom_domain_status", "custom_domain_verified_at", "custom_domain_checked_at", "custom_domain_last_error", "custom_domain_failing_since", "custom_domain_alerted_status"} {
			updates[column] = nil
		}
		return nil
	}
	if s.CustomDomainTarget() == "" {
		// Nothing to verify against: the domain serves as entered.
		updates["custom_domain_status"] = model.HelpcenterDomainVerified
		updates["custom_domain_verified_at"] = time.Now().UTC()
		return nil
	}
	if *domain == s.CustomDomainTarget() || strings.HasSuffix(*domain, "."+s.CustomDomainTarget()) {
		return fmt.Errorf("custom domain must be a domain you own, such as help.yourcompany.com")
	}
	if err := s.hcRepo.ReleaseHelpcenterDomainClaims(ctx, workspaceID, *domain); err != nil {
		if errors.Is(err, repository.ErrHelpcenterDomainTaken) {
			return err
		}
		return fmt.Errorf("check custom domain: %w", err)
	}
	updates["custom_domain_status"] = model.HelpcenterDomainPending
	for _, column := range []string{"custom_domain_verified_at", "custom_domain_checked_at", "custom_domain_last_error", "custom_domain_failing_since", "custom_domain_alerted_status"} {
		updates[column] = nil
	}
	if existing == nil || existing.CustomDomainToken == nil || *existing.CustomDomainToken == "" {
		token, err := newHelpcenterDomainToken()
		if err != nil {
			return err
		}
		updates["custom_domain_token"] = token
	}
	return nil
}

// VerifyCustomDomain checks the workspace's custom domain now and returns the
// updated config.
func (s *DocsHelpcenterService) VerifyCustomDomain(ctx context.Context, workspaceID string) (*model.DocsHelpcenterConfig, error) {
	cfg, err := s.hcRepo.GetConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if cfg == nil || cfg.CustomDomain == nil || *cfg.CustomDomain == "" {
		return nil, ErrHelpcenterDomainNotSet
	}
	if s.CustomDomainTarget() == "" {
		return cfg, nil
	}
	if err := s.checkCustomDomain(ctx, cfg); err != nil {
		return nil, err
	}
	return s.GetConfig(ctx, workspaceID)
}

// RunCustomDomainChecks re-checks custom domains until ctx ends: pending
// domains hourly until they go live, live domains daily. Each domain is
// claimed by one replica per round.
func (s *DocsHelpcenterService) RunCustomDomainChecks(ctx context.Context) {
	if s.CustomDomainTarget() == "" {
		return
	}
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		s.checkDueCustomDomains(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *DocsHelpcenterService) checkDueCustomDomains(ctx context.Context) {
	for ctx.Err() == nil {
		due, err := s.hcRepo.ClaimHelpcenterDomainChecks(ctx, s.domainVerification.now().UTC(), helpcenterPendingCheckEvery, helpcenterLiveCheckEvery, helpcenterCheckBatch)
		if err != nil {
			slog.WarnContext(ctx, "help center domain check claim failed", "error", err)
			return
		}
		for i := range due {
			if err := s.checkCustomDomain(ctx, &due[i]); err != nil {
				slog.WarnContext(ctx, "help center domain check failed", "workspace_id", due[i].WorkspaceID, "error", err)
			}
		}
		if len(due) < helpcenterCheckBatch {
			return
		}
	}
}

// checkCustomDomain records whether the domain is proven and points at
// Helpin, and tells admins when a domain goes live, breaks, or recovers.
func (s *DocsHelpcenterService) checkCustomDomain(ctx context.Context, cfg *model.DocsHelpcenterConfig) error {
	v := s.domainVerification
	domain := *cfg.CustomDomain
	now := v.now().UTC()
	previous := derefString(cfg.CustomDomainStatus)
	routed, routeProblem := v.pointsAtTarget(ctx, domain)
	check := repository.HelpcenterDomainCheck{CheckedAt: now, VerifiedAt: cfg.CustomDomainVerifiedAt}

	switch {
	case previous == model.HelpcenterDomainVerified || previous == model.HelpcenterDomainFailing:
		// Ownership was proven; the daily check only watches that DNS still points here.
		if routed {
			check.Status = model.HelpcenterDomainVerified
		} else {
			check.Status = model.HelpcenterDomainFailing
			check.LastError = &routeProblem
			check.FailingSince = cfg.CustomDomainFailingSince
			if check.FailingSince == nil {
				check.FailingSince = &now
			}
		}
	default:
		owned := v.provesOwnership(ctx, domain, derefString(cfg.CustomDomainToken))
		check.Status = model.HelpcenterDomainPending
		switch {
		case owned && routed:
			check.Status = model.HelpcenterDomainVerified
			check.VerifiedAt = &now
		case !owned && !routed:
			problem := fmt.Sprintf("Add both DNS records below. %s", routeProblem)
			check.LastError = &problem
		case !owned:
			problem := fmt.Sprintf("We couldn’t find the TXT record %s%s yet. DNS changes can take a few minutes to appear.", helpcenterChallengePrefix, domain)
			check.LastError = &problem
		default:
			check.LastError = &routeProblem
		}
	}

	if err := s.hcRepo.RecordHelpcenterDomainCheck(ctx, cfg.WorkspaceID, domain, check); err != nil {
		return err
	}
	if check.Status != previous {
		slog.InfoContext(ctx, "help center domain status changed", "workspace_id", cfg.WorkspaceID, "from", previous, "to", check.Status)
		s.InvalidateHelpcenterCacheForWorkspace(ctx, cfg.WorkspaceID)
	}
	s.alertCustomDomain(ctx, cfg, check.Status)
	return nil
}

// alertCustomDomain tells workspace admins once per change that the domain
// went live, stopped pointing at Helpin, or recovered.
func (s *DocsHelpcenterService) alertCustomDomain(ctx context.Context, cfg *model.DocsHelpcenterConfig, status string) {
	v := s.domainVerification
	alerted := derefString(cfg.CustomDomainAlertedStatus)
	if status == model.HelpcenterDomainPending || status == alerted || v.notifier == nil || v.admins == nil {
		return
	}
	domain := *cfg.CustomDomain
	event := model.NotificationEventInput{
		WorkspaceID: cfg.WorkspaceID, EventType: "helpcenter.custom_domain_" + status,
		EntityType: "helpcenter_config", EntityID: cfg.ID,
		Category: model.NotifCategoryStatusChanges, Priority: "normal",
		Metadata:      model.JSONB{"domain": domain, "status": status, "navigation_target": "settings_helpcenter"},
		SkipFollowers: true, SkipEmailDelivery: true,
	}
	switch {
	case status == model.HelpcenterDomainFailing:
		event.Title = domain + " stopped pointing to Helpin"
		event.Body = "Your help center and customer portal on " + domain + " may be unreachable. Check its DNS records in Help center settings."
		event.Priority = "high"
		event.SkipEmailDelivery = false
	case alerted == model.HelpcenterDomainFailing:
		event.Title = domain + " is working again"
		event.Body = "DNS for " + domain + " points to Helpin again."
	default:
		event.Title = domain + " is live"
		event.Body = "Your help center and customer portal are now served on " + domain + "."
	}
	recipients, err := v.admins.ListAdminOwnerUserIDs(ctx, cfg.WorkspaceID)
	if err != nil || len(recipients) == 0 {
		if err != nil {
			slog.WarnContext(ctx, "help center domain alert recipients failed", "workspace_id", cfg.WorkspaceID, "error", err)
		}
		return
	}
	event.ExplicitRecipients = recipients
	if err := v.notifier.Emit(ctx, event); err != nil {
		slog.WarnContext(ctx, "help center domain alert failed", "workspace_id", cfg.WorkspaceID, "error", err)
		return
	}
	if err := s.hcRepo.MarkHelpcenterDomainAlerted(ctx, cfg.WorkspaceID, domain, status); err != nil {
		slog.WarnContext(ctx, "help center domain alert state failed", "workspace_id", cfg.WorkspaceID, "error", err)
	}
}

// pointsAtTarget accepts a CNAME to the target, or a flattened record that
// resolves to the target's addresses.
func (v *helpcenterDomainVerification) pointsAtTarget(ctx context.Context, domain string) (bool, string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cname, err := v.resolver.LookupCNAME(ctx, domain)
	cname = strings.ToLower(strings.TrimSuffix(cname, "."))
	if err == nil && (cname == v.target || strings.HasSuffix(cname, "."+v.target)) {
		return true, ""
	}
	missing := fmt.Sprintf("%s doesn’t point to %s yet. Add a CNAME record for %s pointing to %s.", domain, v.target, domain, v.target)
	addresses, err := v.resolver.LookupHost(ctx, domain)
	if err != nil || len(addresses) == 0 {
		return false, missing
	}
	targets, err := v.resolver.LookupHost(ctx, v.target)
	if err == nil && sharesAddress(addresses, targets) {
		return true, ""
	}
	return false, missing
}

// provesOwnership looks for the workspace's token at _helpin-challenge.<domain>.
func (v *helpcenterDomainVerification) provesOwnership(ctx context.Context, domain, token string) bool {
	if token == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	records, err := v.resolver.LookupTXT(ctx, helpcenterChallengePrefix+domain)
	if err != nil {
		return false
	}
	for _, record := range records {
		value := strings.Trim(strings.TrimSpace(record), `"`)
		if value == helpcenterChallengeValue+token || value == token {
			return true
		}
	}
	return false
}

func sharesAddress(a, b []string) bool {
	seen := make(map[string]struct{}, len(b))
	for _, address := range b {
		seen[address] = struct{}{}
	}
	for _, address := range a {
		if _, ok := seen[address]; ok {
			return true
		}
	}
	return false
}

func newHelpcenterDomainToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate domain token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// PublicHelpcenterConfig is the config the public help center sees: an
// unverified custom domain is left out, and so is verification detail.
func PublicHelpcenterConfig(cfg *model.DocsHelpcenterConfig) *model.DocsHelpcenterConfig {
	if cfg == nil {
		return nil
	}
	public := *cfg
	if !cfg.CustomDomainLive() {
		public.CustomDomain = nil
		if public.PublicURLMode == model.HelpcenterPublicURLModeCustomDomain {
			public.PublicURLMode = model.HelpcenterPublicURLModeHostedSubdomain
		}
	}
	public.CustomDomainStatus = nil
	public.CustomDomainToken = nil
	public.CustomDomainVerifiedAt = nil
	public.CustomDomainCheckedAt = nil
	public.CustomDomainLastError = nil
	public.CustomDomainFailingSince = nil
	return &public
}
