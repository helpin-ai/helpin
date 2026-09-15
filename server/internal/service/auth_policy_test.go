package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestLocalSignupDoesNotClaimEmailVerification(t *testing.T) {
	db := newTestDB(t)
	users := repository.NewUserRepository(db)
	svc := NewAuthService(users, nil, nil, nil, repository.NewEmailVerificationTokenRepository(db), auth.NewJWTManager("test-secret"), nil, nil, "http://localhost:3000", nil)
	svc.SetEmailVerificationRequired(false)
	response, err := svc.Signup(context.Background(), model.SignupRequest{Email: "local@example.com", FullName: "Local Owner", Password: "a-strong-test-password"})
	if err != nil {
		t.Fatal(err)
	}
	if response.User.EmailVerified || response.User.EmailVerifiedAt != nil {
		t.Fatal("unverified address claimed verified")
	}
	if response.AccessToken == "" {
		t.Fatal("local signup did not return a session")
	}
	var count int64
	if err := db.Model(&model.EmailVerificationToken{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("local signup created a verification token")
	}
	if err := svc.ResendEmailVerification(context.Background(), response.User.ID); err == nil {
		t.Fatal("disabled verification claimed delivery")
	}
}

func TestEmailDependentActionsRequireMail(t *testing.T) {
	svc, _ := newAuthService(t)
	for _, email := range []string{"unknown@example.com", "known@example.com"} {
		if err := svc.ForgotPassword(context.Background(), model.ForgotPasswordRequest{Email: email}); err == nil {
			t.Fatal("missing sender claimed delivery")
		}
	}
	invites := &InviteService{}
	if _, err := invites.CreateInvitation(context.Background(), model.CreateInvitationRequest{}, "owner"); err == nil {
		t.Fatal("invitation admitted without mail")
	}
}
