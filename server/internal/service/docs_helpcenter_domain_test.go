package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type fakeHelpcenterDNS struct {
	cnames map[string]string
	hosts  map[string][]string
	txt    map[string][]string
}

func (f *fakeHelpcenterDNS) LookupCNAME(_ context.Context, host string) (string, error) {
	if cname, ok := f.cnames[host]; ok {
		return cname, nil
	}
	return "", errors.New("no such host")
}

func (f *fakeHelpcenterDNS) LookupHost(_ context.Context, host string) ([]string, error) {
	if addresses, ok := f.hosts[host]; ok {
		return addresses, nil
	}
	return nil, errors.New("no such host")
}

func (f *fakeHelpcenterDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	if records, ok := f.txt[name]; ok {
		return records, nil
	}
	return nil, errors.New("no such host")
}

type recordingDomainNotifier struct {
	events []model.NotificationEventInput
}

func (n *recordingDomainNotifier) Emit(_ context.Context, event model.NotificationEventInput) error {
	n.events = append(n.events, event)
	return nil
}

type fixedAdmins []string

func (a fixedAdmins) ListAdminOwnerUserIDs(context.Context, string) ([]string, error) { return a, nil }

func setupHelpcenterDomains(t *testing.T) (*DocsHelpcenterService, *gorm.DB, *fakeHelpcenterDNS, *recordingDomainNotifier, *time.Time) {
	t.Helper()
	db := setupDocsHelpcenterConfigTestDB(t)
	svc := newDocsHelpcenterConfigServiceForTest(db)
	dns := &fakeHelpcenterDNS{
		cnames: map[string]string{},
		hosts:  map[string][]string{"helpin.center": {"128.140.24.239"}},
		txt:    map[string][]string{},
	}
	notifier := &recordingDomainNotifier{}
	svc.SetCustomDomainVerification("helpin.center", dns, notifier, fixedAdmins{"admin-1"})
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	svc.domainVerification.now = func() time.Time { return now }
	return svc, db, dns, notifier, &now
}

func setCustomDomain(t *testing.T, svc *DocsHelpcenterService, workspaceID, subdomain, domain string) *model.DocsHelpcenterConfig {
	t.Helper()
	cfg, err := svc.UpsertConfig(context.Background(), workspaceID, model.UpdateDocsHelpcenterConfigRequest{
		Subdomain: stringPtr(subdomain), BrandName: stringPtr("Docs"), CustomDomain: stringPtr(domain),
		PublicURLMode: stringPtr(model.HelpcenterPublicURLModeCustomDomain),
	})
	if err != nil {
		t.Fatalf("set custom domain %s: %v", domain, err)
	}
	return cfg
}

func TestCustomDomainGoesLiveOnlyWithOwnershipAndDNS(t *testing.T) {
	ctx := context.Background()
	svc, _, dns, notifier, _ := setupHelpcenterDomains(t)

	cfg := setCustomDomain(t, svc, "ws-acme", "acme", "Help.Acme.com")
	if derefString(cfg.CustomDomainStatus) != model.HelpcenterDomainPending || cfg.CustomDomainToken == nil || cfg.CustomDomainLastError == nil {
		t.Fatalf("new domain = %+v, want pending with a token and a reason", cfg)
	}
	name, value := CustomDomainChallenge(cfg)
	if name != "_helpin-challenge.help.acme.com" || value != "helpin-verify="+*cfg.CustomDomainToken {
		t.Fatalf("challenge = %q %q", name, value)
	}

	// The CNAME alone doesn't prove which workspace owns the domain.
	dns.cnames["help.acme.com"] = "helpin.center."
	if cfg, _ = svc.VerifyCustomDomain(ctx, "ws-acme"); derefString(cfg.CustomDomainStatus) != model.HelpcenterDomainPending ||
		!strings.Contains(derefString(cfg.CustomDomainLastError), "TXT record _helpin-challenge.help.acme.com") {
		t.Fatalf("CNAME only = %v %q", derefString(cfg.CustomDomainStatus), derefString(cfg.CustomDomainLastError))
	}
	// The TXT alone doesn't route traffic to Helpin.
	dns.cnames = map[string]string{}
	dns.txt[name] = []string{value}
	if cfg, _ = svc.VerifyCustomDomain(ctx, "ws-acme"); derefString(cfg.CustomDomainStatus) != model.HelpcenterDomainPending ||
		!strings.Contains(derefString(cfg.CustomDomainLastError), "CNAME record for help.acme.com pointing to helpin.center") {
		t.Fatalf("TXT only = %v %q", derefString(cfg.CustomDomainStatus), derefString(cfg.CustomDomainLastError))
	}
	if len(notifier.events) != 0 {
		t.Fatalf("pending domain alerted admins: %+v", notifier.events)
	}

	dns.cnames["help.acme.com"] = "helpin.center."
	cfg, err := svc.VerifyCustomDomain(ctx, "ws-acme")
	if err != nil || derefString(cfg.CustomDomainStatus) != model.HelpcenterDomainVerified || cfg.CustomDomainVerifiedAt == nil || cfg.CustomDomainLastError != nil {
		t.Fatalf("verified = %+v, %v", cfg, err)
	}
	if len(notifier.events) != 1 || notifier.events[0].Title != "help.acme.com is live" || notifier.events[0].ExplicitRecipients[0] != "admin-1" {
		t.Fatalf("live alert = %+v", notifier.events)
	}
	if resolved, _ := svc.ResolveConfig(ctx, "help.acme.com"); resolved == nil || resolved.WorkspaceID != "ws-acme" {
		t.Fatalf("verified domain does not resolve: %+v", resolved)
	}
}

func TestPendingCustomDomainsStayPrivate(t *testing.T) {
	ctx := context.Background()
	svc, db, _, _, _ := setupHelpcenterDomains(t)
	cfg := setCustomDomain(t, svc, "ws-acme", "acme", "help.acme.com")

	if resolved, _ := svc.ResolveConfig(ctx, "help.acme.com"); resolved != nil {
		t.Fatal("pending domain serves the help center")
	}
	registered, err := repository.NewDocsHelpcenterRepository(db).CustomDomainRegistered(ctx, "help.acme.com")
	if err != nil || registered {
		t.Fatalf("pending domain allowed a certificate: %v %v", registered, err)
	}
	public := PublicHelpcenterConfig(cfg)
	if public.CustomDomain != nil || public.PublicURLMode != model.HelpcenterPublicURLModeHostedSubdomain || public.CustomDomainToken != nil || public.CustomDomainLastError != nil {
		t.Fatalf("public view = %+v", public)
	}
	if cfg.CustomDomain == nil || cfg.CustomDomainToken == nil {
		t.Fatal("public view changed the stored config")
	}
}

func TestUnprovenClaimsGiveWayToTheOwner(t *testing.T) {
	ctx := context.Background()
	svc, _, dns, _, _ := setupHelpcenterDomains(t)
	setCustomDomain(t, svc, "ws-squatter", "squatter", "help.acme.com")

	owner := setCustomDomain(t, svc, "ws-acme", "acme", "help.acme.com")
	squatter, _ := svc.GetConfig(ctx, "ws-squatter")
	if squatter.CustomDomain != nil || squatter.PublicURLMode != model.HelpcenterPublicURLModeHostedSubdomain {
		t.Fatalf("squatter kept the domain: %+v", squatter)
	}

	name, value := CustomDomainChallenge(owner)
	dns.cnames["help.acme.com"] = "helpin.center."
	dns.txt[name] = []string{value}
	if cfg, err := svc.VerifyCustomDomain(ctx, "ws-acme"); err != nil || !cfg.CustomDomainLive() {
		t.Fatalf("owner verification = %+v, %v", cfg, err)
	}
	if _, err := svc.UpsertConfig(ctx, "ws-squatter", model.UpdateDocsHelpcenterConfigRequest{CustomDomain: stringPtr("help.acme.com")}); !errors.Is(err, repository.ErrHelpcenterDomainTaken) {
		t.Fatalf("claiming a verified domain = %v", err)
	}
	for _, own := range []string{"helpin.center", "docs.helpin.center"} {
		if _, err := svc.UpsertConfig(ctx, "ws-squatter", model.UpdateDocsHelpcenterConfigRequest{CustomDomain: stringPtr(own)}); err == nil {
			t.Errorf("claiming %s was allowed", own)
		}
	}
}

func TestDailyChecksAlertOnFailureAndRecovery(t *testing.T) {
	ctx := context.Background()
	svc, db, dns, notifier, now := setupHelpcenterDomains(t)
	cfg := setCustomDomain(t, svc, "ws-acme", "acme", "help.acme.com")
	name, value := CustomDomainChallenge(cfg)
	dns.txt[name] = []string{value}
	// A flattened record resolving to the custom domain entry also counts.
	dns.hosts["help.acme.com"] = []string{"128.140.24.239"}
	if cfg, _ = svc.VerifyCustomDomain(ctx, "ws-acme"); !cfg.CustomDomainLive() {
		t.Fatalf("flattened record = %+v", cfg)
	}
	notifier.events = nil

	advance := func(d time.Duration) { *now = now.Add(d) }
	// Not due yet: live domains are checked daily.
	advance(time.Hour)
	delete(dns.hosts, "help.acme.com")
	svc.checkDueCustomDomains(ctx)
	if cfg, _ = svc.GetConfig(ctx, "ws-acme"); derefString(cfg.CustomDomainStatus) != model.HelpcenterDomainVerified {
		t.Fatalf("checked before due: %v", derefString(cfg.CustomDomainStatus))
	}

	advance(24 * time.Hour)
	svc.checkDueCustomDomains(ctx)
	cfg, _ = svc.GetConfig(ctx, "ws-acme")
	if derefString(cfg.CustomDomainStatus) != model.HelpcenterDomainFailing || cfg.CustomDomainFailingSince == nil || !cfg.CustomDomainLive() {
		t.Fatalf("failing domain = %+v", cfg)
	}
	if len(notifier.events) != 1 || notifier.events[0].Priority != "high" || notifier.events[0].SkipEmailDelivery || notifier.events[0].EntityType != "helpcenter_config" {
		t.Fatalf("failure alert = %+v", notifier.events)
	}
	// A failing domain keeps serving so a DNS blip doesn't take it offline.
	if resolved, _ := svc.ResolveConfig(ctx, "help.acme.com"); resolved == nil {
		t.Fatal("failing domain stopped serving")
	}

	// Still failing the next day: no repeat alert, and the incident start is kept.
	since := *cfg.CustomDomainFailingSince
	advance(25 * time.Hour)
	svc.checkDueCustomDomains(ctx)
	if cfg, _ = svc.GetConfig(ctx, "ws-acme"); !cfg.CustomDomainFailingSince.Equal(since) || len(notifier.events) != 1 {
		t.Fatalf("repeat failure: since=%v events=%d", cfg.CustomDomainFailingSince, len(notifier.events))
	}

	dns.cnames["help.acme.com"] = "helpin.center."
	advance(25 * time.Hour)
	svc.checkDueCustomDomains(ctx)
	cfg, _ = svc.GetConfig(ctx, "ws-acme")
	if derefString(cfg.CustomDomainStatus) != model.HelpcenterDomainVerified || cfg.CustomDomainFailingSince != nil {
		t.Fatalf("recovered domain = %+v", cfg)
	}
	if len(notifier.events) != 2 || notifier.events[1].Title != "help.acme.com is working again" {
		t.Fatalf("recovery alert = %+v", notifier.events)
	}

	// Two replicas never check the same domain in one round.
	advance(25 * time.Hour)
	repo := repository.NewDocsHelpcenterRepository(db)
	first, _ := repo.ClaimHelpcenterDomainChecks(ctx, *now, time.Hour, 24*time.Hour, 10)
	second, _ := repo.ClaimHelpcenterDomainChecks(ctx, *now, time.Hour, 24*time.Hour, 10)
	if len(first) != 1 || len(second) != 0 {
		t.Fatalf("claims = %d then %d", len(first), len(second))
	}
}

func TestPendingDomainsAreRetriedHourly(t *testing.T) {
	ctx := context.Background()
	svc, _, dns, notifier, now := setupHelpcenterDomains(t)
	cfg := setCustomDomain(t, svc, "ws-acme", "acme", "help.acme.com")
	name, value := CustomDomainChallenge(cfg)
	dns.txt[name] = []string{value}
	dns.cnames["help.acme.com"] = "helpin.center."

	*now = now.Add(61 * time.Minute)
	svc.checkDueCustomDomains(ctx)
	if cfg, _ = svc.GetConfig(ctx, "ws-acme"); !cfg.CustomDomainLive() {
		t.Fatalf("hourly retry = %+v", cfg)
	}
	if len(notifier.events) != 1 || notifier.events[0].Title != "help.acme.com is live" {
		t.Fatalf("live alert = %+v", notifier.events)
	}
}

func TestCustomDomainsServeAsEnteredWithoutVerification(t *testing.T) {
	ctx := context.Background()
	db := setupDocsHelpcenterConfigTestDB(t)
	svc := newDocsHelpcenterConfigServiceForTest(db)
	cfg := setCustomDomain(t, svc, "ws-acme", "acme", "help.acme.com")
	if !cfg.CustomDomainLive() {
		t.Fatalf("Community custom domain = %+v", cfg)
	}
	if resolved, _ := svc.ResolveConfig(ctx, "help.acme.com"); resolved == nil {
		t.Fatal("Community custom domain does not resolve")
	}
	// Removing the domain clears its verification state.
	cfg, err := svc.UpsertConfig(ctx, "ws-acme", model.UpdateDocsHelpcenterConfigRequest{CustomDomain: stringPtr(""), PublicURLMode: stringPtr(model.HelpcenterPublicURLModeHostedSubdomain)})
	if err != nil || cfg.CustomDomain != nil || cfg.CustomDomainStatus != nil {
		t.Fatalf("removed domain = %+v, %v", cfg, err)
	}
}
