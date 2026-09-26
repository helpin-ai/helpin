package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const instanceSettingsTestSchema = `CREATE TABLE instance_settings (
	id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	singleton BOOLEAN NOT NULL DEFAULT 1 UNIQUE,
	signup_mode TEXT NOT NULL DEFAULT 'invite_only',
	signup_allowed_domains TEXT NOT NULL DEFAULT '',
	admin_bootstrapped_at DATETIME,
	smtp_host TEXT,
	smtp_port INTEGER,
	smtp_username TEXT,
	smtp_password_encrypted TEXT,
	smtp_from TEXT,
	smtp_tls_mode TEXT,
	smtp_updated_at DATETIME,
	updated_by TEXT,
	created_at DATETIME,
	updated_at DATETIME
)`

type instanceTestEnv struct {
	db       *gorm.DB
	instance *InstanceService
	auth     *AuthService
	email    *authSignupEmailSpy
	mail     *bool
}

func newInstanceTestEnv(t *testing.T, adminEmails ...string) *instanceTestEnv {
	t.Helper()
	db := newTestDB(t)
	mustExec(t, db, instanceSettingsTestSchema)
	mailReady := false
	env := &instanceTestEnv{db: db, email: &authSignupEmailSpy{}, mail: &mailReady}
	env.instance = NewInstanceService(repository.NewInstanceSettingsRepository(db), repository.NewUserRepository(db), InstanceServiceOptions{
		AdminEmails: adminEmails,
		MailReady:   func() bool { return *env.mail },
	})
	env.auth = NewAuthService(
		repository.NewUserRepository(db), nil, repository.NewOrganizationRepository(db), nil,
		repository.NewEmailVerificationTokenRepository(db), auth.NewJWTManager("test-secret"), nil,
		env.email, "http://localhost:5173", []byte("0123456789abcdef0123456789abcdef"),
	)
	env.auth.SetEmailVerificationRequired(false)
	env.auth.SetSignupGate(env.instance)
	return env
}

func (e *instanceTestEnv) signup(t *testing.T, email string) (*model.AuthResponse, error) {
	t.Helper()
	return e.auth.Signup(context.Background(), model.SignupRequest{Email: email, Password: "strongpass1", FullName: "Test Person"})
}

func (e *instanceTestEnv) setPolicy(t *testing.T, mode, domains string) {
	t.Helper()
	if _, err := repository.NewInstanceSettingsRepository(e.db).Ensure(context.Background()); err != nil {
		t.Fatalf("ensure settings: %v", err)
	}
	mustExec(t, e.db, `UPDATE instance_settings SET signup_mode = ?, signup_allowed_domains = ?`, mode, domains)
}

func (e *instanceTestEnv) user(t *testing.T, email string) model.User {
	t.Helper()
	var user model.User
	if err := e.db.Where("email = ?", email).First(&user).Error; err != nil {
		t.Fatalf("load user %s: %v", email, err)
	}
	return user
}

func TestSignupFirstAccountBecomesServerAdmin(t *testing.T) {
	env := newInstanceTestEnv(t)

	resp, err := env.signup(t, "founder@example.com")
	if err != nil {
		t.Fatalf("first signup: %v", err)
	}
	if !resp.User.IsServerAdmin {
		t.Fatal("first account should be the server admin")
	}
	if resp.AccessToken == "" {
		t.Fatal("first account should be signed in")
	}

	// A fresh server is invite-only after the first account.
	_, err = env.signup(t, "second@example.com")
	if !errors.Is(err, ErrSignupInviteOnly) {
		t.Fatalf("second signup error = %v, want ErrSignupInviteOnly", err)
	}
	if err.Error() != "Signup on this server is by invitation. Ask your admin for an invite." {
		t.Fatalf("error text = %q", err.Error())
	}
	var count int64
	env.db.Model(&model.User{}).Count(&count)
	if count != 1 {
		t.Fatalf("users = %d, want 1", count)
	}
}

func TestCreateAccountConcurrentFirstSignupsYieldOneAdmin(t *testing.T) {
	env := newInstanceTestEnv(t)
	env.setPolicy(t, model.SignupModeOpen, "")
	mustExec(t, env.db, `UPDATE instance_settings SET admin_bootstrapped_at = NULL`)

	const signups = 8
	var wg sync.WaitGroup
	admins := make(chan bool, signups)
	errs := make(chan error, signups)
	for i := 0; i < signups; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			email := fmt.Sprintf("racer%d@example.com", i)
			user, admission, err := env.instance.CreateAccount(context.Background(), SignupCandidate{Email: email},
				func(tx *gorm.DB, _ SignupAdmission) (*model.User, error) {
					return repository.NewUserRepository(tx).CreateUser(context.Background(), &model.User{Email: email, PasswordHash: "x", FullName: "Racer"})
				})
			if err != nil {
				errs <- err
				return
			}
			admins <- admission.ServerAdmin && user.IsServerAdmin
		}(i)
	}
	wg.Wait()
	close(admins)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent signup failed: %v", err)
	}
	granted := 0
	for admin := range admins {
		if admin {
			granted++
		}
	}
	if granted != 1 {
		t.Fatalf("server admins granted = %d, want exactly 1", granted)
	}
	var stored int64
	env.db.Model(&model.User{}).Where("is_server_admin = ?", true).Count(&stored)
	if stored != 1 {
		t.Fatalf("stored server admins = %d, want 1", stored)
	}
}

func TestCreateAccountFailedFirstSignupDoesNotSpendClaim(t *testing.T) {
	env := newInstanceTestEnv(t)
	_, _, err := env.instance.CreateAccount(context.Background(), SignupCandidate{Email: "a@example.com"},
		func(*gorm.DB, SignupAdmission) (*model.User, error) { return nil, errors.New("insert failed") })
	if err == nil {
		t.Fatal("want create error")
	}
	resp, err := env.signup(t, "b@example.com")
	if err != nil {
		t.Fatalf("signup after failed first attempt: %v", err)
	}
	if !resp.User.IsServerAdmin {
		t.Fatal("first successful account should be the server admin")
	}
}

func TestSignupPolicyModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		domains  string
		mail     bool
		email    string
		wantErr  error
		wantWait bool
	}{
		{name: "open admits anyone", mode: model.SignupModeOpen, email: "anyone@example.org"},
		{name: "invite only rejects", mode: model.SignupModeInviteOnly, email: "anyone@example.org", wantErr: ErrSignupInviteOnly},
		{name: "domains rejects other domain", mode: model.SignupModeDomains, domains: "acme.com", mail: true, email: "x@example.org", wantErr: ErrSignupDomainNotAllowed},
		{name: "domains rejects subdomain", mode: model.SignupModeDomains, domains: "acme.com", mail: true, email: "x@eu.acme.com", wantErr: ErrSignupDomainNotAllowed},
		{name: "domains needs mail to verify", mode: model.SignupModeDomains, domains: "acme.com", email: "x@acme.com", wantErr: ErrSignupVerificationUnavailable},
		{name: "domains admits pending verification", mode: model.SignupModeDomains, domains: "acme.com,beta.io", mail: true, email: "x@Beta.io", wantWait: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newInstanceTestEnv(t)
			if _, err := env.signup(t, "founder@example.com"); err != nil {
				t.Fatalf("first signup: %v", err)
			}
			env.setPolicy(t, tt.mode, tt.domains)
			*env.mail = tt.mail

			resp, err := env.signup(t, tt.email)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("signup: %v", err)
			}
			if resp.User.IsServerAdmin {
				t.Fatal("later accounts must not become server admin")
			}
			if resp.VerificationRequired != tt.wantWait {
				t.Fatalf("VerificationRequired = %v, want %v", resp.VerificationRequired, tt.wantWait)
			}
			if tt.wantWait && resp.AccessToken != "" {
				t.Fatal("pending account must not receive a session")
			}
		})
	}
}

func TestDomainSignupRequiresVerificationBeforeSignin(t *testing.T) {
	env := newInstanceTestEnv(t)
	ctx := context.Background()
	if _, err := env.signup(t, "founder@example.com"); err != nil {
		t.Fatal(err)
	}
	env.setPolicy(t, model.SignupModeDomains, "acme.com")
	*env.mail = true

	if _, err := env.signup(t, "dev@acme.com"); err != nil {
		t.Fatalf("domain signup: %v", err)
	}
	if env.email.verificationTo != "dev@acme.com" {
		t.Fatalf("verification email to %q, want dev@acme.com (sent even though AUTH_EMAIL_VERIFICATION_REQUIRED is off)", env.email.verificationTo)
	}
	_, err := env.auth.Signin(ctx, model.SigninRequest{Email: "dev@acme.com", Password: "strongpass1"})
	if !errors.Is(err, ErrEmailVerificationPending) {
		t.Fatalf("signin before verification error = %v, want ErrEmailVerificationPending", err)
	}

	token := env.email.verificationURL[strings.Index(env.email.verificationURL, "token=")+len("token="):]
	if _, err := env.auth.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("verify email: %v", err)
	}
	resp, err := env.auth.Signin(ctx, model.SigninRequest{Email: "dev@acme.com", Password: "strongpass1"})
	if err != nil || resp.AccessToken == "" {
		t.Fatalf("signin after verification = %+v, %v", resp, err)
	}
	if env.user(t, "dev@acme.com").SignupVerificationPending {
		t.Fatal("verification should clear the pending flag")
	}
}

func TestGoogleSignupFollowsPolicyWithVerifiedEmail(t *testing.T) {
	env := newInstanceTestEnv(t)
	ctx := context.Background()
	if _, err := env.signup(t, "founder@example.com"); err != nil {
		t.Fatal(err)
	}
	env.setPolicy(t, model.SignupModeDomains, "acme.com")

	// Google verified the address, so no mail is needed and no wait applies.
	resp, err := env.auth.SignInWithGoogle(ctx, GoogleIdentity{Subject: "g-1", Email: "ops@acme.com", EmailVerified: true, FullName: "Ops"})
	if err != nil || resp.AccessToken == "" {
		t.Fatalf("google domain signup = %+v, %v", resp, err)
	}
	_, err = env.auth.SignInWithGoogle(ctx, GoogleIdentity{Subject: "g-2", Email: "x@gmail.com", EmailVerified: true, FullName: "X"})
	if !errors.Is(err, ErrSignupDomainNotAllowed) {
		t.Fatalf("google outside domains error = %v, want ErrSignupDomainNotAllowed", err)
	}
}

func TestInviteAcceptanceWorksWhenInviteOnly(t *testing.T) {
	env := newInstanceTestEnv(t)
	ctx := context.Background()
	first, err := env.signup(t, "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	env.setPolicy(t, model.SignupModeInviteOnly, "")
	if _, err := env.signup(t, "invitee@example.com"); !errors.Is(err, ErrSignupInviteOnly) {
		t.Fatalf("self-signup error = %v, want invite only", err)
	}

	seedWorkspace(t, env.db, "ws-1", "Team", "team", first.User.ID)
	seedWorkspaceMember(t, env.db, "wm-1", "ws-1", first.User.ID, "owner@example.com", "Owner", "owner")
	invites := NewInviteService(repository.NewInvitationRepository(env.db), repository.NewWorkspaceRepository(env.db), nil,
		repository.NewUserRepository(env.db), repository.NewSettingsRepository(env.db), nil, "http://localhost:5173", auth.NewJWTManager("test-secret"))
	invitation, err := invites.CreateInvitation(ctx, model.CreateInvitationRequest{WorkspaceID: "ws-1", Email: "invitee@example.com", Role: "member"}, first.User.ID)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	accepted, err := invites.AcceptInvitationWithSignup(ctx, model.AcceptInvitationWithSignupRequest{
		Token: strings.TrimPrefix(invitation.JoinURL, "http://localhost:5173/join/"), Password: "strongpass1", FullName: "Invitee",
	})
	if err != nil {
		t.Fatalf("accept invitation under invite_only: %v", err)
	}
	if accepted.User.IsServerAdmin {
		t.Fatal("invitees must not become server admin")
	}
}

func TestServerAdminGrantAndRevoke(t *testing.T) {
	env := newInstanceTestEnv(t, "ops@example.com")
	ctx := context.Background()
	first, _ := env.signup(t, "founder@example.com")
	env.setPolicy(t, model.SignupModeOpen, "")
	second, _ := env.signup(t, "second@example.com")
	founderID, secondID := first.User.ID, second.User.ID

	if _, err := env.instance.GrantAdmin(ctx, secondID, "founder@example.com"); !errors.Is(err, ErrNotServerAdmin) {
		t.Fatalf("grant by non-admin error = %v, want ErrNotServerAdmin", err)
	}
	if _, err := env.instance.RevokeAdmin(ctx, founderID, founderID); !errors.Is(err, ErrLastServerAdmin) {
		t.Fatalf("revoking the last admin error = %v, want ErrLastServerAdmin", err)
	}
	if _, err := env.instance.GrantAdmin(ctx, founderID, "nobody@example.com"); !errors.Is(err, ErrServerAdminUserNotFound) {
		t.Fatalf("grant unknown error = %v", err)
	}
	admins, err := env.instance.GrantAdmin(ctx, founderID, " Second@Example.com ")
	if err != nil || len(admins) != 2 {
		t.Fatalf("grant = %+v, %v", admins, err)
	}
	// Transfer: the new admin removes the founder.
	admins, err = env.instance.RevokeAdmin(ctx, secondID, founderID)
	if err != nil || len(admins) != 1 || admins[0].UserID != secondID {
		t.Fatalf("revoke = %+v, %v", admins, err)
	}
	if _, err := env.instance.RevokeAdmin(ctx, secondID, secondID); !errors.Is(err, ErrLastServerAdmin) {
		t.Fatalf("revoking the remaining admin error = %v, want ErrLastServerAdmin", err)
	}
}

func TestEnvServerAdminsAreGrantedAndProtected(t *testing.T) {
	env := newInstanceTestEnv(t, "OPS@example.com")
	ctx := context.Background()
	first, _ := env.signup(t, "founder@example.com")
	env.setPolicy(t, model.SignupModeOpen, "")

	// An unverified password signup on an env address is not trusted.
	resp, err := env.signup(t, "ops@example.com")
	if err != nil || resp.User.IsServerAdmin {
		t.Fatalf("unverified env signup = %+v, %v; want a regular account", resp, err)
	}
	// Startup never grants an unverified account: whoever registered the
	// address first could otherwise claim admin.
	if err := env.instance.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if admins, err := env.instance.ListAdmins(ctx); err != nil || len(admins) != 1 {
		t.Fatalf("admins before verification = %+v, %v; want only the founder", admins, err)
	}

	mustExec(t, env.db, `UPDATE users SET email_verified_at = CURRENT_TIMESTAMP WHERE email = ?`, "ops@example.com")
	if err := env.instance.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap after verification: %v", err)
	}
	admins, err := env.instance.ListAdmins(ctx)
	if err != nil || len(admins) != 2 {
		t.Fatalf("admins after bootstrap = %+v, %v", admins, err)
	}
	var opsID string
	for _, admin := range admins {
		if admin.Email == "ops@example.com" {
			opsID = admin.UserID
			if admin.Source != model.ServerAdminSourceEnv {
				t.Fatalf("env admin source = %q", admin.Source)
			}
		}
	}
	if _, err := env.instance.RevokeAdmin(ctx, first.User.ID, opsID); !errors.Is(err, ErrServerAdminFromEnv) {
		t.Fatalf("revoke env admin error = %v, want ErrServerAdminFromEnv", err)
	}
}

func TestBootstrapAssignsAdminOnUpgrade(t *testing.T) {
	t.Run("owner of the oldest organization", func(t *testing.T) {
		env := newInstanceTestEnv(t)
		ctx := context.Background()
		old := time.Now().Add(-48 * time.Hour)
		seedUser(t, env.db, "u-early", "early@example.com", "Early Invitee", "x")
		seedUser(t, env.db, "u-owner", "owner@example.com", "Owner", "x")
		mustExec(t, env.db, `UPDATE users SET created_at = ? WHERE id = 'u-early'`, old)
		mustExec(t, env.db, `INSERT INTO organizations (id, name, slug, owner_id, created_at, updated_at) VALUES ('o-1', 'Old', 'old', 'u-owner', ?, ?)`, old, old)
		mustExec(t, env.db, `INSERT INTO organizations (id, name, slug, owner_id, created_at, updated_at) VALUES ('o-2', 'New', 'new', 'u-early', ?, ?)`, time.Now(), time.Now())
		// The migration's row for an existing server: open signup, claim spent.
		mustExec(t, env.db, `INSERT INTO instance_settings (singleton, signup_mode, admin_bootstrapped_at) VALUES (1, 'open', ?)`, time.Now())

		if err := env.instance.Bootstrap(ctx); err != nil {
			t.Fatalf("bootstrap: %v", err)
		}
		admins, _ := env.instance.ListAdmins(ctx)
		if len(admins) != 1 || admins[0].UserID != "u-owner" {
			t.Fatalf("admins = %+v, want the oldest organization's owner", admins)
		}
		policy, _ := env.instance.PublicPolicy(ctx)
		if policy.Mode != model.SignupModeOpen || policy.FirstUser {
			t.Fatalf("upgraded policy = %+v, want open and no first-user claim", policy)
		}
		// Idempotent: a second start changes nothing.
		if err := env.instance.Bootstrap(ctx); err != nil {
			t.Fatal(err)
		}
		if admins, _ := env.instance.ListAdmins(ctx); len(admins) != 1 {
			t.Fatalf("admins after second bootstrap = %+v", admins)
		}
	})
	t.Run("oldest account without organizations and missing settings row", func(t *testing.T) {
		env := newInstanceTestEnv(t)
		ctx := context.Background()
		seedUser(t, env.db, "u-2", "b@example.com", "B", "x")
		seedUser(t, env.db, "u-1", "a@example.com", "A", "x")
		mustExec(t, env.db, `UPDATE users SET created_at = ? WHERE id = 'u-1'`, time.Now().Add(-time.Hour))

		if err := env.instance.Bootstrap(ctx); err != nil {
			t.Fatalf("bootstrap: %v", err)
		}
		admins, _ := env.instance.ListAdmins(ctx)
		if len(admins) != 1 || admins[0].UserID != "u-1" {
			t.Fatalf("admins = %+v, want the oldest account", admins)
		}
		if _, err := env.signup(t, "later@example.com"); err != nil {
			t.Fatalf("existing server keeps open signup: %v", err)
		}
	})
	t.Run("fresh server stays unclaimed", func(t *testing.T) {
		env := newInstanceTestEnv(t)
		ctx := context.Background()
		if err := env.instance.Bootstrap(ctx); err != nil {
			t.Fatal(err)
		}
		policy, _ := env.instance.PublicPolicy(ctx)
		if policy.Mode != model.SignupModeInviteOnly || !policy.FirstUser {
			t.Fatalf("fresh policy = %+v, want invite_only awaiting the first account", policy)
		}
	})
}

func TestUpdateSignupPolicyValidation(t *testing.T) {
	env := newInstanceTestEnv(t)
	ctx := context.Background()
	first, _ := env.signup(t, "founder@example.com")
	admin := first.User.ID

	if _, err := env.instance.UpdateSignupPolicy(ctx, admin, model.UpdateSignupPolicyRequest{Mode: "everyone"}); !errors.Is(err, ErrInvalidSignupPolicy) {
		t.Fatalf("bad mode error = %v", err)
	}
	if _, err := env.instance.UpdateSignupPolicy(ctx, admin, model.UpdateSignupPolicyRequest{Mode: model.SignupModeDomains, AllowedDomains: []string{"acme.com"}}); !errors.Is(err, ErrInvalidSignupPolicy) {
		t.Fatalf("domains without mail error = %v", err)
	}
	*env.mail = true
	if _, err := env.instance.UpdateSignupPolicy(ctx, admin, model.UpdateSignupPolicyRequest{Mode: model.SignupModeDomains}); !errors.Is(err, ErrInvalidSignupPolicy) {
		t.Fatalf("domains without a domain error = %v", err)
	}
	if _, err := env.instance.UpdateSignupPolicy(ctx, admin, model.UpdateSignupPolicyRequest{Mode: model.SignupModeDomains, AllowedDomains: []string{"not a domain"}}); !errors.Is(err, ErrInvalidSignupPolicy) {
		t.Fatalf("invalid domain error = %v", err)
	}
	policy, err := env.instance.UpdateSignupPolicy(ctx, admin, model.UpdateSignupPolicyRequest{Mode: model.SignupModeDomains, AllowedDomains: []string{" @Acme.com", "acme.com", "beta.io"}})
	if err != nil {
		t.Fatalf("valid policy: %v", err)
	}
	if strings.Join(policy.AllowedDomains, ",") != "acme.com,beta.io" || policy.RecommendInviteOnly {
		t.Fatalf("policy = %+v", policy)
	}
	public, _ := env.instance.PublicPolicy(ctx)
	if public.Mode != model.SignupModeDomains || len(public.AllowedDomains) != 2 {
		t.Fatalf("public policy = %+v", public)
	}
	policy, _ = env.instance.UpdateSignupPolicy(ctx, admin, model.UpdateSignupPolicyRequest{Mode: model.SignupModeOpen})
	if !policy.RecommendInviteOnly {
		t.Fatal("open signup should recommend invite-only")
	}
	if public, _ := env.instance.PublicPolicy(ctx); len(public.AllowedDomains) != 0 {
		t.Fatalf("public domains outside domains mode = %v", public.AllowedDomains)
	}
}

func TestSignupWithoutGateKeepsOpenSignup(t *testing.T) {
	// Enterprise wires no signup gate: signup stays open and nobody becomes a
	// server admin.
	env := newInstanceTestEnv(t)
	env.auth.SetSignupGate(nil)
	for _, email := range []string{"a@example.com", "b@example.com"} {
		resp, err := env.signup(t, email)
		if err != nil {
			t.Fatalf("signup %s: %v", email, err)
		}
		if resp.User.IsServerAdmin {
			t.Fatalf("%s became server admin without a gate", email)
		}
	}
}
