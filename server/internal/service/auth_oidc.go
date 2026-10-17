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

// oidcInvitations is the slice of the invite service OIDC sign-in needs, so an
// invited person is admitted under any signup policy.
type oidcInvitations interface {
	HasPendingInvitation(ctx context.Context, email string) (bool, error)
	AcceptPendingInvitations(ctx context.Context, userID, email string) (int, error)
}

// SetOIDCInvitations lets OIDC sign-in admit people who hold a pending invitation.
func (s *AuthService) SetOIDCInvitations(invitations oidcInvitations) {
	s.oidcInvitations = invitations
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
	created := false
	if user == nil {
		user, created, err = s.createOIDCUser(ctx, issuer, subject, identity)
		if err != nil {
			return nil, err
		}
	} else if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncUser(ctx, user)
	}
	// Every sign-in accepts pending invitations, so one that failed or raced on
	// an earlier sign-in is retried. A new account joins the invited workspace
	// instead of getting a default organization, as with invitation signup.
	accepted := s.acceptOIDCInvitations(ctx, user)
	if created && accepted == 0 {
		s.autoCreateOrganization(ctx, user)
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

// createOIDCUser reports created=false when a concurrent sign-in for the same
// identity created the account first and this call returns that account.
func (s *AuthService) createOIDCUser(ctx context.Context, issuer, subject string, identity OIDCIdentity) (user *model.User, created bool, err error) {
	email := normalizeAuthEmail(identity.Email)
	if email == "" {
		return nil, false, fmt.Errorf("oidc account did not return a usable email")
	}
	if !identity.EmailVerified {
		return nil, false, fmt.Errorf("oidc account email is not verified")
	}
	if isDisposableEmailDomain(email) {
		return nil, false, fmt.Errorf("temporary email addresses are not allowed")
	}
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, false, fmt.Errorf("get user by email: %w", err)
	}
	if existing != nil {
		// A concurrent first sign-in for this identity may have just created it.
		if winner, err := s.oidcUserByIdentity(ctx, issuer, subject); err == nil && winner != nil {
			return winner, false, nil
		}
		s.logger.WarnContext(ctx, "oidc sign-in refused: email belongs to an unlinked account",
			"issuer", issuer, "subject", subject, "user_id", existing.ID)
		return nil, false, ErrOIDCEmailInUse
	}

	fullName := strings.TrimSpace(identity.FullName)
	if fullName == "" {
		fullName = email
	}
	passwordHash, err := auth.HashPassword(randomOAuthPassword())
	if err != nil {
		return nil, false, fmt.Errorf("hash oauth password placeholder: %w", err)
	}
	now := time.Now().UTC()
	newUser := &model.User{
		Email:           email,
		PasswordHash:    passwordHash,
		FullName:        fullName,
		EmailVerifiedAt: &now,
	}
	// The user and its identity row commit together, so a failure never
	// leaves an account that the provider cannot sign in to.
	create := func(tx *gorm.DB, admission SignupAdmission) (*model.User, error) {
		newUser.SignupVerificationPending = admission.VerificationRequired
		created, err := repository.NewUserRepository(tx).CreateUser(ctx, newUser)
		if err != nil {
			return nil, err
		}
		link := &model.UserOIDCIdentity{UserID: created.ID, Issuer: issuer, Subject: subject}
		if err := repository.NewUserOIDCIdentityRepository(tx).Create(ctx, link); err != nil {
			return nil, err
		}
		return created, nil
	}
	inTx := func(admission SignupAdmission) (*model.User, error) {
		var created *model.User
		err := s.oidcIdentityRepo.Transaction(ctx, func(tx *gorm.DB) error {
			var createErr error
			created, createErr = create(tx, admission)
			if createErr != nil || !admission.ServerAdmin {
				return createErr
			}
			// Same transaction as the account, as the signup gate does.
			if err := repository.NewInstanceSettingsRepository(tx).SetServerAdmin(ctx, created.ID, true); err != nil {
				return err
			}
			created.IsServerAdmin = true
			return nil
		})
		return created, err
	}
	var createErr error
	if s.signupGate != nil {
		user, _, createErr = s.signupGate.CreateAccount(ctx, SignupCandidate{Email: email, EmailVerified: true}, create)
		if (errors.Is(createErr, ErrSignupInviteOnly) || errors.Is(createErr, ErrSignupDomainNotAllowed)) && s.hasPendingInvitation(ctx, email) {
			// An invitation works under every signup policy; the provider has
			// verified that this person owns the invited address.
			user, createErr = inTx(SignupAdmission{ServerAdmin: s.isEnvAdminEmail(email)})
		}
	} else {
		user, createErr = inTx(SignupAdmission{})
	}
	if createErr != nil {
		if isSignupPolicyError(createErr) {
			return nil, false, createErr
		}
		// A concurrent first sign-in for this identity may have won the unique
		// index on the user email or on (issuer, subject).
		if winner, err := s.oidcUserByIdentity(ctx, issuer, subject); err == nil && winner != nil {
			return winner, false, nil
		}
		return nil, false, fmt.Errorf("create oidc user: %w", createErr)
	}
	if s.customerIOIdentity != nil {
		s.customerIOIdentity.SyncUser(ctx, user)
		s.customerIOIdentity.TrackUserSignedUp(ctx, user, "oidc")
	}
	return user, true, nil
}

func (s *AuthService) userHasOIDCIdentity(ctx context.Context, userID string) (bool, error) {
	if s.oidcIdentityRepo == nil {
		return false, nil
	}
	return s.oidcIdentityRepo.HasIdentity(ctx, userID)
}

func (s *AuthService) hasPendingInvitation(ctx context.Context, email string) bool {
	if s.oidcInvitations == nil {
		return false
	}
	pending, err := s.oidcInvitations.HasPendingInvitation(ctx, email)
	if err != nil {
		s.logger.WarnContext(ctx, "could not check pending invitations for oidc user", "error", err)
		return false
	}
	return pending
}

// isEnvAdminEmail reports whether HELPIN_ADMIN_EMAILS lists the address. The
// caller must have a provider-verified address.
func (s *AuthService) isEnvAdminEmail(email string) bool {
	checker, ok := s.signupGate.(interface{ IsEnvAdmin(email string) bool })
	return ok && checker.IsEnvAdmin(email)
}

// acceptOIDCInvitations accepts the user's pending invitations on a best-effort
// basis and returns how many were accepted. Failures stay pending.
func (s *AuthService) acceptOIDCInvitations(ctx context.Context, user *model.User) int {
	if s.oidcInvitations == nil {
		return 0
	}
	accepted, err := s.oidcInvitations.AcceptPendingInvitations(ctx, user.ID, user.Email)
	if err != nil {
		s.logger.WarnContext(ctx, "could not accept every invitation for oidc user", "user_id", user.ID, "accepted", accepted, "error", err)
	}
	return accepted
}
