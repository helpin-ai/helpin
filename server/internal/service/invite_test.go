package service

import (
	"context"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
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
		nil,
		userRepo,
		settingsRepo,
		stubInviteEmailSender{},
		"http://localhost:3000",
		jwtManager,
	)
	return svc, invitationRepo, workspaceRepo, userRepo, settingsRepo
}

// setupInviteTestData seeds a user, workspace, and workspace member (owner)
// into the test DB and returns their IDs.
func setupInviteTestData(t *testing.T, svc *InviteService) (ownerID, wsID string) {
	t.Helper()

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

func TestCreateInvitation_InvalidRole(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()
	req := model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "invitee@example.com",
		Role:        "superadmin",
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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	_, err := svc.CreateInvitation(ctx, req, ownerID)
	if err != nil {
		t.Fatalf("first CreateInvitation() error = %v", err)
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

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

	list, err := svc.ListInvitations(ctx, wsID, ownerID)
	if err != nil {
		t.Fatalf("ListInvitations() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListInvitations() returned %d invitations, want 2", len(list))
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	adminID := "admin-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedUser(t, db, adminID, "admin@example.com", "Admin User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")
	seedWorkspaceMember(t, db, "wm-002", wsID, adminID, "admin@example.com", "Admin User", "admin")

	ctx := context.Background()

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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
	if token == "" || token == resp.JoinURL {
		t.Fatalf("could not extract token from JoinURL %q", resp.JoinURL)
	}

	err = svc.AcceptInvitation(ctx, token, inviteeID)
	if err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}

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

func TestAcceptInvitationPreservesExistingOrganizationRole(t *testing.T) {
	tests := []struct {
		name         string
		existingRole string
		wantRole     string
	}{
		{name: "missing membership becomes member", wantRole: model.RoleMember},
		{name: "admin remains admin", existingRole: model.RoleAdmin, wantRole: model.RoleAdmin},
		{name: "owner remains owner", existingRole: model.RoleOwner, wantRole: model.RoleOwner},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			invitationRepo := repository.NewInvitationRepository(db)
			workspaceRepo := repository.NewWorkspaceRepository(db)
			organizationRepo := repository.NewOrganizationRepository(db)
			userRepo := repository.NewUserRepository(db)
			settingsRepo := repository.NewSettingsRepository(db)
			jwtManager := auth.NewJWTManager("test-secret")
			svc := NewInviteService(
				invitationRepo,
				workspaceRepo,
				organizationRepo,
				userRepo,
				settingsRepo,
				stubInviteEmailSender{},
				"http://localhost:3000",
				jwtManager,
			)

			const (
				ownerID   = "owner-001"
				inviteeID = "user-002"
				orgID     = "org-001"
				wsID      = "ws-001"
			)
			now := time.Now()
			seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
			seedUser(t, db, inviteeID, "invitee@example.com", "Invitee User", "hashed")
			mustExec(t, db, `INSERT INTO organizations (id, name, slug, owner_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
				orgID, "Test Organization", "test-org", ownerID, now, now)
			seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
			mustExec(t, db, `UPDATE workspaces SET organization_id = ? WHERE id = ?`, orgID, wsID)
			seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", model.RoleOwner)
			if tt.existingRole != "" {
				mustExec(t, db, `INSERT INTO organization_members (id, organization_id, user_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
					"om-002", orgID, inviteeID, tt.existingRole, now, now)
			}

			resp, err := svc.CreateInvitation(context.Background(), model.CreateInvitationRequest{
				WorkspaceID: wsID,
				Email:       "invitee@example.com",
				Role:        model.RoleMember,
			}, ownerID)
			if err != nil {
				t.Fatalf("CreateInvitation() error = %v", err)
			}
			token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")
			if err := svc.AcceptInvitation(context.Background(), token, inviteeID); err != nil {
				t.Fatalf("AcceptInvitation() error = %v", err)
			}

			gotRole, err := organizationRepo.GetMemberRole(context.Background(), orgID, inviteeID)
			if err != nil {
				t.Fatalf("GetMemberRole() error = %v", err)
			}
			if gotRole != tt.wantRole {
				t.Fatalf("organization role = %q, want %q", gotRole, tt.wantRole)
			}
		})
	}
}

func TestAcceptInvitation_EmailMismatch(t *testing.T) {
	db := newTestDB(t)
	invitationRepo := repository.NewInvitationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	err = svc.AcceptInvitation(ctx, token, inviteeID)
	if err != nil {
		t.Fatalf("first AcceptInvitation() error = %v", err)
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	err = svc.RevokeInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("RevokeInvitation() error = %v", err)
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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
		Email:       "revokee@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	err = svc.RevokeInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("first RevokeInvitation() error = %v", err)
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	err = svc.RevokeInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("RevokeInvitation() error = %v", err)
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "resend@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	originalToken := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

	err = svc.ResendInvitation(ctx, resp.ID, ownerID)
	if err != nil {
		t.Fatalf("ResendInvitation() error = %v", err)
	}

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

	ownerID := "owner-001"
	wsID := "ws-001"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hashed")
	seedWorkspace(t, db, wsID, "Test Workspace", "test-ws", ownerID)
	seedWorkspaceMember(t, db, "wm-001", wsID, ownerID, "owner@example.com", "Owner User", "owner")

	ctx := context.Background()

	resp, err := svc.CreateInvitation(ctx, model.CreateInvitationRequest{
		WorkspaceID: wsID,
		Email:       "newuser@example.com",
		Role:        "member",
	}, ownerID)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}

	token := strings.TrimPrefix(resp.JoinURL, "http://localhost:3000/join/")

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

			svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

	svc := NewInviteService(invitationRepo, workspaceRepo, nil, userRepo, settingsRepo, stubInviteEmailSender{}, "http://localhost:3000", jwtManager)

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

type stubInviteEmailSender struct{}

func (stubInviteEmailSender) SendInviteEmail(to, inviter, workspace, url string) error { return nil }
