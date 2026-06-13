package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/email"
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

func TestSupportInboxServiceCompleteEmailSenderForwardingVerification(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	senderRepo := repository.NewSupportEmailSenderRepository(db)
	routeRepo := repository.NewSupportEmailRouteRepository(db)
	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
		SetEmailRouteRepository(routeRepo).
		SetRouteDomain("on.helpin.email")

	sender := verifiedSender(workspaceID, nil, "support@example.com", "none", actorID)
	sender.ForwardingVerificationToken = "abc123"
	sender.ForwardingAddress = "verify-abc123@on.helpin.email"
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	if err := senderRepo.Create(ctx, sender); err != nil {
		t.Fatalf("create sender: %v", err)
	}
	route := &model.SupportEmailRoute{
		ID:             "33333333-3333-3333-3333-333333333333",
		WorkspaceID:    workspaceID,
		RouteKey:       "route-shared123",
		InboundAddress: "inbox@acme.on.helpin.email",
		ProviderType:   "forwarding",
		Active:         true,
		CreatedByID:    actorID,
	}
	if err := routeRepo.Create(ctx, route); err != nil {
		t.Fatalf("create route: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MessageID:         "pm-verify-1",
		OriginalRecipient: sender.ForwardingAddress,
		To:                "Helpin Support <" + sender.Email + ">",
		FromFull:          model.PostmarkAddress{Email: "customer@example.com"},
		Headers: []model.PostmarkHeader{
			{Name: "To", Value: sender.Email},
		},
	}
	updated, verified, err := svc.CompleteEmailSenderForwardingVerification(ctx, "verify-abc123", payload)
	if err != nil {
		t.Fatalf("complete forwarding verification: %v", err)
	}
	if !verified {
		t.Fatal("expected forwarding verification to succeed")
	}
	if updated == nil || updated.ForwardingStatus != supportEmailSenderForwardingVerified || updated.ForwardingVerifiedAt == nil {
		t.Fatalf("unexpected updated sender: %#v", updated)
	}
	if updated.ForwardingLastError != nil {
		t.Fatalf("expected forwarding error to be cleared, got %q", *updated.ForwardingLastError)
	}
	if updated.EmailRouteID == nil || *updated.EmailRouteID != route.ID {
		t.Fatalf("expected active route to be attached, got %#v", updated.EmailRouteID)
	}
}

func TestSupportInboxServiceCompleteEmailSenderForwardingVerificationRejectsWrongAddress(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	senderRepo := repository.NewSupportEmailSenderRepository(db)
	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
		SetRouteDomain("on.helpin.email")

	sender := verifiedSender(workspaceID, nil, "support@example.com", "none", actorID)
	sender.ForwardingVerificationToken = "abc123"
	sender.ForwardingAddress = "verify-abc123@on.helpin.email"
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	if err := senderRepo.Create(ctx, sender); err != nil {
		t.Fatalf("create sender: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MessageID:         "pm-verify-wrong",
		OriginalRecipient: sender.ForwardingAddress,
		To:                "not-support@example.com",
		Headers: []model.PostmarkHeader{
			{Name: "X-Forwarded-To", Value: "not-support@example.com"},
		},
	}
	updated, verified, err := svc.CompleteEmailSenderForwardingVerification(ctx, "abc123", payload)
	if err != nil {
		t.Fatalf("complete forwarding verification: %v", err)
	}
	if verified {
		t.Fatal("expected forwarding verification to fail for wrong sender address")
	}
	if updated == nil || updated.ForwardingStatus != supportEmailSenderForwardingFailed {
		t.Fatalf("expected failed sender state, got %#v", updated)
	}
	if updated.ForwardingVerifiedAt != nil {
		t.Fatalf("expected forwarding_verified_at to remain nil, got %v", updated.ForwardingVerifiedAt)
	}
	if updated.ForwardingLastCheckedAt == nil {
		t.Fatal("expected forwarding_last_checked_at to be set")
	}
	if updated.ForwardingLastError == nil || !strings.Contains(*updated.ForwardingLastError, "did not reference sender address") {
		t.Fatalf("expected useful forwarding error, got %#v", updated.ForwardingLastError)
	}
}

func TestSupportInboxServiceCompleteEmailSenderForwardingVerificationIgnoresMissingToken(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	senderRepo := repository.NewSupportEmailSenderRepository(db)
	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
		SetRouteDomain("on.helpin.email")

	for _, token := range []string{"", "verify-", "verify-missing"} {
		sender, verified, err := svc.CompleteEmailSenderForwardingVerification(ctx, token, model.PostmarkInboundPayload{To: "support@example.com"})
		if err != nil {
			t.Fatalf("complete forwarding verification for token %q: %v", token, err)
		}
		if sender != nil || verified {
			t.Fatalf("expected no-op for token %q, got sender=%#v verified=%v", token, sender, verified)
		}
	}
}

func TestSupportInboxServiceListEmailSendersBackfillsForwardingVerification(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	senderRepo := repository.NewSupportEmailSenderRepository(db)
	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
		SetRouteDomain("on.helpin.email")

	sender := verifiedSender(workspaceID, nil, "support@example.com", "none", actorID)
	sender.ForwardingVerificationToken = ""
	sender.ForwardingAddress = ""
	sender.ForwardingStatus = supportEmailSenderForwardingNotStarted
	if err := senderRepo.Create(ctx, sender); err != nil {
		t.Fatalf("create sender: %v", err)
	}

	senders, err := svc.ListEmailSenders(ctx, workspaceID)
	if err != nil {
		t.Fatalf("list senders: %v", err)
	}
	if len(senders) != 1 {
		t.Fatalf("expected 1 sender, got %d", len(senders))
	}
	if senders[0].ForwardingVerificationToken == "" || senders[0].ForwardingAddress == "" {
		t.Fatalf("expected forwarding verification fields to be backfilled, got %#v", senders[0])
	}
	if senders[0].ForwardingStatus != supportEmailSenderForwardingPending {
		t.Fatalf("expected forwarding status pending, got %q", senders[0].ForwardingStatus)
	}

	reloaded, err := senderRepo.GetByID(ctx, workspaceID, sender.ID)
	if err != nil {
		t.Fatalf("reload sender: %v", err)
	}
	if reloaded == nil || reloaded.ForwardingVerificationToken != senders[0].ForwardingVerificationToken || reloaded.ForwardingAddress != senders[0].ForwardingAddress {
		t.Fatalf("expected backfill to persist, got %#v", reloaded)
	}
}

func TestSupportInboxServiceSetDefaultEmailSenderRequiresForwardingForMailboxOnly(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	mailboxRepo := repository.NewSupportMailboxRepository(db)
	senderRepo := repository.NewSupportEmailSenderRepository(db)
	svc := NewSupportInboxService(nil, mailboxRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
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

	sender := verifiedSender(workspaceID, &mailbox.ID, "billing@example.com", "none", actorID)
	sender.ForwardingStatus = supportEmailSenderForwardingPending
	if err := senderRepo.Create(ctx, sender); err != nil {
		t.Fatalf("create sender: %v", err)
	}

	if _, err := svc.SetDefaultEmailSender(ctx, workspaceID, sender.ID, model.SetSupportEmailSenderDefaultRequest{DefaultScope: supportEmailSenderDefaultScopeWorkspace}); err != nil {
		t.Fatalf("workspace default should not require forwarding verification: %v", err)
	}
	if _, err := svc.SetDefaultEmailSender(ctx, workspaceID, sender.ID, model.SetSupportEmailSenderDefaultRequest{DefaultScope: supportEmailSenderDefaultScopeMailbox, MailboxID: &mailbox.ID}); err == nil || !strings.Contains(err.Error(), "forwarding must be verified") {
		t.Fatalf("expected mailbox default to require forwarding verification, got %v", err)
	}

	sender.ForwardingStatus = supportEmailSenderForwardingVerified
	if err := senderRepo.Update(ctx, sender); err != nil {
		t.Fatalf("mark sender forwarding verified: %v", err)
	}
	if _, err := svc.SetDefaultEmailSender(ctx, workspaceID, sender.ID, model.SetSupportEmailSenderDefaultRequest{DefaultScope: supportEmailSenderDefaultScopeMailbox, MailboxID: &mailbox.ID}); err != nil {
		t.Fatalf("mailbox default should pass after forwarding verification: %v", err)
	}
}

func TestSupportInboxServiceCreateEmailSenderReusesExistingPostmarkDomain(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Acme", "acme", actorID)

	var postDomains, getDomains int
	postmarkClient := email.NewDomainClient("account-token")
	postmarkClient.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/domains":
			postDomains++
			return &http.Response{
				StatusCode: http.StatusUnprocessableEntity,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ErrorCode":300,"Message":"Domain already exists"}`)),
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/domains":
			getDomains++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"TotalCount": 1,
					"Domains": [{
						"ID": 123,
						"Name": "example.com",
						"ReturnPathDomain": "pm-bounces.example.com",
						"ReturnPathDomainCNAMEValue": "pm.mtasv.net",
						"DKIMHost": "pm._domainkey.example.com",
						"DKIMPendingTextValue": "k=rsa; p=test"
					}]
				}`)),
			}, nil
		default:
			t.Fatalf("unexpected postmark request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})})

	senderRepo := repository.NewSupportEmailSenderRepository(db)
	svc := NewSupportInboxService(nil, repository.NewSupportMailboxRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetEmailSenderRepository(senderRepo).
		SetPostmarkDomainClient(postmarkClient).
		SetRouteDomain("on.helpin.email")

	sender, err := svc.CreateEmailSender(ctx, workspaceID, model.CreateSupportEmailSenderRequest{
		Email:       "support@example.com",
		DisplayName: "Support",
	}, actorID)
	if err != nil {
		t.Fatalf("create sender should reuse existing postmark domain: %v", err)
	}
	if sender.PostmarkDomainID == nil || *sender.PostmarkDomainID != 123 {
		t.Fatalf("expected existing postmark domain id 123, got %#v", sender.PostmarkDomainID)
	}
	if postDomains != 1 || getDomains != 1 {
		t.Fatalf("expected one create attempt and one lookup, got post=%d get=%d", postDomains, getDomains)
	}
}

func TestInboundPayloadMentionsAddressMatchesExactEmailTokens(t *testing.T) {
	payload := model.PostmarkInboundPayload{
		To:      "Helpin Support <support@example.com>",
		Subject: "Forward support@example.com",
		Headers: []model.PostmarkHeader{
			{Name: "X-Forwarded-To", Value: "Billing <billing@example.com>, support@example.com"},
		},
	}
	if !inboundPayloadMentionsAddress(payload, "support@example.com") {
		t.Fatal("expected exact sender address to match")
	}

	payload.To = "not-support@example.com"
	payload.Subject = "Forwarding not-support@example.com"
	payload.Headers = []model.PostmarkHeader{{Name: "X-Forwarded-To", Value: "not-support@example.com"}}
	if inboundPayloadMentionsAddress(payload, "support@example.com") {
		t.Fatal("expected substring-only sender address to be rejected")
	}
}

func verifiedSender(workspaceID string, mailboxID *string, emailAddress, defaultScope, actorID string) *model.SupportEmailSender {
	localPart, domain, _ := strings.Cut(emailAddress, "@")
	return &model.SupportEmailSender{
		ID:                          strings.NewReplacer("@", "-", ".", "-").Replace(emailAddress),
		WorkspaceID:                 workspaceID,
		MailboxID:                   mailboxID,
		Email:                       emailAddress,
		LocalPart:                   localPart,
		Domain:                      domain,
		DisplayName:                 "Support",
		ReturnPathDomain:            "pm-bounces." + domain,
		ReturnPathDomainCNAMEValue:  "pm.mtasv.net",
		ReturnPathDomainVerified:    true,
		DKIMHost:                    "pm._domainkey." + domain,
		DKIMTextValue:               "k=rsa; p=test",
		DKIMVerified:                true,
		DomainStatus:                supportEmailSenderStatusVerified,
		ForwardingStatus:            supportEmailSenderForwardingNotStarted,
		ForwardingVerificationToken: "verifytoken-" + strings.NewReplacer("@", "-", ".", "-").Replace(emailAddress),
		ForwardingAddress:           "verify-verifytoken-" + strings.NewReplacer("@", "-", ".", "-").Replace(emailAddress) + "@on.helpin.email",
		VerificationStatus:          supportEmailSenderStatusVerified,
		DefaultScope:                defaultScope,
		Active:                      defaultScope != "none",
		CreatedByID:                 actorID,
	}
}
