//go:build ee

package service

import (
	"context"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"strings"
	"testing"
)

func TestCreateInvitation_LockedWorkspaceBlocksNewInvite(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	billingRepo := eerepository.NewBillingRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	createInviteBillingTable(t, db)

	billingService := NewBillingService(billingRepo, nil, nil)
	billingService.SetWorkspaceRepository(workspaceRepo)
	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)
	svc.SetBillingService(billingService)

	ownerID := "owner-001"
	memberID := "member-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, memberID, "member@example.com", "Member User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, memberID, "member@example.com", "Member User", "member")
	mustExec(t, db, `INSERT INTO workspace_billing (id, workspace_id, plan, status, billing_interval, included_credits, credits_used, current_period_start, current_period_end, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, datetime('now', '+1 month'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"billing-001", wsID, model.BillingPlanGrowth, model.BillingStatusTrialExpired, "monthly", 25000, 0)

	_, err := svc.CreateInvitation(context.Background(), model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "third@example.com",
		Role:        "member",
	}, ownerID)
	if err == nil || !strings.Contains(err.Error(), "workspace is locked") {
		t.Fatalf("CreateInvitation() error = %v, want locked workspace error", err)
	}
}

func TestCreateInvitation_ActivePaidPlanAllowsAdditionalSeats(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	billingRepo := eerepository.NewBillingRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	createInviteBillingTable(t, db)

	billingService := NewBillingService(billingRepo, nil, nil)
	billingService.SetWorkspaceRepository(workspaceRepo)
	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)
	svc.SetBillingService(billingService)

	ownerID := "owner-001"
	memberID := "member-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, memberID, "member@example.com", "Member User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, memberID, "member@example.com", "Member User", "member")
	mustExec(t, db, `INSERT INTO workspace_billing (id, workspace_id, plan, status, billing_interval, included_credits, credits_used, current_period_start, current_period_end, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, datetime('now', '+1 month'), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"billing-001", wsID, model.BillingPlanStarter, model.BillingStatusActive, "monthly", 5000, 0)

	resp, err := svc.CreateInvitation(context.Background(), model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "third@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if resp.Email != "third@example.com" {
		t.Fatalf("Email = %q, want third@example.com", resp.Email)
	}
}
