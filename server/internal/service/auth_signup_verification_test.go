package service

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type authSignupEmailSpy struct {
	verificationTo  string
	verificationURL string
}

func (s *authSignupEmailSpy) SendPasswordResetEmail(to, fullName, resetURL string) error {
	return nil
}

func (s *authSignupEmailSpy) SendVerificationEmail(to, fullName, verificationURL string) error {
	s.verificationTo = to
	s.verificationURL = verificationURL
	return nil
}

func TestAuthServiceSignupCreatesUnverifiedUserAndSendsVerification(t *testing.T) {
	svc, db, emailSpy := newAuthSignupTestService(t)

	resp, err := svc.Signup(context.Background(), model.SignupRequest{
		Email:    "founder@example.com",
		Password: "strongpass1",
		FullName: "Founder Example",
	})
	if err != nil {
		t.Fatalf("Signup() error = %v", err)
	}
	if resp.User.EmailVerified {
		t.Fatal("new password signup should not be email verified immediately")
	}
	if emailSpy.verificationTo != "founder@example.com" {
		t.Fatalf("verification email to = %q, want founder@example.com", emailSpy.verificationTo)
	}
	if !strings.Contains(emailSpy.verificationURL, "/verify-email?token=") {
		t.Fatalf("verification URL = %q, want verify-email token URL", emailSpy.verificationURL)
	}

	var user model.User
	if err := db.Where("email = ?", "founder@example.com").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.EmailVerifiedAt != nil {
		t.Fatalf("EmailVerifiedAt = %v, want nil", user.EmailVerifiedAt)
	}

	var tokenCount int64
	if err := db.Model(&model.EmailVerificationToken{}).Where("user_id = ? AND used_at IS NULL", user.ID).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count verification tokens: %v", err)
	}
	if tokenCount != 1 {
		t.Fatalf("active verification token count = %d, want 1", tokenCount)
	}
}

func TestAuthServiceSignupCreatesUniqueDefaultOrganizationWhenSlugExists(t *testing.T) {
	svc, db, _ := newAuthSignupTestService(t)
	ctx := context.Background()

	_, err := svc.Signup(ctx, model.SignupRequest{
		Email:    "waqar.one@example.com",
		Password: "strongpass1",
		FullName: "Waqar One",
	})
	if err != nil {
		t.Fatalf("first Signup() error = %v", err)
	}

	resp, err := svc.Signup(ctx, model.SignupRequest{
		Email:    "waqar.two@example.com",
		Password: "strongpass1",
		FullName: "Waqar Two",
	})
	if err != nil {
		t.Fatalf("second Signup() error = %v", err)
	}

	orgs, err := repository.NewOrganizationRepository(db).List(ctx, resp.User.ID)
	if err != nil {
		t.Fatalf("list organizations: %v", err)
	}
	if len(orgs) != 1 {
		t.Fatalf("organization count = %d, want 1", len(orgs))
	}
	if orgs[0].Slug != "waqar-s-organization-2" {
		t.Fatalf("organization slug = %q, want waqar-s-organization-2", orgs[0].Slug)
	}
	if orgs[0].Role != model.RoleOwner {
		t.Fatalf("organization role = %q, want owner", orgs[0].Role)
	}
}

func TestAuthServiceSignupRejectsDisposableEmail(t *testing.T) {
	svc, _, _ := newAuthSignupTestService(t)

	_, err := svc.Signup(context.Background(), model.SignupRequest{
		Email:    "trial@mailinator.com",
		Password: "strongpass1",
		FullName: "Trial User",
	})
	if err == nil {
		t.Fatal("Signup() error = nil, want disposable email rejection")
	}
	if !strings.Contains(err.Error(), "temporary email addresses are not allowed") {
		t.Fatalf("Signup() error = %q, want disposable email message", err.Error())
	}
}

func TestAuthServiceVerifyEmailMarksUserVerifiedAndConsumesToken(t *testing.T) {
	svc, db, emailSpy := newAuthSignupTestService(t)

	_, err := svc.Signup(context.Background(), model.SignupRequest{
		Email:    "verify@example.com",
		Password: "strongpass1",
		FullName: "Verify Example",
	})
	if err != nil {
		t.Fatalf("Signup() error = %v", err)
	}
	rawToken := emailSpy.verificationURL[strings.LastIndex(emailSpy.verificationURL, "token=")+len("token="):]

	resp, err := svc.VerifyEmail(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("VerifyEmail() error = %v", err)
	}
	if !resp.EmailVerified {
		t.Fatal("VerifyEmail() returned EmailVerified = false, want true")
	}

	var user model.User
	if err := db.Where("email = ?", "verify@example.com").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("EmailVerifiedAt = nil, want timestamp")
	}

	if _, err := svc.VerifyEmail(context.Background(), rawToken); err == nil {
		t.Fatal("VerifyEmail() reused token error = nil, want invalid/expired error")
	}
}

func TestAuthServiceGoogleAuthCreatesVerifiedUser(t *testing.T) {
	svc, db, _ := newAuthSignupTestService(t)

	resp, err := svc.SignInWithGoogle(context.Background(), GoogleIdentity{
		Subject:       "google-sub-123",
		Email:         "google@example.com",
		EmailVerified: true,
		FullName:      "Google Example",
	})
	if err != nil {
		t.Fatalf("SignInWithGoogle() error = %v", err)
	}
	if resp.User.Email != "google@example.com" {
		t.Fatalf("user email = %q, want google@example.com", resp.User.Email)
	}
	if !resp.User.EmailVerified {
		t.Fatal("Google verified email should mark Helpin account verified")
	}

	var user model.User
	if err := db.Where("email = ?", "google@example.com").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.GoogleSubject == nil || *user.GoogleSubject != "google-sub-123" {
		t.Fatalf("GoogleSubject = %v, want google-sub-123", user.GoogleSubject)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("EmailVerifiedAt = nil, want timestamp")
	}
}

func newAuthSignupTestService(t *testing.T) (*AuthService, *gorm.DB, *authSignupEmailSpy) {
	t.Helper()

	db := newTestDB(t)
	emailSpy := &authSignupEmailSpy{}
	svc := NewAuthService(
		repository.NewUserRepository(db),
		nil,
		repository.NewOrganizationRepository(db),
		nil,
		repository.NewEmailVerificationTokenRepository(db),
		auth.NewJWTManager("test-secret"),
		nil,
		emailSpy,
		"http://localhost:5173",
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	return svc, db, emailSpy
}
