package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// _appEmailCacheTTL bounds how long another API replica may keep using stale
// SMTP settings after a save. Saves in the same process apply immediately.
const _appEmailCacheTTL = 30 * time.Second

// _appEmailResolveTimeout bounds the settings read made while sending mail.
const _appEmailResolveTimeout = 5 * time.Second

var (
	// ErrAppEmailSetByEnv rejects changes to settings owned by the environment.
	ErrAppEmailSetByEnv = errors.New("application email is set by the server's configuration (SMTP_* or POSTMARK_* variables); change it there")
	// ErrInvalidAppEmailSettings wraps SMTP settings validation failures.
	ErrInvalidAppEmailSettings = errors.New("invalid email settings")
)

type appEmailSettingsStore interface {
	Get(ctx context.Context) (*model.InstanceSettings, error)
	SaveSMTP(ctx context.Context, smtp repository.SMTPSettings, updatedBy string, now time.Time) error
	ClearSMTP(ctx context.Context, updatedBy string, now time.Time) error
}

// AppEmailConfigOptions configures AppEmailConfigService.
type AppEmailConfigOptions struct {
	// EnvSender is the sender built from SMTP_* or POSTMARK_* variables; when
	// set it takes precedence over stored settings.
	EnvSender email.AppSender
	// EnvSMTP holds the SMTP_* values, shown read-only when they are in use.
	EnvSMTP email.SMTPConfig
	// EncryptionKey is the 32-byte key for the stored SMTP password.
	EncryptionKey []byte
	// CacheTTL overrides _appEmailCacheTTL; tests only.
	CacheTTL time.Duration
}

// AppEmailConfigService resolves application email from the environment or
// from SMTP settings saved in the app, and rebuilds the sender when the saved
// settings change so no restart is needed.
type AppEmailConfigService struct {
	store appEmailSettingsStore
	opts  AppEmailConfigOptions
	now   func() time.Time

	mu       sync.Mutex
	cached   storedAppEmail
	cachedAt time.Time
	hasCache bool
}

// storedAppEmail is the sender built from the settings row.
type storedAppEmail struct {
	sender      email.AppSender
	fingerprint string
	problem     string
}

// NewAppEmailConfigService returns the application email source.
func NewAppEmailConfigService(store appEmailSettingsStore, opts AppEmailConfigOptions) *AppEmailConfigService {
	opts.EncryptionKey = append([]byte(nil), opts.EncryptionKey...)
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = _appEmailCacheTTL
	}
	return &AppEmailConfigService{store: store, opts: opts, now: time.Now}
}

// Sender returns an application sender that always delivers through the
// current configuration.
func (s *AppEmailConfigService) Sender() *email.DynamicAppSender {
	return email.NewDynamicAppSender(func() email.AppSender {
		ctx, cancel := context.WithTimeout(context.Background(), _appEmailResolveTimeout)
		defer cancel()
		sender, _, _ := s.resolve(ctx)
		return sender
	})
}

// AppEmailState reports whether application email is configured and a
// fingerprint of its non-secret settings, so a recorded test email stops
// applying when the settings change.
func (s *AppEmailConfigService) AppEmailState(ctx context.Context) (bool, string) {
	sender, _, fingerprint := s.resolve(ctx)
	return sender != nil, fingerprint
}

// Invalidate drops the cached stored settings.
func (s *AppEmailConfigService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasCache = false
	s.cached = storedAppEmail{}
}

// Settings describes the active application email settings without secrets.
func (s *AppEmailConfigService) Settings(ctx context.Context) (*model.AppEmailSettingsResponse, error) {
	if s.opts.EnvSender != nil {
		return s.envSettings(), nil
	}
	row, err := s.store.Get(ctx)
	if err != nil {
		return nil, err
	}
	response := &model.AppEmailSettingsResponse{Source: model.AppEmailSourceNone, Editable: true, Port: 587, TLSMode: "starttls"}
	if row == nil || strings.TrimSpace(derefString(row.SMTPHost)) == "" {
		return response, nil
	}
	stored := s.stored(ctx)
	response.Source = model.AppEmailSourceDatabase
	response.Provider = "smtp"
	response.Host = derefString(row.SMTPHost)
	response.Port = derefInt(row.SMTPPort)
	response.Username = derefString(row.SMTPUsername)
	response.From = derefString(row.SMTPFrom)
	response.TLSMode = derefString(row.SMTPTLSMode)
	response.PasswordSet = strings.TrimSpace(derefString(row.SMTPPasswordEncrypted)) != ""
	response.Problem = stored.problem
	return response, nil
}

// Save validates and stores SMTP settings, then applies them immediately.
func (s *AppEmailConfigService) Save(ctx context.Context, actorID string, req model.UpdateAppEmailSettingsRequest) (*model.AppEmailSettingsResponse, error) {
	if s.opts.EnvSender != nil {
		return nil, ErrAppEmailSetByEnv
	}
	config := email.SMTPConfig{
		Host: strings.TrimSpace(req.Host), Port: req.Port, Username: strings.TrimSpace(req.Username),
		From: strings.TrimSpace(req.From), TLSMode: strings.TrimSpace(strings.ToLower(req.TLSMode)),
	}
	if config.Port == 0 {
		config.Port = 587
	}
	if config.TLSMode == "" {
		config.TLSMode = "starttls"
	}
	row, err := s.store.Get(ctx)
	if err != nil {
		return nil, err
	}
	encrypted, err := s.passwordFor(row, config, req.Password)
	if err != nil {
		return nil, err
	}
	if encrypted != nil {
		// Validate with the real password so the auth combination is checked.
		config.Password, err = s.decryptPassword(*encrypted)
		if err != nil {
			return nil, err
		}
	}
	if err := validateSMTPSettings(config); err != nil {
		return nil, err
	}
	if _, err := email.NewSMTPClient(config); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidAppEmailSettings, err.Error())
	}
	stored := repository.SMTPSettings{
		Host: config.Host, Port: config.Port, Username: config.Username, From: config.From,
		TLSMode: config.TLSMode, PasswordEncrypted: encrypted,
	}
	if err := s.store.SaveSMTP(ctx, stored, actorID, s.now().UTC()); err != nil {
		return nil, err
	}
	s.Invalidate()
	slog.InfoContext(ctx, "application email settings saved", "user_id", actorID, "host", config.Host, "port", config.Port)
	return s.Settings(ctx)
}

// Clear removes the stored SMTP settings.
func (s *AppEmailConfigService) Clear(ctx context.Context, actorID string) error {
	if s.opts.EnvSender != nil {
		return ErrAppEmailSetByEnv
	}
	if err := s.store.ClearSMTP(ctx, actorID, s.now().UTC()); err != nil {
		return err
	}
	s.Invalidate()
	slog.InfoContext(ctx, "application email settings cleared", "user_id", actorID)
	return nil
}

// passwordFor returns the ciphertext to store: nil password keeps the stored
// one, "" clears it, anything else is encrypted.
func (s *AppEmailConfigService) passwordFor(row *model.InstanceSettings, config email.SMTPConfig, password *string) (*string, error) {
	if password == nil {
		if config.Username == "" || row == nil || strings.TrimSpace(derefString(row.SMTPPasswordEncrypted)) == "" {
			return nil, nil
		}
		existing := *row.SMTPPasswordEncrypted
		return &existing, nil
	}
	if *password == "" {
		return nil, nil
	}
	if len(s.opts.EncryptionKey) != 32 {
		return nil, fmt.Errorf("%w: the server has no CRM_ENCRYPTION_KEY to protect the SMTP password", ErrInvalidAppEmailSettings)
	}
	ciphertext, err := appcrypto.EncryptStringWithAAD(*password, s.opts.EncryptionKey, appEmailPasswordAAD())
	if err != nil {
		return nil, fmt.Errorf("encrypt smtp password: %w", err)
	}
	return &ciphertext, nil
}

func (s *AppEmailConfigService) decryptPassword(ciphertext string) (string, error) {
	if len(s.opts.EncryptionKey) != 32 {
		return "", fmt.Errorf("%w: the server has no CRM_ENCRYPTION_KEY to read the stored SMTP password", ErrInvalidAppEmailSettings)
	}
	plaintext, err := appcrypto.DecryptStringWithAAD(ciphertext, s.opts.EncryptionKey, appEmailPasswordAAD())
	if err != nil {
		return "", fmt.Errorf("%w: the stored SMTP password can't be read; enter it again", ErrInvalidAppEmailSettings)
	}
	return plaintext, nil
}

// resolve returns the active sender, its source and fingerprint.
func (s *AppEmailConfigService) resolve(ctx context.Context) (email.AppSender, string, string) {
	if s.opts.EnvSender != nil {
		return s.opts.EnvSender, model.AppEmailSourceEnv, s.envFingerprint()
	}
	stored := s.stored(ctx)
	if stored.sender == nil {
		return nil, model.AppEmailSourceNone, ""
	}
	return stored.sender, model.AppEmailSourceDatabase, stored.fingerprint
}

func (s *AppEmailConfigService) stored(ctx context.Context) storedAppEmail {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hasCache && s.now().Sub(s.cachedAt) < s.opts.CacheTTL {
		return s.cached
	}
	row, err := s.store.Get(ctx)
	if err != nil {
		// Keep serving the last good settings through a transient read failure.
		slog.WarnContext(ctx, "read application email settings failed", "error", err)
		return s.cached
	}
	s.cached = s.build(ctx, row)
	s.cachedAt = s.now()
	s.hasCache = true
	return s.cached
}

func (s *AppEmailConfigService) build(ctx context.Context, row *model.InstanceSettings) storedAppEmail {
	if row == nil || strings.TrimSpace(derefString(row.SMTPHost)) == "" {
		return storedAppEmail{}
	}
	config := email.SMTPConfig{
		Host: derefString(row.SMTPHost), Port: derefInt(row.SMTPPort), Username: derefString(row.SMTPUsername),
		From: derefString(row.SMTPFrom), TLSMode: derefString(row.SMTPTLSMode),
	}
	if ciphertext := strings.TrimSpace(derefString(row.SMTPPasswordEncrypted)); ciphertext != "" {
		password, err := s.decryptPassword(ciphertext)
		if err != nil {
			slog.ErrorContext(ctx, "stored smtp password unreadable", "error", err)
			return storedAppEmail{problem: "The stored SMTP password can't be read, possibly because CRM_ENCRYPTION_KEY changed. Enter it again."}
		}
		config.Password = password
	}
	sender, err := email.NewSMTPClient(config)
	if err != nil {
		slog.ErrorContext(ctx, "stored smtp settings invalid", "error", err)
		return storedAppEmail{problem: "The saved SMTP settings are incomplete. Review and save them again."}
	}
	updated := ""
	if row.SMTPUpdatedAt != nil {
		updated = strconv.FormatInt(row.SMTPUpdatedAt.UnixNano(), 10)
	}
	fingerprint := AppEmailFingerprint("smtp-db", config.Host, strconv.Itoa(config.Port), config.Username, config.From, config.TLSMode, updated)
	return storedAppEmail{sender: sender, fingerprint: fingerprint}
}

func (s *AppEmailConfigService) envSettings() *model.AppEmailSettingsResponse {
	response := &model.AppEmailSettingsResponse{Source: model.AppEmailSourceEnv}
	env := s.opts.EnvSMTP
	if strings.TrimSpace(env.Host) != "" {
		response.Provider = "smtp"
		response.Host = env.Host
		response.Port = env.Port
		if response.Port == 0 {
			response.Port = 587
		}
		response.Username = env.Username
		response.From = env.From
		response.TLSMode = env.TLSMode
		if response.TLSMode == "" {
			response.TLSMode = "starttls"
		}
		response.PasswordSet = env.Password != ""
		return response
	}
	response.Provider = "postmark"
	response.From = s.opts.EnvSender.FromEmail()
	return response
}

// envFingerprint matches the fingerprint used before settings could be
// saved in the app, so earlier test results still apply.
func (s *AppEmailConfigService) envFingerprint() string {
	env := s.opts.EnvSMTP
	if strings.TrimSpace(env.Host) != "" {
		return AppEmailFingerprint("smtp", env.Host, strconv.Itoa(env.Port), env.Username, env.From, env.TLSMode)
	}
	return AppEmailFingerprint("provider", s.opts.EnvSender.FromEmail())
}

func validateSMTPSettings(config email.SMTPConfig) error {
	switch {
	case config.Host == "" || strings.ContainsAny(config.Host, "/\r\n :"):
		return fmt.Errorf("%w: enter the mail server's hostname, without a port or scheme", ErrInvalidAppEmailSettings)
	case config.Port < 1 || config.Port > 65535:
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidAppEmailSettings)
	case config.TLSMode != "starttls" && config.TLSMode != "tls" && config.TLSMode != "none":
		return fmt.Errorf("%w: security must be STARTTLS, TLS or none", ErrInvalidAppEmailSettings)
	case (config.Username == "") != (config.Password == ""):
		return fmt.Errorf("%w: enter both a username and a password, or neither", ErrInvalidAppEmailSettings)
	case config.TLSMode == "none" && config.Username != "":
		return fmt.Errorf("%w: sign-in requires STARTTLS or TLS", ErrInvalidAppEmailSettings)
	}
	if _, err := mail.ParseAddress(config.From); err != nil || strings.ContainsAny(config.From, "\r\n") {
		return fmt.Errorf("%w: enter a valid sender address, such as Helpin <helpin@example.com>", ErrInvalidAppEmailSettings)
	}
	return nil
}

func appEmailPasswordAAD() []byte { return []byte("instance_settings:smtp_password") }

// appEmailReady reports whether an application mail sender can deliver now:
// false for a nil sender and for a dynamic sender with no current settings.
func appEmailReady(sender any) bool {
	if sender == nil {
		return false
	}
	if dynamic, ok := sender.(interface{ Configured() bool }); ok {
		return dynamic.Configured()
	}
	return true
}
