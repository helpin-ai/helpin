package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// newInviteService is a test helper that wires up InviteService with real
// repositories backed by the in-memory SQLite test database.
func newInviteService(t *testing.T) (*InviteService, *repository.InvitationRepository, *repository.WorkspaceRepository, *repository.UserRepository, *repository.SettingsRepository) {
	t.Helper()
	db := newTestDB(t)

	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(
		invitationRepo,
		workspaceRepo,
		nil, // organizationRepo — not needed for tests
		userRepo,
		settingsRepo,
		nil, // emailClient — not needed for tests
		"http://localhost:3000",
		jwtManager,
	)
	return svc, invitationRepo, workspaceRepo, userRepo, settingsRepo
}

// setupInviteTestData seeds a user, workspace, and workspace member (owner)
// into the test DB and returns their IDs.
func setupInviteTestData(t *testing.T, svc *InviteService) (ownerID, wsID string) {
	t.Helper()
	// We need the underlying DB; extract it from the workspace repo via a
	// small trick: create the DB independently and pass it in.  Instead,
	// let's just use the helper at test-call sites.
	return "", ""
}

func createInviteBillingTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	mustExec(t, db, `CREATE TABLE workspace_billing (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL UNIQUE,
		plan TEXT NOT NULL DEFAULT 'free',
		status TEXT NOT NULL DEFAULT 'active',
		stripe_customer_id TEXT,
		stripe_subscription_id TEXT,
		stripe_price_id TEXT,
		billing_interval TEXT NOT NULL DEFAULT 'monthly',
		included_credits INTEGER NOT NULL DEFAULT 1000,
		credits_used INTEGER NOT NULL DEFAULT 0,
		on_demand_enabled BOOLEAN NOT NULL DEFAULT 0,
		on_demand_blocks_invoiced INTEGER NOT NULL DEFAULT 0,
		current_period_start DATETIME NOT NULL,
		current_period_end DATETIME NOT NULL,
		trial_ends_at DATETIME,
		pending_plan TEXT,
		pending_billing_interval TEXT,
		pending_change_at DATETIME,
		cancel_at_period_end BOOLEAN NOT NULL DEFAULT 0,
		canceled_at DATETIME,
		billing_notice_type TEXT,
		billing_notice_message TEXT,
		billing_notice_at DATETIME,
		payment_failed_at DATETIME,
		trial_will_end_at DATETIME,
		last_stripe_event_id TEXT,
		payment_method_id TEXT,
		billing_owner_user_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
}

func TestCreateInvitation(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()
	req := model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "invitee@example.com",
		Role:        "member",
	}

	resp, err := svc.CreateInvitation(ctx, req, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	if resp.Email != "invitee@example.com" {
		t.Errorf("Email = %q, want %q", resp.Email, "invitee@example.com")
	}
	if resp.Role != "member" {
		t.Errorf("Role = %q, want %q", resp.Role, "member")
	}
	if resp.Status != "pending" {
		t.Errorf("Status = %q, want %q", resp.Status, "pending")
	}
	if resp.WorkspaceID != wsID {
		t.Errorf("WorkspaceID = %q, want %q", resp.WorkspaceID, wsID)
	}
	if resp.InvitedBy != ownerID {
		t.Errorf("InvitedBy = %q, want %q", resp.InvitedBy, ownerID)
	}
	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.JoinURL == "" {
		t.Error("JoinURL should not be empty")
	}
	if !strings.HasPrefix(resp.JoinURL, "http://localhost:3000/join/") {
		t.Errorf("JoinURL = %q, want prefix %q", resp.JoinURL, "http://localhost:3000/join/")
	}
	if resp.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}
	if resp.WorkspaceMemberID == nil {
		t.Error("WorkspaceMemberID should not be nil (pending member should be created)")
	}
}

func TestCreateInvitation_LockedWorkspaceBlocksNewInvite(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	createInviteBillingTable(t, db)

	billingService := NewBillingService(billingRepo, nil, nil)
	billingService.SetWorkspaceRepository(workspaceRepo)
	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)
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
	billingRepo := repository.NewBillingRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	createInviteBillingTable(t, db)

	billingService := NewBillingService(billingRepo, nil, nil)
	billingService.SetWorkspaceRepository(workspaceRepo)
	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)
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

func TestCreateInvitation_InvalidRole(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()
	req := model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "invitee@example.com",
		Role:        "superadmin", // invalid
	}

	_, err := svc.CreateInvitation(ctx, req, ownerID)
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "invalid role")
	}
}

func TestCreateInvitation_DuplicateEmail(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()
	req := model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "duplicate@example.com",
		Role:        "member",
	}

	// First invitation succeeds
	_, err := svc.CreateInvitation(ctx, req, ownerID)
	if err != nil {
		t.Fatalf("first CreateInvitation() error = %v", err)
	}

	// Second invitation for same email should fail
	_, err = svc.CreateInvitation(ctx, req, ownerID)
	if err == nil {
		t.Fatal("expected error for duplicate invitation, got nil")
	}
	if !strings.Contains(err.Error(), "pending invitation already exists") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "pending invitation already exists")
	}
}

func TestCreateInvitation_AlreadyMember(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	existingUserID := "user-002"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, existingUserID, "existing@example.com", "Existing User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, existingUserID, "existing@example.com", "Existing User", "member")

	ctx := context.Background()
	req := model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "existing@example.com",
		Role:        "member",
	}

	_, err := svc.CreateInvitation(ctx, req, ownerID)
	if err == nil {
		t.Fatal("expected error for already-a-member email, got nil")
	}
	if !strings.Contains(err.Error(), "already a member") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "already a member")
	}
}

func TestCreateInvitation_EmailNormalization(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()
	req := model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "  UPPER@EXAMPLE.COM  ",
		Role:        "member",
	}

	resp, err := svc.CreateInvitation(ctx, req, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if resp.Email != "upper@example.com" {
		t.Errorf("Email = %q, want %q (should be lowercased and trimmed)", resp.Email, "upper@example.com")
	}
}

func TestListInvitations(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	// Create two invitations
	_, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "alice@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation(alice) error = %v", err)
	}

	_, err = svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "bob@example.com",
		Role:        "admin",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation(bob) error = %v", err)
	}

	// List as owner (should succeed)
	list, err := svc.ListInvitations(ctx, wsID, ownerID)
	if err != nil {
		t.Fatalf("ListInvitations() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListInvitations() returned %d invitations, want 2", len(list))
	}

	// Verify both emails are present
	emails := map[string]bool{}
	for _, inv := range list {
		emails[inv.Email] = true
	}
	if !emails["alice@example.com"] {
		t.Error("expected alice@example.com in list")
	}
	if !emails["bob@example.com"] {
		t.Error("expected bob@example.com in list")
	}
}

func TestListInvitations_NonAdminForbidden(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	memberID := "member-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, memberID, "member@example.com", "Member User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, memberID, "member@example.com", "Member User", "member")

	ctx := context.Background()

	_, err := svc.ListInvitations(ctx, wsID, memberID)
	if err == nil {
		t.Fatal("expected error for non-admin listing invitations, got nil")
	}
	if !strings.Contains(err.Error(), "only admins") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "only admins")
	}
}

func TestListInvitations_AdminAllowed(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	adminID := "admin-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, adminID, "admin@example.com", "Admin User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, adminID, "admin@example.com", "Admin User", "admin")

	ctx := context.Background()

	// Admin should be able to list invitations
	list, err := svc.ListInvitations(ctx, wsID, adminID)
	if err != nil {
		t.Fatalf("ListInvitations() as admin error = %v", err)
	}
	if list == nil {
		t.Error("ListInvitations() returned nil, want empty slice")
	}
}

func TestAcceptInvitation(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	inviteeID := "user-002"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, inviteeID, "invitee@example.com", "Invitee User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	// Create an invitation
	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "invitee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	// Extract token from JoinURL
	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")
	if token == "" || token == resp.JoinURL {
		t.Fatalf("could not extract token from JoinURL %q", resp.JoinURL)
	}

	// Accept the invitation
	err = svc.AcceptInvitation(ctx, token, inviteeID)
	if err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}

	// Verify the invitation status is now "accepted"
	inv, err := invitationRepo.GetByToken(ctx, token)
	if err != nil {
		t.Fatalf("GetByToken() error = %v", err)
	}
	if inv == nil {
		t.Fatal("invitation should still exist after acceptance")
	}
	if inv.Status != "accepted" {
		t.Errorf("Status = %q, want %q", inv.Status, "accepted")
	}
	if inv.AcceptedAt == nil {
		t.Error("AcceptedAt should not be nil after acceptance")
	}
}

func TestAcceptInvitation_EmailMismatch(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wrongUserID := "user-003"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, wrongUserID, "wrong@example.com", "Wrong User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "invitee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	// Try to accept with a user whose email doesn't match
	err = svc.AcceptInvitation(ctx, token, wrongUserID)
	if err == nil {
		t.Fatal("expected error for email mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "does not match")
	}
}

func TestAcceptInvitation_AlreadyAccepted(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	inviteeID := "user-002"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, inviteeID, "invitee@example.com", "Invitee User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "invitee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	// Accept once
	err = svc.AcceptInvitation(ctx, token, inviteeID)
	if err != nil {
		t.Fatalf("first AcceptInvitation() error = %v", err)
	}

	// Accept again should fail
	err = svc.AcceptInvitation(ctx, token, inviteeID)
	if err == nil {
		t.Fatal("expected error for already-accepted invitation, got nil")
	}
	if !strings.Contains(err.Error(), "no longer pending") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "no longer pending")
	}
}

func TestAcceptInvitation_InvalidToken(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ctx := context.Background()

	err := svc.AcceptInvitation(ctx, "nonexistent-token", "user-001")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found")
	}
}

func TestRevokeInvitation(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	// Create an invitation
	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "revokee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	// Revoke it
	err = svc.RevokeInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("RevokeInvitation() error = %v", err)
	}

	// Verify the invitation status is "revoked"
	inv, err := invitationRepo.GetByID(ctx, resp.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if inv == nil {
		t.Fatal("invitation should still exist after revocation")
	}
	if inv.Status != "revoked" {
		t.Errorf("Status = %q, want %q", inv.Status, "revoked")
	}

	// Verify it no longer appears as pending in a list
	list, err := svc.ListInvitations(ctx, wsID, ownerID)
	if err != nil {
		t.Fatalf("ListInvitations() error = %v", err)
	}
	for _, item := range list {
		if item.ID == resp.ID && item.Status == "pending" {
			t.Error("revoked invitation should not have status=pending in list")
		}
	}
}

func TestRevokeInvitation_NonAdminForbidden(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	memberID := "member-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, memberID, "member@example.com", "Member User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, memberID, "member@example.com", "Member User", "member")

	ctx := context.Background()

	// Create invitation as owner
	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "revokee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	// Try to revoke as non-admin member
	err = svc.RevokeInvitation(ctx, resp.ID, memberID)
	if err == nil {
		t.Fatal("expected error for non-admin revoking invitation, got nil")
	}
	if !strings.Contains(err.Error(), "only admins") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "only admins")
	}
}

func TestRevokeInvitation_AlreadyRevoked(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "revokee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	// Revoke once
	err = svc.RevokeInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("first RevokeInvitation() error = %v", err)
	}

	// Revoke again should fail
	err = svc.RevokeInvitation(ctx, resp.ID, ownerID)
	if err == nil {
		t.Fatal("expected error for revoking already-revoked invitation, got nil")
	}
	if !strings.Contains(err.Error(), "only revoke pending") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "only revoke pending")
	}
}

func TestRevokeInvitation_ThenCannotAccept(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	inviteeID := "user-002"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, inviteeID, "invitee@example.com", "Invitee User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "invitee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	// Revoke
	err = svc.RevokeInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("RevokeInvitation() error = %v", err)
	}

	// Try to accept the revoked invitation
	err = svc.AcceptInvitation(ctx, token, inviteeID)
	if err == nil {
		t.Fatal("expected error for accepting revoked invitation, got nil")
	}
	if !strings.Contains(err.Error(), "no longer pending") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "no longer pending")
	}
}

func TestResendInvitation(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	// Create an invitation
	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "resend@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	originalToken := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	// Resend
	err = svc.ResendInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("ResendInvitation() error = %v", err)
	}

	// The token should have changed
	inv, err := invitationRepo.GetByID(ctx, resp.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if inv.Token == originalToken {
		t.Error("token should have been regenerated after resend")
	}
	if inv.Status != "pending" {
		t.Errorf("Status = %q, want %q after resend", inv.Status, "pending")
	}
}

func TestResendInvitation_NonAdminForbidden(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	memberID := "member-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, memberID, "member@example.com", "Member User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, memberID, "member@example.com", "Member User", "member")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "resend@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	err = svc.ResendInvitation(ctx, resp.ID, memberID)
	if err == nil {
		t.Fatal("expected error for non-admin resending invitation, got nil")
	}
	if !strings.Contains(err.Error(), "only admins") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "only admins")
	}
}

func TestGetInviteInfo(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "info@example.com",
		Role:        "admin",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	info, err := svc.GetInviteInfo(ctx, token)
	if err != nil {
		t.Fatalf("GetInviteInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetInviteInfo() returned nil")
	}
	if info.WorkspaceName != "Test Workspace" {
		t.Errorf("WorkspaceName = %q, want %q", info.WorkspaceName, "Test Workspace")
	}
	if info.WorkspaceSlug != "test-ws" {
		t.Errorf("WorkspaceSlug = %q, want %q", info.WorkspaceSlug, "test-ws")
	}
	if info.Email != "info@example.com" {
		t.Errorf("Email = %q, want %q", info.Email, "info@example.com")
	}
	if info.AccountExists {
		t.Error("AccountExists should be false for an invitee without an account")
	}
	if info.Role != "admin" {
		t.Errorf("Role = %q, want %q", info.Role, "admin")
	}
	if info.InvitedByName != "Owner User" {
		t.Errorf("InvitedByName = %q, want %q", info.InvitedByName, "Owner User")
	}
	if info.Status != "pending" {
		t.Errorf("Status = %q, want %q", info.Status, "pending")
	}
	if info.Expired {
		t.Error("Expired should be false for a fresh invitation")
	}
}

func TestGetInviteInfo_InvalidToken(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ctx := context.Background()

	_, err := svc.GetInviteInfo(ctx, "bogus-token")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found")
	}
}

func TestAcceptInvitationWithSignup(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	// Create invitation for a brand new user
	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "newuser@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	// Accept with signup
	signupResp, err := svc.AcceptInvitationWithSignup(ctx, model.AcceptInvitationWithSignupRequest{
		Token:    token,
		Password: "securepassword123",
		FullName: "New User",
	})
	if err != nil {
		t.Fatalf("AcceptInvitationWithSignup() error = %v", err)
	}

	if signupResp.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if signupResp.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}
	if signupResp.User.Email != "newuser@example.com" {
		t.Errorf("User.Email = %q, want %q", signupResp.User.Email, "newuser@example.com")
	}
	if signupResp.User.FullName != "New User" {
		t.Errorf("User.FullName = %q, want %q", signupResp.User.FullName, "New User")
	}
	if signupResp.WorkspaceSlug != "test-ws" {
		t.Errorf("WorkspaceSlug = %q, want %q", signupResp.WorkspaceSlug, "test-ws")
	}

	// Verify the invitation was marked as accepted
	inv, err := invitationRepo.GetByToken(ctx, token)
	if err != nil {
		t.Fatalf("GetByToken() error = %v", err)
	}
	if inv.Status != "accepted" {
		t.Errorf("Status = %q, want %q", inv.Status, "accepted")
	}
}

func TestAcceptInvitationWithSignup_ShortPassword(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ctx := context.Background()

	_, err := svc.AcceptInvitationWithSignup(ctx, model.AcceptInvitationWithSignupRequest{
		Token:    "some-token",
		Password: "short",
		FullName: "User",
	})
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}
	if !strings.Contains(err.Error(), "at least 8 characters") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "at least 8 characters")
	}
}

func TestAcceptInvitationWithSignup_EmptyName(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ctx := context.Background()

	_, err := svc.AcceptInvitationWithSignup(ctx, model.AcceptInvitationWithSignupRequest{
		Token:    "some-token",
		Password: "securepassword123",
		FullName: "   ",
	})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !strings.Contains(err.Error(), "full name is required") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "full name is required")
	}
}

func TestAcceptInvitationWithSignup_EmptyToken(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ctx := context.Background()

	_, err := svc.AcceptInvitationWithSignup(ctx, model.AcceptInvitationWithSignupRequest{
		Token:    "",
		Password: "securepassword123",
		FullName: "User",
	})
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "token is required")
	}
}

func TestAcceptInvitationWithSignup_ExistingUser(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, "existing-user", "existing@example.com", "Existing User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "existing@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	// Try to sign up with an email that already has an account
	_, err = svc.AcceptInvitationWithSignup(ctx, model.AcceptInvitationWithSignupRequest{
		Token:    token,
		Password: "securepassword123",
		FullName: "Existing User",
	})
	if err == nil {
		t.Fatal("expected error for existing user signup, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "already exists")
	}

	info, err := svc.GetInviteInfo(ctx, token)
	if err != nil {
		t.Fatalf("GetInviteInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetInviteInfo() returned nil")
	}
	if !info.AccountExists {
		t.Error("AccountExists should be true for an invitee with an existing account")
	}
}

func TestCreateInvitation_MultipleRoles(t *testing.T) {
	validRoles := []string{"admin", "manager", "member", "viewer"}

	for _, role := range validRoles {
		t.Run(role, func(t *testing.T) {
			db := newTestDB(t)
			invitationRepo := repository.NewInvitationRepository(db)
			workspaceRepo := repository.NewWorkspaceRepository(db)
			userRepo := repository.NewUserRepository(db)
			settingsRepo := repository.NewSettingsRepository(db)
			jwtManager := auth.NewJWTManager("test-secret")

			svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

			ownerID := "owner-001"
			wsID := "ws-001"

			seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
			seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
			seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

			ctx := context.Background()
			resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
				WorkspaceID: wsID,
				Email:       "user-" + role + "@example.com",
				Role:        role,
			}, ownerID)
			if err != nil {
				t.Fatalf("CreateInvitation(role=%s) error = %v", role, err)
			}
			if resp.Role != role {
				t.Errorf("Role = %q, want %q", resp.Role, role)
			}
		})
	}
}

func TestCreateInvitation_OwnerRoleForbidden(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, nil, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()
	_, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "newowner@example.com",
		Role:        "owner",
	}, ownerID)
	if err == nil {
		t.Fatal("expected error for owner role invitation, got nil")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "invalid role")
	}
}
