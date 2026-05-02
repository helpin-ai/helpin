package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportInboxServiceCreateEmailRouteUsesWorkspaceSlugNamespace(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	mailboxRepo := repository.NewSupportMailboxRepository(db)
	routeRepo := repository.NewSupportEmailRouteRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	svc := NewSupportInboxService(nil, mailboxRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailRouteRepository(routeRepo).
		SetWorkspaceRepo(workspaceRepo).
		SetRouteDomain("on.helpin.email")

	mailbox := &model.SupportMailbox{
		WorkspaceID:    workspaceID,
		Name:           "Billing",
		Handle:         "billing",
		Icon:           "credit-card",
		TriageEligible: true,
		VisibilityMode: "members_only",
		AssignmentMode: "manual",
		Active:         true,
		CreatedByID:    actorID,
	}
	if err := mailboxRepo.Create(ctx, mailbox); err != nil {
		t.Fatalf("create mailbox: %v", err)
	}

	route, err := svc.CreateEmailRoute(ctx, workspaceID, model.CreateSupportEmailRouteRequest{MailboxID: &mailbox.ID}, actorID)
	if err != nil {
		t.Fatalf("create email route: %v", err)
	}
	if route.InboundAddress != "billing@acme.on.helpin.email" {
		t.Fatalf("expected branded mailbox route, got %q", route.InboundAddress)
	}
	if !strings.HasPrefix(route.RouteKey, "route-") {
		t.Fatalf("expected internal route key to be preserved, got %q", route.RouteKey)
	}
}

func TestSupportInboxServiceCreateEmailRouteUsesInboxForSharedRoute(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	routeRepo := repository.NewSupportEmailRouteRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailRouteRepository(routeRepo).
		SetWorkspaceRepo(workspaceRepo).
		SetRouteDomain("on.helpin.email")

	route, err := svc.CreateEmailRoute(ctx, workspaceID, model.CreateSupportEmailRouteRequest{}, actorID)
	if err != nil {
		t.Fatalf("create shared email route: %v", err)
	}
	if route.InboundAddress != "inbox@acme.on.helpin.email" {
		t.Fatalf("expected shared branded route, got %q", route.InboundAddress)
	}
}

func TestSupportInboxServiceUpdateMailboxUpdatesBrandedRouteAddress(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	mailboxRepo := repository.NewSupportMailboxRepository(db)
	routeRepo := repository.NewSupportEmailRouteRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	svc := NewSupportInboxService(nil, mailboxRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailRouteRepository(routeRepo).
		SetWorkspaceRepo(workspaceRepo).
		SetRouteDomain("on.helpin.email")

	mailbox := &model.SupportMailbox{
		WorkspaceID:    workspaceID,
		Name:           "Billing",
		Handle:         "billing",
		Icon:           "credit-card",
		TriageEligible: true,
		VisibilityMode: "members_only",
		AssignmentMode: "manual",
		Active:         true,
		CreatedByID:    actorID,
	}
	if err := mailboxRepo.Create(ctx, mailbox); err != nil {
		t.Fatalf("create mailbox: %v", err)
	}

	route, err := svc.CreateEmailRoute(ctx, workspaceID, model.CreateSupportEmailRouteRequest{MailboxID: &mailbox.ID}, actorID)
	if err != nil {
		t.Fatalf("create email route: %v", err)
	}
	if route.InboundAddress != "billing@acme.on.helpin.email" {
		t.Fatalf("expected initial branded mailbox route, got %q", route.InboundAddress)
	}

	nextHandle := "finance"
	updatedMailbox, err := svc.UpdateMailbox(ctx, workspaceID, mailbox.ID, model.UpdateSupportMailboxRequest{Handle: &nextHandle})
	if err != nil {
		t.Fatalf("update mailbox: %v", err)
	}
	if updatedMailbox.Handle != "finance" {
		t.Fatalf("expected normalized mailbox handle to persist, got %q", updatedMailbox.Handle)
	}

	storedRoute, err := routeRepo.GetByID(ctx, workspaceID, route.ID)
	if err != nil {
		t.Fatalf("reload route: %v", err)
	}
	if storedRoute == nil {
		t.Fatal("expected route to remain active")
	}
	if storedRoute.InboundAddress != "finance@acme.on.helpin.email" {
		t.Fatalf("expected route address to follow mailbox handle, got %q", storedRoute.InboundAddress)
	}
}

func TestSupportInboxServiceCreateMailboxRejectsReservedInboxHandle(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if _, err := svc.CreateMailbox(ctx, workspaceID, model.CreateSupportMailboxRequest{
		Name:           "Inbox",
		Handle:         "inbox",
		AssignmentMode: "manual",
	}, actorID); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved handle error, got %v", err)
	}
}

func TestSupportInboxServiceBuildOutboundFromAddressPrefersVerifiedMailboxSender(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	mailboxRepo := repository.NewSupportMailboxRepository(db)
	senderRepo := repository.NewSupportEmailSenderRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	svc := NewSupportInboxService(nil, mailboxRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
		SetWorkspaceRepo(workspaceRepo).
		SetRouteDomain("on.helpin.email")

	mailbox := &model.SupportMailbox{
		WorkspaceID:    workspaceID,
		Name:           "Billing",
		Handle:         "billing",
		Icon:           "credit-card",
		TriageEligible: true,
		VisibilityMode: "members_only",
		AssignmentMode: "manual",
		Active:         true,
		CreatedByID:    actorID,
	}
	if err := mailboxRepo.Create(ctx, mailbox); err != nil {
		t.Fatalf("create mailbox: %v", err)
	}

	if err := senderRepo.Create(ctx, verifiedSender(workspaceID, nil, "support@example.com", "workspace", actorID)); err != nil {
		t.Fatalf("create workspace sender: %v", err)
	}
	if err := senderRepo.Create(ctx, verifiedSender(workspaceID, &mailbox.ID, "billing@example.com", "mailbox", actorID)); err != nil {
		t.Fatalf("create mailbox sender: %v", err)
	}

	address, err := svc.BuildOutboundFromAddress(ctx, workspaceID, &mailbox.ID)
	if err != nil {
		t.Fatalf("build outbound address: %v", err)
	}
	if address != "billing@example.com" {
		t.Fatalf("expected mailbox sender, got %q", address)
	}
}

func TestSupportInboxServiceBuildOutboundFromAddressFallsBackToWorkspaceSenderThenRouteAddress(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	senderRepo := repository.NewSupportEmailSenderRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
		SetWorkspaceRepo(workspaceRepo).
		SetRouteDomain("on.helpin.email")

	unverified := verifiedSender(workspaceID, nil, "pending@example.com", "workspace", actorID)
	unverified.DKIMVerified = false
	unverified.VerificationStatus = supportEmailSenderStatusPendingDNS
	unverified.DomainStatus = supportEmailSenderStatusPendingDNS
	if err := senderRepo.Create(ctx, unverified); err != nil {
		t.Fatalf("create unverified sender: %v", err)
	}

	address, err := svc.BuildOutboundFromAddress(ctx, workspaceID, nil)
	if err != nil {
		t.Fatalf("build route fallback address: %v", err)
	}
	if address != "inbox@acme.on.helpin.email" {
		t.Fatalf("expected route fallback for unverified sender, got %q", address)
	}

	if err := senderRepo.Create(ctx, verifiedSender(workspaceID, nil, "support@example.com", "workspace", actorID)); err != nil {
		t.Fatalf("create workspace sender: %v", err)
	}
	address, err = svc.BuildOutboundFromAddress(ctx, workspaceID, nil)
	if err != nil {
		t.Fatalf("build workspace sender address: %v", err)
	}
	if address != "support@example.com" {
		t.Fatalf("expected workspace sender, got %q", address)
	}
}

func TestSupportEmailSenderRepositorySetDefaultScopes(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	mailboxRepo := repository.NewSupportMailboxRepository(db)
	senderRepo := repository.NewSupportEmailSenderRepository(db)
	mailbox := &model.SupportMailbox{
		WorkspaceID:    workspaceID,
		Name:           "Billing",
		Handle:         "billing",
		Icon:           "credit-card",
		TriageEligible: true,
		VisibilityMode: "members_only",
		AssignmentMode: "manual",
		Active:         true,
		CreatedByID:    actorID,
	}
	if err := mailboxRepo.Create(ctx, mailbox); err != nil {
		t.Fatalf("create mailbox: %v", err)
	}

	workspaceSender := verifiedSender(workspaceID, nil, "support@example.com", "none", actorID)
	mailboxSender := verifiedSender(workspaceID, nil, "billing@example.com", "none", actorID)
	if err := senderRepo.Create(ctx, workspaceSender); err != nil {
		t.Fatalf("create workspace sender: %v", err)
	}
	if err := senderRepo.Create(ctx, mailboxSender); err != nil {
		t.Fatalf("create mailbox sender: %v", err)
	}

	if err := senderRepo.SetDefault(ctx, workspaceID, workspaceSender.ID, "workspace", nil); err != nil {
		t.Fatalf("set workspace default: %v", err)
	}
	defaultWorkspaceSender, err := senderRepo.GetWorkspaceDefaultVerified(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get workspace default: %v", err)
	}
	if defaultWorkspaceSender == nil || defaultWorkspaceSender.Email != "support@example.com" {
		t.Fatalf("expected workspace default sender, got %#v", defaultWorkspaceSender)
	}

	if err := senderRepo.SetDefault(ctx, workspaceID, mailboxSender.ID, "mailbox", &mailbox.ID); err != nil {
		t.Fatalf("set mailbox default: %v", err)
	}
	defaultMailboxSender, err := senderRepo.GetMailboxDefaultVerified(ctx, workspaceID, &mailbox.ID)
	if err != nil {
		t.Fatalf("get mailbox default: %v", err)
	}
	if defaultMailboxSender == nil || defaultMailboxSender.Email != "billing@example.com" {
		t.Fatalf("expected mailbox default sender, got %#v", defaultMailboxSender)
	}
}

func TestNormalizeSupportSenderEmail(t *testing.T) {
	email, localPart, domain, err := normalizeSupportSenderEmail("Helpin Support <Support@Example.COM>")
	if err != nil {
		t.Fatalf("normalize sender email: %v", err)
	}
	if email != "support@example.com" || localPart != "support" || domain != "example.com" {
		t.Fatalf("unexpected normalized email=%q local=%q domain=%q", email, localPart, domain)
	}

	if _, _, _, err := normalizeSupportSenderEmail("not-an-email"); err == nil {
		t.Fatal("expected invalid sender email to fail")
	}
}

func verifiedSender(workspaceID string, mailboxID *string, emailAddress, defaultScope, actorID string) *model.SupportEmailSender {
	localPart, domain, _ := strings.Cut(emailAddress, "@")
	return &model.SupportEmailSender{
		ID:                         strings.NewReplacer("@", "-", ".", "-").Replace(emailAddress),
		WorkspaceID:                workspaceID,
		MailboxID:                  mailboxID,
		Email:                      emailAddress,
		LocalPart:                  localPart,
		Domain:                     domain,
		DisplayName:                "Support",
		ReturnPathDomain:           "pm-bounces." + domain,
		ReturnPathDomainCNAMEValue: "pm.mtasv.net",
		ReturnPathDomainVerified:   true,
		DKIMHost:                   "pm._domainkey." + domain,
		DKIMTextValue:              "k=rsa; p=test",
		DKIMVerified:               true,
		DomainStatus:               supportEmailSenderStatusVerified,
		ForwardingStatus:           supportEmailSenderForwardingNotStarted,
		VerificationStatus:         supportEmailSenderStatusVerified,
		DefaultScope:               defaultScope,
		Active:                     defaultScope != "none",
		CreatedByID:                actorID,
	}
}
