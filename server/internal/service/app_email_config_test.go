package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var appEmailTestKey = []byte("0123456789abcdef0123456789abcdef")

func newAppEmailTestService(t *testing.T, opts AppEmailConfigOptions) (*AppEmailConfigService, *repository.InstanceSettingsRepository) {
	t.Helper()
	db := newTestDB(t)
	mustExec(t, db, instanceSettingsTestSchema)
	repo := repository.NewInstanceSettingsRepository(db)
	if opts.EncryptionKey == nil {
		opts.EncryptionKey = appEmailTestKey
	}
	return NewAppEmailConfigService(repo, opts), repo
}

func TestAppEmailSettingsSaveRedactsAndEncryptsPassword(t *testing.T) {
	svc, repo := newAppEmailTestService(t, AppEmailConfigOptions{})
	ctx := context.Background()

	settings, err := svc.Settings(ctx)
	if err != nil || settings.Source != model.AppEmailSourceNone || !settings.Editable {
		t.Fatalf("initial settings = %+v, %v", settings, err)
	}

	saved, err := svc.Save(ctx, "", model.UpdateAppEmailSettingsRequest{
		Host: "smtp.example.com", Port: 587, Username: "mailer", Password: strPtr("s3cret-pass"),
		From: "Helpin <helpin@example.com>", TLSMode: "starttls",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Source != model.AppEmailSourceDatabase || !saved.PasswordSet || saved.Host != "smtp.example.com" || saved.Username != "mailer" {
		t.Fatalf("saved settings = %+v", saved)
	}

	row, err := repo.Get(ctx)
	if err != nil || row == nil || row.SMTPPasswordEncrypted == nil {
		t.Fatalf("stored row = %+v, %v", row, err)
	}
	if strings.Contains(*row.SMTPPasswordEncrypted, "s3cret-pass") {
		t.Fatal("the SMTP password must be stored encrypted")
	}

	// Saving again without a password keeps the stored one.
	kept, err := svc.Save(ctx, "", model.UpdateAppEmailSettingsRequest{
		Host: "smtp2.example.com", Port: 465, Username: "mailer", From: "helpin@example.com", TLSMode: "tls",
	})
	if err != nil || !kept.PasswordSet || kept.Port != 465 {
		t.Fatalf("resave without password = %+v, %v", kept, err)
	}

	// Clearing the password with a username set is rejected.
	if _, err := svc.Save(ctx, "", model.UpdateAppEmailSettingsRequest{
		Host: "smtp2.example.com", Port: 465, Username: "mailer", Password: strPtr(""), From: "helpin@example.com", TLSMode: "tls",
	}); !errors.Is(err, ErrInvalidAppEmailSettings) {
		t.Fatalf("clearing password with username error = %v", err)
	}

	if err := svc.Clear(ctx, ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	cleared, _ := svc.Settings(ctx)
	if cleared.Source != model.AppEmailSourceNone || cleared.PasswordSet {
		t.Fatalf("after clear = %+v", cleared)
	}
}

func TestAppEmailSettingsValidation(t *testing.T) {
	svc, _ := newAppEmailTestService(t, AppEmailConfigOptions{})
	ctx := context.Background()
	cases := []model.UpdateAppEmailSettingsRequest{
		{Host: "", From: "a@example.com"},
		{Host: "smtp://mail.example.com", From: "a@example.com"},
		{Host: "mail.example.com", Port: 70000, From: "a@example.com"},
		{Host: "mail.example.com", From: "not an address"},
		{Host: "mail.example.com", From: "a@example.com", TLSMode: "ssl"},
		{Host: "mail.example.com", From: "a@example.com", TLSMode: "none", Username: "u", Password: strPtr("p")},
	}
	for _, req := range cases {
		if _, err := svc.Save(ctx, "", req); !errors.Is(err, ErrInvalidAppEmailSettings) {
			t.Errorf("Save(%+v) error = %v, want ErrInvalidAppEmailSettings", req, err)
		}
	}
	// An unauthenticated local relay needs no encryption key.
	noKey, _ := newAppEmailTestService(t, AppEmailConfigOptions{EncryptionKey: []byte{}})
	if _, err := noKey.Save(ctx, "", model.UpdateAppEmailSettingsRequest{Host: "relay", Port: 25, From: "a@example.com", TLSMode: "none"}); err != nil {
		t.Fatalf("relay without password: %v", err)
	}
	if _, err := noKey.Save(ctx, "", model.UpdateAppEmailSettingsRequest{Host: "relay", Port: 587, Username: "u", Password: strPtr("p"), From: "a@example.com"}); !errors.Is(err, ErrInvalidAppEmailSettings) {
		t.Fatalf("password without key error = %v", err)
	}
}

func TestAppEmailSenderReloadsAfterSave(t *testing.T) {
	svc, _ := newAppEmailTestService(t, AppEmailConfigOptions{CacheTTL: time.Hour})
	ctx := context.Background()
	sender := svc.Sender()

	if sender.Configured() || email.Configured(sender) {
		t.Fatal("sender should not be configured before settings are saved")
	}
	if err := sender.SendEmail("a@example.com", "s", "<p>b</p>", "b"); !errors.Is(err, email.ErrAppEmailNotConfigured) {
		t.Fatalf("send without settings error = %v", err)
	}
	configured, fingerprint := svc.AppEmailState(ctx)
	if configured || fingerprint != "" {
		t.Fatalf("state before save = %v %q", configured, fingerprint)
	}

	if _, err := svc.Save(ctx, "", model.UpdateAppEmailSettingsRequest{Host: "smtp.example.com", Port: 587, From: "first@example.com"}); err != nil {
		t.Fatal(err)
	}
	// The same sender instance picks up the saved settings, despite the long cache TTL.
	if !sender.Configured() || sender.FromEmail() != "first@example.com" {
		t.Fatalf("after save: configured=%v from=%q", sender.Configured(), sender.FromEmail())
	}
	_, firstFingerprint := svc.AppEmailState(ctx)

	if _, err := svc.Save(ctx, "", model.UpdateAppEmailSettingsRequest{Host: "smtp.example.com", Port: 587, From: "second@example.com"}); err != nil {
		t.Fatal(err)
	}
	if sender.FromEmail() != "second@example.com" {
		t.Fatalf("after second save from = %q", sender.FromEmail())
	}
	if _, secondFingerprint := svc.AppEmailState(ctx); secondFingerprint == firstFingerprint || secondFingerprint == "" {
		t.Fatal("a new save must change the fingerprint so old test results stop applying")
	}
}

type fakeAppSender struct{ from string }

func (f fakeAppSender) SendEmail(string, string, string, string) error       { return nil }
func (f fakeAppSender) SendVerificationEmail(string, string, string) error   { return nil }
func (f fakeAppSender) SendPasswordResetEmail(string, string, string) error  { return nil }
func (f fakeAppSender) SendInviteEmail(string, string, string, string) error { return nil }
func (f fakeAppSender) FromEmail() string                                    { return f.from }

func TestAppEmailEnvironmentTakesPrecedence(t *testing.T) {
	env := email.SMTPConfig{Host: "env-smtp.example.com", Port: 2525, Username: "envuser", Password: "envpass", From: "env@example.com", TLSMode: "tls"}
	svc, repo := newAppEmailTestService(t, AppEmailConfigOptions{EnvSender: fakeAppSender{from: "env@example.com"}, EnvSMTP: env})
	ctx := context.Background()
	// Even a stored row is ignored while the environment configures email.
	if err := repo.SaveSMTP(ctx, repository.SMTPSettings{Host: "db.example.com", Port: 587, From: "db@example.com", TLSMode: "starttls"}, "", time.Now()); err != nil {
		t.Fatal(err)
	}

	settings, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Source != model.AppEmailSourceEnv || settings.Editable || settings.Host != "env-smtp.example.com" || !settings.PasswordSet {
		t.Fatalf("env settings = %+v", settings)
	}
	if svc.Sender().FromEmail() != "env@example.com" {
		t.Fatal("the environment sender must deliver")
	}
	if _, err := svc.Save(ctx, "", model.UpdateAppEmailSettingsRequest{Host: "x", From: "x@example.com"}); !errors.Is(err, ErrAppEmailSetByEnv) {
		t.Fatalf("save over env error = %v", err)
	}
	if err := svc.Clear(ctx, ""); !errors.Is(err, ErrAppEmailSetByEnv) {
		t.Fatalf("clear over env error = %v", err)
	}
	// The fingerprint matches the one used before in-app settings existed.
	_, fingerprint := svc.AppEmailState(ctx)
	if want := AppEmailFingerprint("smtp", "env-smtp.example.com", "2525", "envuser", "env@example.com", "tls"); fingerprint != want {
		t.Fatalf("env fingerprint = %q, want %q", fingerprint, want)
	}

	postmark, _ := newAppEmailTestService(t, AppEmailConfigOptions{EnvSender: fakeAppSender{from: "pm@example.com"}})
	pm, _ := postmark.Settings(ctx)
	if pm.Provider != "postmark" || pm.From != "pm@example.com" || pm.Editable {
		t.Fatalf("postmark settings = %+v", pm)
	}
}

func TestAppEmailUnreadablePasswordReportsProblem(t *testing.T) {
	svc, repo := newAppEmailTestService(t, AppEmailConfigOptions{})
	ctx := context.Background()
	garbage := "not-ciphertext"
	if err := repo.SaveSMTP(ctx, repository.SMTPSettings{Host: "smtp.example.com", Port: 587, Username: "u", PasswordEncrypted: &garbage, From: "a@example.com", TLSMode: "starttls"}, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	settings, err := svc.Settings(ctx)
	if err != nil || settings.Problem == "" {
		t.Fatalf("settings = %+v, %v; want a problem", settings, err)
	}
	if svc.Sender().Configured() {
		t.Fatal("unusable stored settings must not count as configured")
	}
}

func TestInviteWithoutEmailReturnsShareableLink(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, instanceSettingsTestSchema)
	seedUser(t, db, "owner-1", "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, "ws-1", "Team", "team", "owner-1")
	seedWorkspaceMember(t, db, "wm-1", "ws-1", "owner-1", "owner@example.com", "Owner", "owner")
	mailer, _ := newAppEmailTestService(t, AppEmailConfigOptions{})
	svc := NewInviteService(repository.NewInvitationRepository(db), repository.NewWorkspaceRepository(db), nil,
		repository.NewUserRepository(db), repository.NewSettingsRepository(db), mailer.Sender(), "https://helpin.example.com", nil)

	resp, err := svc.CreateInvitation(context.Background(), model.CreateInvitationRequest{WorkspaceID: "ws-1", Email: "new@example.com", Role: "member"}, "owner-1")
	if err != nil {
		t.Fatalf("create invitation without email: %v", err)
	}
	if resp.EmailSent == nil || *resp.EmailSent {
		t.Fatalf("EmailSent = %v, want false", resp.EmailSent)
	}
	token := strings.TrimPrefix(resp.JoinURL, "https://helpin.example.com/join/")
	if token == resp.JoinURL || len(token) != 64 {
		t.Fatalf("JoinURL = %q, want a 32-byte hex token link", resp.JoinURL)
	}
	if err := svc.ResendInvitation(context.Background(), resp.ID, "owner-1"); err == nil {
		t.Fatal("resend without email should explain that the link must be shared")
	}

	withEmail := NewInviteService(repository.NewInvitationRepository(db), repository.NewWorkspaceRepository(db), nil,
		repository.NewUserRepository(db), repository.NewSettingsRepository(db), stubInviteEmailSender{}, "https://helpin.example.com", nil)
	sent, err := withEmail.CreateInvitation(context.Background(), model.CreateInvitationRequest{WorkspaceID: "ws-1", Email: "other@example.com", Role: "member"}, "owner-1")
	if err != nil || sent.EmailSent == nil || !*sent.EmailSent {
		t.Fatalf("invitation with email = %+v, %v", sent, err)
	}
}
