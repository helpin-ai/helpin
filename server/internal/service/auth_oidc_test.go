package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const testOIDCIssuer = "https://auth.example.com"

func newOIDCTestEnv(t *testing.T) *instanceTestEnv {
	t.Helper()
	env := newInstanceTestEnv(t)
	env.auth.SetOIDCIdentityRepository(repository.NewUserOIDCIdentityRepository(env.db))
	return env
}

func oidcIdentity(subject, email string) OIDCIdentity {
	return OIDCIdentity{Issuer: testOIDCIssuer, Subject: subject, Email: email, EmailVerified: true, FullName: "Oidc Person"}
}

func countOIDCIdentities(t *testing.T, env *instanceTestEnv) int64 {
	t.Helper()
	var n int64
	if err := env.db.Model(&model.UserOIDCIdentity{}).Count(&n).Error; err != nil {
		t.Fatalf("count oidc identities: %v", err)
	}
	return n
}

func TestSignInWithOIDCCreatesVerifiedUserAndIdentity(t *testing.T) {
	env := newOIDCTestEnv(t)

	resp, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-1", "Alice@Example.com"))
	if err != nil {
		t.Fatalf("SignInWithOIDC() error = %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("SignInWithOIDC() issued no tokens")
	}
	if resp.User.Email != "alice@example.com" || !resp.User.EmailVerified {
		t.Fatalf("user = %+v, want verified alice@example.com", resp.User)
	}
	var link model.UserOIDCIdentity
	if err := env.db.Where("issuer = ? AND subject = ?", testOIDCIssuer, "sub-1").First(&link).Error; err != nil {
		t.Fatalf("load identity: %v", err)
	}
	if link.UserID != resp.User.ID {
		t.Fatalf("identity user_id = %q, want %q", link.UserID, resp.User.ID)
	}
}

func TestSignInWithOIDCReturningUserMatchesBySubjectNotEmail(t *testing.T) {
	env := newOIDCTestEnv(t)
	first, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-1", "alice@example.com"))
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}

	second, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-1", "renamed@example.com"))
	if err != nil {
		t.Fatalf("returning sign-in: %v", err)
	}
	if second.User.ID != first.User.ID {
		t.Fatalf("returning user id = %q, want %q", second.User.ID, first.User.ID)
	}
	var users int64
	if err := env.db.Model(&model.User{}).Count(&users).Error; err != nil || users != 1 {
		t.Fatalf("user count = %d (err %v), want 1", users, err)
	}
}

func TestSignInWithOIDCRefusesEmailOfUnlinkedAccount(t *testing.T) {
	env := newOIDCTestEnv(t)
	existing, err := env.signup(t, "taken@example.com")
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	var logs bytes.Buffer
	env.auth.logger = slog.New(slog.NewTextHandler(&logs, nil))

	_, err = env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-evil", "taken@example.com"))
	if !errors.Is(err, ErrOIDCEmailInUse) {
		t.Fatalf("SignInWithOIDC() error = %v, want ErrOIDCEmailInUse", err)
	}
	if n := countOIDCIdentities(t, env); n != 0 {
		t.Fatalf("identity rows = %d, want 0: email must never link accounts", n)
	}
	for _, want := range []string{testOIDCIssuer, "sub-evil", existing.User.ID} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("warn log %q does not contain %q", logs.String(), want)
		}
	}
	if strings.Contains(logs.String(), "taken@example.com") {
		t.Fatalf("warn log leaks the email address: %q", logs.String())
	}
}

func TestSignInWithOIDCAppliesSignupPolicy(t *testing.T) {
	env := newOIDCTestEnv(t)
	if _, err := env.signup(t, "admin@example.com"); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	env.setPolicy(t, model.SignupModeInviteOnly, "")
	if _, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-1", "new@example.com")); !errors.Is(err, ErrSignupInviteOnly) {
		t.Fatalf("invite-only error = %v, want ErrSignupInviteOnly", err)
	}

	env.setPolicy(t, model.SignupModeDomains, "corp.example")
	if _, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-2", "new@other.example")); !errors.Is(err, ErrSignupDomainNotAllowed) {
		t.Fatalf("wrong-domain error = %v, want ErrSignupDomainNotAllowed", err)
	}
	if n := countOIDCIdentities(t, env); n != 0 {
		t.Fatalf("identity rows after refusals = %d, want 0", n)
	}

	resp, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-3", "new@corp.example"))
	if err != nil {
		t.Fatalf("allowed-domain sign-in: %v", err)
	}
	if resp.User.Email != "new@corp.example" {
		t.Fatalf("user email = %q", resp.User.Email)
	}
}

func TestSignInWithOIDCNewUserNeedsVerifiedEmail(t *testing.T) {
	env := newOIDCTestEnv(t)

	unverified := oidcIdentity("sub-1", "alice@example.com")
	unverified.EmailVerified = false
	if _, err := env.auth.SignInWithOIDC(context.Background(), unverified); err == nil {
		t.Fatal("unverified email created an account")
	}
	if _, err := env.auth.SignInWithOIDC(context.Background(), oidcIdentity("sub-2", "")); err == nil {
		t.Fatal("empty email created an account")
	}
	var users int64
	if err := env.db.Model(&model.User{}).Count(&users).Error; err != nil || users != 0 {
		t.Fatalf("user count = %d (err %v), want 0", users, err)
	}
}
