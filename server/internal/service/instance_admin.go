package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Signup and server admin errors. Their text is shown to users as is.
var (
	// ErrSignupInviteOnly rejects self-signup on an invite-only server.
	ErrSignupInviteOnly = errors.New("Signup on this server is by invitation. Ask your admin for an invite.")
	// ErrSignupDomainNotAllowed rejects an address outside the allowed domains.
	ErrSignupDomainNotAllowed = errors.New("Signup on this server is limited to approved email domains. Use your work email or ask your admin for an invite.")
	// ErrSignupVerificationUnavailable rejects domain signup while the server
	// cannot send the verification email.
	ErrSignupVerificationUnavailable = errors.New("Signup is unavailable because this server can't send verification email yet. Ask your admin for an invite.")
	// ErrNotServerAdmin rejects server administration by other accounts.
	ErrNotServerAdmin = errors.New("only server admins can change server settings")
	// ErrLastServerAdmin keeps at least one server admin.
	ErrLastServerAdmin = errors.New("a server needs at least one admin; add another admin before removing this one")
	// ErrServerAdminFromEnv protects admins designated by HELPIN_ADMIN_EMAILS.
	ErrServerAdminFromEnv = errors.New("this admin is set by HELPIN_ADMIN_EMAILS on the server; remove the address there")
	// ErrServerAdminUserNotFound reports a grant for an unknown address.
	ErrServerAdminUserNotFound = errors.New("no account uses that email; invite the person first")
	// ErrInvalidSignupPolicy wraps signup policy validation failures.
	ErrInvalidSignupPolicy = errors.New("invalid signup policy")
)

const _maxSignupDomains = 50

var _signupDomainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// SignupCandidate describes an account about to be created by self-signup.
type SignupCandidate struct {
	Email string
	// EmailVerified is true when the identity provider verified the address
	// (Google sign-in); password signups are unverified.
	EmailVerified bool
}

// SignupAdmission is the policy decision for an admitted signup.
type SignupAdmission struct {
	// ServerAdmin is true for the first account on a fresh server and for a
	// verified HELPIN_ADMIN_EMAILS address.
	ServerAdmin bool
	// VerificationRequired blocks sign-in until the email is verified.
	VerificationRequired bool
}

// InstanceServiceOptions configures InstanceService.
type InstanceServiceOptions struct {
	// AdminEmails are the HELPIN_ADMIN_EMAILS addresses.
	AdminEmails []string
	// MailReady reports whether application email can currently be sent.
	MailReady func() bool
}

// InstanceService owns self-hosted server administration (Community only):
// the first-account server admin, server admin grants, and the signup policy.
type InstanceService struct {
	repo        *repository.InstanceSettingsRepository
	users       *repository.UserRepository
	adminEmails map[string]struct{}
	mailReady   func() bool
	now         func() time.Time
	logger      *slog.Logger
	// signupMu serializes signups within one API process. Across processes
	// the conditional UPDATE in ClaimFirstAdmin is the guard.
	signupMu sync.Mutex
}

// NewInstanceService creates an InstanceService.
func NewInstanceService(repo *repository.InstanceSettingsRepository, users *repository.UserRepository, opts InstanceServiceOptions) *InstanceService {
	emails := make(map[string]struct{}, len(opts.AdminEmails))
	for _, address := range opts.AdminEmails {
		if normalized := normalizeAuthEmail(address); normalized != "" {
			emails[normalized] = struct{}{}
		}
	}
	mailReady := opts.MailReady
	if mailReady == nil {
		mailReady = func() bool { return false }
	}
	return &InstanceService{
		repo: repo, users: users, adminEmails: emails, mailReady: mailReady,
		now: time.Now, logger: slog.Default().With("service", "instance"),
	}
}

// Bootstrap prepares server administration at startup: it creates the
// settings row when missing, grants HELPIN_ADMIN_EMAILS accounts, and, on a
// server that has accounts but no server admin (an upgrade), makes the owner
// of the oldest organization the server admin.
func (s *InstanceService) Bootstrap(ctx context.Context) error {
	if _, err := s.repo.Ensure(ctx); err != nil {
		return err
	}
	if granted, err := s.repo.GrantServerAdminByEmails(ctx, s.envAdminList()); err != nil {
		return err
	} else if granted > 0 {
		s.logger.InfoContext(ctx, "granted server admin from HELPIN_ADMIN_EMAILS", "accounts", granted)
	}
	return s.repo.WithTx(ctx, func(repo *repository.InstanceSettingsRepository, _ *gorm.DB) error {
		if err := repo.LockSettings(ctx); err != nil {
			return err
		}
		admins, err := repo.CountServerAdmins(ctx)
		if err != nil || admins > 0 {
			return err
		}
		candidate, err := repo.UpgradeAdminCandidate(ctx)
		if err != nil || candidate == "" {
			return err
		}
		if err := repo.SetServerAdmin(ctx, candidate, true); err != nil {
			return err
		}
		if err := repo.MarkBootstrapped(ctx, s.now().UTC()); err != nil {
			return err
		}
		s.logger.InfoContext(ctx, "assigned server admin to existing server", "user_id", candidate)
		return nil
	})
}

// CreateAccount admits a self-signup under the signup policy and creates the
// account with create in the same transaction. The first account on a fresh
// server is always admitted and becomes the server admin; concurrent first
// signups resolve to exactly one admin.
func (s *InstanceService) CreateAccount(
	ctx context.Context,
	candidate SignupCandidate,
	create func(tx *gorm.DB, admission SignupAdmission) (*model.User, error),
) (*model.User, SignupAdmission, error) {
	candidate.Email = normalizeAuthEmail(candidate.Email)
	s.signupMu.Lock()
	defer s.signupMu.Unlock()

	var user *model.User
	var admission SignupAdmission
	err := s.repo.WithTx(ctx, func(repo *repository.InstanceSettingsRepository, tx *gorm.DB) error {
		settings, err := repo.Ensure(ctx)
		if err != nil {
			return err
		}
		first, err := repo.ClaimFirstAdmin(ctx, s.now().UTC())
		if err != nil {
			return err
		}
		if first {
			admission = SignupAdmission{ServerAdmin: true}
		} else {
			admission, err = s.admit(settings, candidate)
			if err != nil {
				return err
			}
		}
		user, err = create(tx, admission)
		if err != nil {
			return err
		}
		if admission.ServerAdmin {
			if err := repo.SetServerAdmin(ctx, user.ID, true); err != nil {
				return err
			}
			user.IsServerAdmin = true
		}
		return nil
	})
	if err != nil {
		return nil, SignupAdmission{}, err
	}
	if admission.ServerAdmin {
		s.logger.InfoContext(ctx, "server admin created at signup", "user_id", user.ID)
	}
	return user, admission, nil
}

// CheckSignup reports the policy error a signup would get, without creating
// anything, so callers can reject early and avoid revealing whether an
// address already has an account.
func (s *InstanceService) CheckSignup(ctx context.Context, candidate SignupCandidate) error {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	if settings == nil || settings.AdminBootstrappedAt == nil {
		hasUsers, err := s.repo.HasUsers(ctx)
		if err != nil || !hasUsers {
			return err
		}
		if settings == nil {
			settings = &model.InstanceSettings{SignupMode: model.SignupModeOpen}
		}
	}
	candidate.Email = normalizeAuthEmail(candidate.Email)
	_, err = s.admit(settings, candidate)
	return err
}

func (s *InstanceService) admit(settings *model.InstanceSettings, candidate SignupCandidate) (SignupAdmission, error) {
	admission := SignupAdmission{ServerAdmin: candidate.EmailVerified && s.isEnvAdmin(candidate.Email)}
	switch settings.SignupMode {
	case model.SignupModeOpen:
		return admission, nil
	case model.SignupModeDomains:
		if !emailDomainAllowed(candidate.Email, splitSignupDomains(settings.SignupAllowedDomains)) {
			return SignupAdmission{}, ErrSignupDomainNotAllowed
		}
		if candidate.EmailVerified {
			return admission, nil
		}
		// Anyone can type an address on an allowed domain, so the account
		// must prove it owns the mailbox before it can be used.
		if !s.mailReady() {
			return SignupAdmission{}, ErrSignupVerificationUnavailable
		}
		admission.VerificationRequired = true
		return admission, nil
	default:
		return SignupAdmission{}, ErrSignupInviteOnly
	}
}

// PublicPolicy returns what the sign-in and registration pages need.
func (s *InstanceService) PublicPolicy(ctx context.Context) (model.PublicSignupPolicy, error) {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return model.PublicSignupPolicy{}, err
	}
	if settings == nil {
		hasUsers, err := s.repo.HasUsers(ctx)
		if err != nil {
			return model.PublicSignupPolicy{}, err
		}
		if hasUsers {
			return model.PublicSignupPolicy{Mode: model.SignupModeOpen, AllowedDomains: []string{}}, nil
		}
		return model.PublicSignupPolicy{Mode: model.SignupModeInviteOnly, AllowedDomains: []string{}, FirstUser: true}, nil
	}
	policy := model.PublicSignupPolicy{Mode: settings.SignupMode, AllowedDomains: []string{}, FirstUser: settings.AdminBootstrappedAt == nil}
	if settings.SignupMode == model.SignupModeDomains {
		policy.AllowedDomains = splitSignupDomains(settings.SignupAllowedDomains)
	}
	return policy, nil
}

// SignupPolicy returns the signup policy for server admins.
func (s *InstanceService) SignupPolicy(ctx context.Context) (*model.SignupPolicyResponse, error) {
	settings, err := s.repo.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	return &model.SignupPolicyResponse{
		Mode:                settings.SignupMode,
		AllowedDomains:      splitSignupDomains(settings.SignupAllowedDomains),
		AppEmailConfigured:  s.mailReady(),
		RecommendInviteOnly: settings.SignupMode == model.SignupModeOpen,
	}, nil
}

// UpdateSignupPolicy validates and stores the signup policy.
func (s *InstanceService) UpdateSignupPolicy(ctx context.Context, actorID string, req model.UpdateSignupPolicyRequest) (*model.SignupPolicyResponse, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	mode := strings.TrimSpace(req.Mode)
	domains, err := normalizeSignupDomains(req.AllowedDomains)
	if err != nil {
		return nil, err
	}
	switch mode {
	case model.SignupModeOpen, model.SignupModeInviteOnly:
	case model.SignupModeDomains:
		if len(domains) == 0 {
			return nil, fmt.Errorf("%w: add at least one email domain", ErrInvalidSignupPolicy)
		}
		if !s.mailReady() {
			return nil, fmt.Errorf("%w: domain signup sends a verification email, so set up application email first", ErrInvalidSignupPolicy)
		}
	default:
		return nil, fmt.Errorf("%w: mode must be open, invite_only or domains", ErrInvalidSignupPolicy)
	}
	if err := s.repo.UpdateSignupPolicy(ctx, mode, domains, actorID); err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "signup policy updated", "user_id", actorID, "mode", mode, "domains", len(domains))
	return s.SignupPolicy(ctx)
}

// IsServerAdmin reports whether userID administers the server.
func (s *InstanceService) IsServerAdmin(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, nil
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil && user.IsServerAdmin, nil
}

// ListAdmins returns the server admins.
func (s *InstanceService) ListAdmins(ctx context.Context) ([]model.ServerAdmin, error) {
	users, err := s.repo.ListServerAdmins(ctx)
	if err != nil {
		return nil, err
	}
	admins := make([]model.ServerAdmin, 0, len(users))
	for _, user := range users {
		source := model.ServerAdminSourceGranted
		if s.isEnvAdmin(user.Email) {
			source = model.ServerAdminSourceEnv
		}
		admins = append(admins, model.ServerAdmin{UserID: user.ID, Email: user.Email, FullName: user.FullName, Source: source})
	}
	return admins, nil
}

// GrantAdmin makes the account with email a server admin.
func (s *InstanceService) GrantAdmin(ctx context.Context, actorID, email string) ([]model.ServerAdmin, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	user, err := s.users.GetByEmail(ctx, normalizeAuthEmail(email))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrServerAdminUserNotFound
	}
	if !user.IsServerAdmin {
		if err := s.repo.SetServerAdmin(ctx, user.ID, true); err != nil {
			return nil, err
		}
		s.logger.InfoContext(ctx, "server admin granted", "user_id", user.ID, "granted_by", actorID)
	}
	return s.ListAdmins(ctx)
}

// RevokeAdmin removes server admin from userID, never from the last admin or
// from an address in HELPIN_ADMIN_EMAILS.
func (s *InstanceService) RevokeAdmin(ctx context.Context, actorID, userID string) ([]model.ServerAdmin, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	err := s.repo.WithTx(ctx, func(repo *repository.InstanceSettingsRepository, tx *gorm.DB) error {
		if err := repo.LockSettings(ctx); err != nil {
			return err
		}
		user, err := repository.NewUserRepository(tx).GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if user == nil || !user.IsServerAdmin {
			return nil
		}
		if s.isEnvAdmin(user.Email) {
			return ErrServerAdminFromEnv
		}
		admins, err := repo.CountServerAdmins(ctx)
		if err != nil {
			return err
		}
		if admins <= 1 {
			return ErrLastServerAdmin
		}
		return repo.SetServerAdmin(ctx, userID, false)
	})
	if err != nil {
		return nil, err
	}
	s.logger.InfoContext(ctx, "server admin revoked", "user_id", userID, "revoked_by", actorID)
	return s.ListAdmins(ctx)
}

// GrantEnvAdminOnVerification grants server admin once an account on
// HELPIN_ADMIN_EMAILS has verified its email.
func (s *InstanceService) GrantEnvAdminOnVerification(ctx context.Context, user *model.User) {
	if user == nil || user.IsServerAdmin || user.EmailVerifiedAt == nil || !s.isEnvAdmin(user.Email) {
		return
	}
	if err := s.repo.SetServerAdmin(ctx, user.ID, true); err != nil {
		s.logger.ErrorContext(ctx, "grant server admin after verification failed", "error", err, "user_id", user.ID)
		return
	}
	user.IsServerAdmin = true
}

func (s *InstanceService) requireAdmin(ctx context.Context, userID string) error {
	admin, err := s.IsServerAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !admin {
		return ErrNotServerAdmin
	}
	return nil
}

func (s *InstanceService) isEnvAdmin(email string) bool {
	_, ok := s.adminEmails[normalizeAuthEmail(email)]
	return ok
}

func (s *InstanceService) envAdminList() []string {
	emails := make([]string, 0, len(s.adminEmails))
	for email := range s.adminEmails {
		emails = append(emails, email)
	}
	sort.Strings(emails)
	return emails
}

// normalizeSignupDomains lowercases, deduplicates and validates domains,
// accepting "@example.com" and surrounding spaces.
func normalizeSignupDomains(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	domains := make([]string, 0, len(raw))
	for _, value := range raw {
		domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "@")
		if domain == "" {
			continue
		}
		if !_signupDomainPattern.MatchString(domain) || len(domain) > 253 {
			return nil, fmt.Errorf("%w: %q is not an email domain", ErrInvalidSignupPolicy, value)
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	if len(domains) > _maxSignupDomains {
		return nil, fmt.Errorf("%w: at most %d domains are allowed", ErrInvalidSignupPolicy, _maxSignupDomains)
	}
	return domains, nil
}

func splitSignupDomains(stored string) []string {
	domains := []string{}
	for _, domain := range strings.Split(stored, ",") {
		if domain = strings.TrimSpace(domain); domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains
}

// emailDomainAllowed matches the address's domain exactly; subdomains must be
// listed on their own.
func emailDomainAllowed(email string, domains []string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, allowed := range domains {
		if domain == allowed {
			return true
		}
	}
	return false
}
