package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// newInvitedOIDCEnv returns an invite-only server with a workspace owner and a
// pending invitation for invitee@example.com.
func newInvitedOIDCEnv(t *testing.T) (*instanceTestEnv, *InviteService) {
	t.Helper()
	env, invite := newOIDCInviteEnv(t, model.SignupModeInviteOnly)
	invite("invitee@example.com", "member")
	return env, nil
}

// newOIDCInviteEnv returns a server with the given signup policy, a workspace
// owner, and a function that invites an address to that workspace.
func newOIDCInviteEnv(t *testing.T, policy string, adminEmails ...string) (*instanceTestEnv, func(email, role string)) {
	t.Helper()
	env := newInstanceTestEnv(t, adminEmails...)
	env.auth.SetOIDCIdentityRepository(repository.NewUserOIDCIdentityRepository(env.db))
	first, err := env.signup(t, "owner@example.com")
	if err != nil {
		t.Fatalf("bootstrap owner: %v", err)
	}
	env.setPolicy(t, policy, "")
	seedWorkspace(t, env.db, "ws-1", "Team", "team", first.User.ID)
	seedWorkspaceMember(t, env.db, "wm-1", "ws-1", first.User.ID, "owner@example.com", "Owner", "owner")
	invites := NewInviteService(repository.NewInvitationRepository(env.db), repository.NewWorkspaceRepository(env.db), nil,
		repository.NewUserRepository(env.db), repository.NewSettingsRepository(env.db), nil, "http://localhost:5173", auth.NewJWTManager("test-secret"))
	env.auth.SetOIDCInvitations(invites)
	return env, func(email, role string) {
		t.Helper()
		if _, err := invites.CreateInvitation(context.Background(), model.CreateInvitationRequest{WorkspaceID: "ws-1", Email: email, Role: role}, first.User.ID); err != nil {
			t.Fatalf("create invitation: %v", err)
		}
	}
}

func workspaceRole(t *testing.T, env *instanceTestEnv, userID string) string {
	t.Helper()
	var role string
	if err := env.db.Raw("SELECT role FROM workspace_members WHERE workspace_id = 'ws-1' AND user_id = ? AND status = 'active'", userID).Scan(&role).Error; err != nil {
		t.Fatalf("load workspace role: %v", err)
	}
	return role
}

func ownedOrganizations(t *testing.T, env *instanceTestEnv, userID string) int64 {
	t.Helper()
	var n int64
	if err := env.db.Raw("SELECT COUNT(*) FROM organizations WHERE owner_id = ?", userID).Scan(&n).Error; err != nil {
		t.Fatalf("count organizations: %v", err)
	}
	return n
}

func TestSignInWithOIDCInviteeJoinsWithInvitedRoleUnderOpenSignup(t *testing.T) {
	env, invite := newOIDCInviteEnv(t, model.SignupModeOpen)
	invite("invitee@example.com", "admin")

	resp, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-invitee", "invitee@example.com"))
	if err != nil {
		t.Fatalf("SignInWithOIDC() error = %v", err)
	}
	if role := workspaceRole(t, env, resp.User.ID); role != "admin" {
		t.Fatalf("workspace role = %q, want the invited role admin", role)
	}
	if n := ownedOrganizations(t, env, resp.User.ID); n != 0 {
		t.Fatalf("organizations owned = %d, want 0: the invitation replaces the default organization", n)
	}
}

func TestSignInWithOIDCCreatesDefaultOrganizationWithoutInvitation(t *testing.T) {
	env, _ := newOIDCInviteEnv(t, model.SignupModeOpen)

	resp, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-plain", "plain@example.com"))
	if err != nil {
		t.Fatalf("SignInWithOIDC() error = %v", err)
	}
	if n := ownedOrganizations(t, env, resp.User.ID); n != 1 {
		t.Fatalf("organizations owned = %d, want the default one", n)
	}
}

func TestSignInWithOIDCReturningUserAcceptsLaterInvitation(t *testing.T) {
	env, invite := newOIDCInviteEnv(t, model.SignupModeOpen)
	first, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-1", "late@example.com"))
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}
	if role := workspaceRole(t, env, first.User.ID); role != "" {
		t.Fatalf("workspace role before the invitation = %q, want none", role)
	}
	invite("late@example.com", "admin")

	second, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-1", "late@example.com"))
	if err != nil {
		t.Fatalf("returning sign-in: %v", err)
	}
	if role := workspaceRole(t, env, second.User.ID); role != "admin" {
		t.Fatalf("workspace role after the next sign-in = %q, want admin", role)
	}
	if n := ownedOrganizations(t, env, second.User.ID); n != 1 {
		t.Fatalf("organizations owned = %d, want only the one from the first sign-in", n)
	}
}

func TestSignInWithOIDCInvitedEnvAdminBecomesServerAdmin(t *testing.T) {
	env, invite := newOIDCInviteEnv(t, model.SignupModeInviteOnly, "boss@example.com")
	invite("boss@example.com", "member")

	resp, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-boss", "boss@example.com"))
	if err != nil {
		t.Fatalf("SignInWithOIDC() error = %v", err)
	}
	if user := env.user(t, "boss@example.com"); !resp.User.IsServerAdmin || !user.IsServerAdmin {
		t.Fatalf("response admin = %v, stored admin = %v; want a HELPIN_ADMIN_EMAILS address to become server admin", resp.User.IsServerAdmin, user.IsServerAdmin)
	}
}

func TestSignInWithOIDCAdmitsInvitedPersonOnInviteOnlyServer(t *testing.T) {
	env, _ := newInvitedOIDCEnv(t)

	resp, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-invitee", "Invitee@Example.com"))
	if err != nil {
		t.Fatalf("SignInWithOIDC() error = %v", err)
	}
	if resp.User.IsServerAdmin {
		t.Fatal("an invited person must not become server admin")
	}
	var status string
	if err := env.db.Raw("SELECT status FROM workspace_invitations WHERE LOWER(email) = 'invitee@example.com'").Scan(&status).Error; err != nil || status != "accepted" {
		t.Fatalf("invitation status = %q (err %v), want accepted", status, err)
	}
	if role := workspaceRole(t, env, resp.User.ID); role != "member" {
		t.Fatalf("workspace role = %q, want the invited role member", role)
	}
	if n := countOIDCIdentities(t, env); n != 1 {
		t.Fatalf("identity rows = %d, want 1", n)
	}
}

func TestSignInWithOIDCRefusesUninvitedPersonOnInviteOnlyServer(t *testing.T) {
	env, _ := newInvitedOIDCEnv(t)

	_, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-stranger", "stranger@example.com"))
	if !errors.Is(err, ErrSignupInviteOnly) {
		t.Fatalf("SignInWithOIDC() error = %v, want ErrSignupInviteOnly", err)
	}
	if n := countOIDCIdentities(t, env); n != 0 {
		t.Fatalf("identity rows = %d, want 0", n)
	}
}

func TestSignInWithOIDCIgnoresExpiredInvitation(t *testing.T) {
	env, _ := newInvitedOIDCEnv(t)
	if err := env.db.Exec("UPDATE workspace_invitations SET expires_at = '2000-01-01 00:00:00'").Error; err != nil {
		t.Fatalf("expire invitation: %v", err)
	}

	_, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-invitee", "invitee@example.com"))
	if !errors.Is(err, ErrSignupInviteOnly) {
		t.Fatalf("SignInWithOIDC() error = %v, want ErrSignupInviteOnly", err)
	}
}

func TestSignInWithGoogleRefusesEmailLinkToOIDCAccount(t *testing.T) {
	env := newOIDCTestEnv(t)
	ctx := context.Background()
	if _, err := env.auth.SignInWithOIDC(ctx, oidcIdentity("sub-1", "shared@example.com")); err != nil {
		t.Fatalf("oidc sign-in: %v", err)
	}

	_, err := env.auth.SignInWithGoogle(ctx, GoogleIdentity{Subject: "g-1", Email: "shared@example.com", EmailVerified: true, FullName: "Shared"})
	if !errors.Is(err, ErrOIDCEmailInUse) {
		t.Fatalf("SignInWithGoogle() error = %v, want ErrOIDCEmailInUse", err)
	}
	if user := env.user(t, "shared@example.com"); user.GoogleSubject != nil {
		t.Fatalf("GoogleSubject = %q, want it left unlinked", *user.GoogleSubject)
	}
}

func TestSignInWithGoogleStillLinksPlainAccountByEmail(t *testing.T) {
	env := newOIDCTestEnv(t)
	if _, err := env.signup(t, "plain@example.com"); err != nil {
		t.Fatalf("signup: %v", err)
	}

	if _, err := env.auth.SignInWithGoogle(context.Background(), GoogleIdentity{Subject: "g-1", Email: "plain@example.com", EmailVerified: true, FullName: "Plain"}); err != nil {
		t.Fatalf("SignInWithGoogle() error = %v, want the existing email link to keep working", err)
	}
}

func TestCreateOIDCUserReturnsConcurrentWinner(t *testing.T) {
	env := newOIDCTestEnv(t)
	ctx := context.Background()
	winner, err := env.auth.SignInWithOIDC(ctx, oidcIdentity("sub-1", "race@example.com"))
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}

	// The loser passed its identity lookup before the winner committed, so it
	// reaches user creation and must resolve to the winner instead of failing.
	user, created, err := env.auth.createOIDCUser(ctx, testOIDCIssuer, "sub-1", oidcIdentity("sub-1", "race@example.com"))
	if err != nil || created || user.ID != winner.User.ID {
		t.Fatalf("createOIDCUser() = %+v, created=%v, %v; want the winner %s", user, created, err, winner.User.ID)
	}
	var users int64
	if err := env.db.Model(&model.User{}).Where("email = ?", "race@example.com").Count(&users).Error; err != nil || users != 1 {
		t.Fatalf("users with the email = %d (err %v), want 1", users, err)
	}
}
