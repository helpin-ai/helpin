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
