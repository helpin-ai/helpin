package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrOIDCEmailInUse rejects an OIDC sign-in whose email already belongs to a
// Helpin account that is not linked to that OIDC identity. Accounts are never
// linked by email; an operator links them by hand.
var ErrOIDCEmailInUse = errors.New("an account with this email already exists and is not linked to this sign-in provider")

// OIDCIdentity is the normalized profile returned by an OpenID Connect provider.
type OIDCIdentity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	FullName      string
}

// SetOIDCIdentityRepository enables OpenID Connect sign-in.
func (s *AuthService) SetOIDCIdentityRepository(repo *repository.UserOIDCIdentityRepository) {
	s.oidcIdentityRepo = repo
}

// SignInWithOIDC signs in the user linked to the provider's issuer and subject.
// A first sign-in creates an account through the signup policy, but only when
// the provider returns a verified email that no existing account uses.
func (s *AuthService) SignInWithOIDC(ctx context.Context, identity OIDCIdentity) (*model.AuthResponse, error) {
	if s.oidcIdentityRepo == nil {
		return nil, fmt.Errorf("oidc sign-in is not configured")
	}
	issuer := strings.TrimSpace(identity.Issuer)
	subject := strings.TrimSpace(identity.Subject)
	if issuer == "" || subject == "" {
		return nil, fmt.Errorf("oidc identity is missing issuer or subject")
	}

	user, err := s.oidcUserByIdentity(ctx, issuer, subject)
	if err != nil {
		return nil, err
	}
	if user == nil {
		user, err = s.createOIDCUser(ctx, issuer, subject, identity)
		if err != nil {
			return nil, err
		}
	} else if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncUser(ctx, user)
	}

	accessToken, refreshToken, err := s.generateTokenPairForUser(user, true, false)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}
	s.logger.InfoContext(ctx, "user signed in with oidc", "user_id", user.ID, "issuer", issuer)
	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toUserProfile(user),
	}, nil
}

func (s *AuthService) oidcUserByIdentity(ctx context.Context, issuer, subject string) (*model.User, error) {
	linked, err := s.oidcIdentityRepo.GetByIssuerSubject(ctx, issuer, subject)
	if err != nil {
		return nil, err
	}
	if linked == nil {
		return nil, nil
	}
	user, err := s.userRepo.GetByID(ctx, linked.UserID)
	if err != nil {
		return nil, fmt.Errorf("get oidc user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("oidc identity %s references a missing user", linked.ID)
	}
	return user, nil
}

func (s *AuthService) createOIDCUser(ctx context.Context, issuer, subject string, identity OIDCIdentity) (*model.User, error) {
	email := normalizeAuthEmail(identity.Email)
	if email == "" {
		return nil, fmt.Errorf("oidc account did not return a usable email")
	}
	if !identity.EmailVerified {
		return nil, fmt.Errorf("oidc account email is not verified")
	}
	if isDisposableEmailDomain(email) {
		return nil, fmt.Errorf("temporary email addresses are not allowed")
	}
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	if existing != nil {
		s.logger.WarnContext(ctx, "oidc sign-in refused: email belongs to an unlinked account",
			"issuer", issuer, "subject", subject, "user_id", existing.ID)
		return nil, ErrOIDCEmailInUse
	}

	fullName := strings.TrimSpace(identity.FullName)
	if fullName == "" {
		fullName = email
	}
	passwordHash, err := auth.HashPassword(randomOAuthPassword())
	if err != nil {
		return nil, fmt.Errorf("hash oauth password placeholder: %w", err)
	}
	now := time.Now().UTC()
	user := &model.User{
		Email:           email,
		PasswordHash:    passwordHash,
		FullName:        fullName,
		EmailVerifiedAt: &now,
	}
	// The user and its identity row commit together, so a failure never
	// leaves an account that the provider cannot sign in to.
	create := func(tx *gorm.DB, admission SignupAdmission) (*model.User, error) {
		user.SignupVerificationPending = admission.VerificationRequired
		created, err := repository.NewUserRepository(tx).CreateUser(ctx, user)
		if err != nil {
			return nil, err
		}
		link := &model.UserOIDCIdentity{UserID: created.ID, Issuer: issuer, Subject: subject}
		if err := repository.NewUserOIDCIdentityRepository(tx).Create(ctx, link); err != nil {
			return nil, err
		}
		return created, nil
	}
	if s.signupGate != nil {
		user, _, err = s.signupGate.CreateAccount(ctx, SignupCandidate{Email: email, EmailVerified: true}, create)
	} else {
		err = s.oidcIdentityRepo.Transaction(ctx, func(tx *gorm.DB) error {
			var createErr error
			user, createErr = create(tx, SignupAdmission{})
			return createErr
		})
	}
	if err != nil {
		if isSignupPolicyError(err) {
			return nil, err
		}
		return nil, fmt.Errorf("create oidc user: %w", err)
	}
	s.autoCreateOrganization(ctx, user)
	if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncUser(ctx, user)
		s.customerIOIdentity.TrackUserSignedUp(ctx, user, "oidc")
	}
	return user, nil
}
