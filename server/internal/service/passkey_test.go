package service

import (
	"context"
	"errors"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	gwebauthn "github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	appwebauthn "github.com/helpin-ai/helpin/server/internal/webauthn"
)

type stubPasskeyWebAuthnClient struct {
	sessionInfo                 *appwebauthn.SessionInfo
	sessionInfoErr              error
	finishAuthenticationResp    gwebauthn.User
	finishAuthenticationCred    *gwebauthn.Credential
	finishAuthenticationErr     error
	finishAuthenticationUser    gwebauthn.User
	finishAuthenticationForUser *gwebauthn.Credential
	finishAuthenticationForErr  error
}

func (s *stubPasskeyWebAuthnClient) Configured() bool {
	return true
}

func (s *stubPasskeyWebAuthnClient) BeginRegistration(context.Context, gwebauthn.User, []protocol.CredentialDescriptor) (*protocol.CredentialCreation, string, error) {
	panic("unexpected BeginRegistration call")
}

func (s *stubPasskeyWebAuthnClient) BeginAuthentication(context.Context) (*protocol.CredentialAssertion, string, error) {
	panic("unexpected BeginAuthentication call")
}

func (s *stubPasskeyWebAuthnClient) BeginAuthenticationForUser(context.Context, gwebauthn.User) (*protocol.CredentialAssertion, string, error) {
	panic("unexpected BeginAuthenticationForUser call")
}

func (s *stubPasskeyWebAuthnClient) FinishRegistration(context.Context, gwebauthn.User, string, []byte) (*gwebauthn.Credential, error) {
	panic("unexpected FinishRegistration call")
}

func (s *stubPasskeyWebAuthnClient) FinishAuthentication(_ context.Context, _ string, _ []byte, _ gwebauthn.DiscoverableUserHandler) (gwebauthn.User, *gwebauthn.Credential, error) {
	return s.finishAuthenticationResp, s.finishAuthenticationCred, s.finishAuthenticationErr
}

func (s *stubPasskeyWebAuthnClient) FinishAuthenticationForUser(_ context.Context, user gwebauthn.User, _ string, _ []byte) (*gwebauthn.Credential, error) {
	s.finishAuthenticationUser = user
	return s.finishAuthenticationForUser, s.finishAuthenticationForErr
}

func (s *stubPasskeyWebAuthnClient) SessionInfo(context.Context, string) (*appwebauthn.SessionInfo, error) {
	return s.sessionInfo, s.sessionInfoErr
}

func newPasskeyServiceForTest(t *testing.T, encryptionKey []byte) (*PasskeyService, *repository.UserRepository, *repository.PasskeyRepository, *stubPasskeyWebAuthnClient, *gorm.DB) {
	t.Helper()

	db := newTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	jwtMgr := auth.NewJWTManager("test-secret")
	webauthnClient := &stubPasskeyWebAuthnClient{}

	svc := NewPasskeyService(userRepo, passkeyRepo, jwtMgr, webauthnClient, encryptionKey)
	return svc, userRepo, passkeyRepo, webauthnClient, db
}

func TestPasskeyAuthenticationBypassesTwoFactorForVerifiedCredential(t *testing.T) {
	t.Parallel()

	svc, userRepo, _, webauthnClient, db := newPasskeyServiceForTest(t, []byte("0123456789abcdef0123456789abcdef"))
	ctx := context.Background()

	user, err := userRepo.Create(ctx, "verified@example.com", "password-hash", "Verified User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	secret := "encrypted-secret"
	if _, err := userRepo.UpsertTwoFactor(ctx, user.ID, &secret, true, nil); err != nil {
		t.Fatalf("enable 2fa: %v", err)
	}

	credentialID := []byte("verified-passkey-credential")
	seedPasskey(t, db, "passkey-verified", user.ID, "Mac passkey", credentialID, []byte("public-key"), 0, false, 1)

	flags := gwebauthn.NewCredentialFlags(0)
	flags.UserVerified = true

	webauthnClient.sessionInfo = &appwebauthn.SessionInfo{
		Kind:   appwebauthn.SessionKindKnownUserLogin,
		UserID: []byte(user.ID),
	}
	webauthnClient.finishAuthenticationForUser = &gwebauthn.Credential{
		ID:    credentialID,
		Flags: flags,
		Authenticator: gwebauthn.Authenticator{
			SignCount: 9,
		},
	}

	resp, err := svc.FinishAuthentication(ctx, model.PasskeyAuthenticateRequest{
		Challenge:  "challenge",
		Credential: []byte(`{"id":"credential"}`),
		RememberMe: true,
	})
	if err != nil {
		t.Fatalf("finish authentication: %v", err)
	}
	if resp.Requires2FA {
		t.Fatal("did not expect an extra 2fa challenge for a verified passkey")
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected direct auth tokens for verified passkey authentication")
	}
	claims, err := svc.jwtManager.ValidateToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if !claims.MFASatisfied {
		t.Fatal("expected verified passkey auth to satisfy MFA in access token")
	}
	if resp.User == nil || resp.User.Email != user.Email {
		t.Fatalf("expected authenticated user %q, got %+v", user.Email, resp.User)
	}
	if webauthnClient.finishAuthenticationUser == nil {
		t.Fatal("expected FinishAuthenticationForUser to receive the resolved user")
	}

	storedPasskey, err := svc.passkeyRepo.GetByID(ctx, user.ID, "passkey-verified")
	if err != nil {
		t.Fatalf("reload passkey: %v", err)
	}
	if storedPasskey == nil {
		t.Fatal("expected stored passkey")
	}
	if !storedPasskey.Verified {
		t.Fatal("expected passkey verified flag to be updated after authentication")
	}
	if storedPasskey.SignCount != 9 {
		t.Fatalf("expected sign_count to be updated to 9, got %d", storedPasskey.SignCount)
	}
}

func TestPasskeyAuthenticationRequiresTwoFactorWhenCredentialNotVerified(t *testing.T) {
	t.Parallel()

	svc, userRepo, _, webauthnClient, db := newPasskeyServiceForTest(t, []byte("0123456789abcdef0123456789abcdef"))
	ctx := context.Background()

	user, err := userRepo.Create(ctx, "totp@example.com", "password-hash", "Needs TOTP")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	secret := "encrypted-secret"
	if _, err := userRepo.UpsertTwoFactor(ctx, user.ID, &secret, true, nil); err != nil {
		t.Fatalf("enable 2fa: %v", err)
	}

	credentialID := []byte("totp-passkey-credential")
	seedPasskey(t, db, "passkey-totp", user.ID, "Windows passkey", credentialID, []byte("public-key"), 0, false, 2)

	webauthnClient.sessionInfo = &appwebauthn.SessionInfo{
		Kind:   appwebauthn.SessionKindKnownUserLogin,
		UserID: []byte(user.ID),
	}
	webauthnClient.finishAuthenticationForUser = &gwebauthn.Credential{
		ID: credentialID,
		Authenticator: gwebauthn.Authenticator{
			SignCount: 4,
		},
	}

	resp, err := svc.FinishAuthentication(ctx, model.PasskeyAuthenticateRequest{
		Challenge:  "challenge",
		Credential: []byte(`{"id":"credential"}`),
		RememberMe: true,
	})
	if err != nil {
		t.Fatalf("finish authentication: %v", err)
	}
	if !resp.Requires2FA {
		t.Fatal("expected a 2fa challenge when the passkey did not provide user verification")
	}
	if resp.TwoFAToken == "" {
		t.Fatal("expected a 2fa token")
	}
	if resp.AccessToken != "" || resp.RefreshToken != "" {
		t.Fatal("did not expect full auth tokens before 2fa completion")
	}

	storedPasskey, err := svc.passkeyRepo.GetByID(ctx, user.ID, "passkey-totp")
	if err != nil {
		t.Fatalf("reload passkey: %v", err)
	}
	if storedPasskey == nil {
		t.Fatal("expected stored passkey")
	}
	if storedPasskey.Verified {
		t.Fatal("did not expect passkey to be marked verified")
	}
	if storedPasskey.SignCount != 4 {
		t.Fatalf("expected sign_count to be updated to 4, got %d", storedPasskey.SignCount)
	}
}

func TestPasskeyBeginAuthenticationWithEmailHintRequiresRegisteredPasskey(t *testing.T) {
	t.Parallel()

	svc, userRepo, _, _, _ := newPasskeyServiceForTest(t, []byte("0123456789abcdef0123456789abcdef"))
	ctx := context.Background()

	if _, err := userRepo.Create(ctx, "nopasskey@example.com", "password-hash", "No Passkey"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, err := svc.BeginAuthentication(ctx, model.PasskeyAuthenticationOptionsRequest{
		EmailHint: ptr("nopasskey@example.com"),
	})
	if err == nil {
		t.Fatal("expected email-hinted authentication to fail without a registered passkey")
	}
	if !errors.Is(err, ErrNoPasskeyForAccount) {
		t.Fatalf("expected ErrNoPasskeyForAccount, got %v", err)
	}
}
