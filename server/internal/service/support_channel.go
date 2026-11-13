package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// Channel errors returned to handlers.
var (
	ErrChannelInvalid               = errors.New("invalid channel request")
	ErrChannelTokenRejected         = errors.New("the provider rejected this token")
	ErrChannelAlreadyConnected      = errors.New("this account is already connected")
	ErrChannelEncryptionUnavailable = errors.New("channel encryption key not configured")
	ErrChannelPublicURLUnavailable  = errors.New("no public HTTPS API URL configured")
	ErrChannelProviderUnavailable   = errors.New("the provider could not be reached")
	ErrChannelWebhookUnauthorized   = errors.New("webhook secret mismatch")
	ErrChannelWebhookInvalid        = errors.New("webhook payload could not be read")
)

// SupportInboundKindChannelEvent is the inbound job kind for provider events.
const SupportInboundKindChannelEvent = "channel_event"

// SupportChannelConfig configures connected channels.
type SupportChannelConfig struct {
	// EncryptionKey is AES-256 (32 bytes); without it nothing can connect.
	EncryptionKey []byte
	// PublicAPIBaseURL is the externally reachable API origin, without /api.
	PublicAPIBaseURL string
	// RequireHTTPS rejects a plain-HTTP webhook URL. Providers like Telegram
	// insist on HTTPS; only local fakes run without it.
	RequireHTTPS bool
}

// SupportChannelService connects provider accounts and accepts their webhooks.
type SupportChannelService struct {
	accounts *repository.SupportChannelAccountRepository
	jobs     *repository.SupportInboundJobRepository
	adapters *channels.Registry
	inbox    *SupportInboxService
	outbox   *repository.SupportChannelOutboxRepository
	cfg      SupportChannelConfig
	now      func() time.Time
	logger   *slog.Logger
}

// NewSupportChannelService creates the service.
func NewSupportChannelService(accounts *repository.SupportChannelAccountRepository, jobs *repository.SupportInboundJobRepository, adapters *channels.Registry, cfg SupportChannelConfig) *SupportChannelService {
	cfg.PublicAPIBaseURL = strings.TrimRight(strings.TrimSpace(cfg.PublicAPIBaseURL), "/")
	return &SupportChannelService{
		accounts: accounts, jobs: jobs, adapters: adapters, cfg: cfg,
		now:    func() time.Time { return time.Now().UTC() },
		logger: slog.Default().With("service", "support_channel"),
	}
}

// List returns the workspace's connected accounts.
func (s *SupportChannelService) List(ctx context.Context, workspaceID string) ([]model.SupportChannelAccountView, error) {
	return s.accounts.List(ctx, workspaceID)
}

// Connect validates a token, registers the webhook, and stores the account.
// Nothing is stored if any step fails.
func (s *SupportChannelService) Connect(ctx context.Context, workspaceID, actorID string, req model.ConnectSupportChannelRequest) (*model.SupportChannelAccount, error) {
	adapter, ok := s.adapters.Lookup(strings.TrimSpace(req.Provider))
	if !ok {
		return nil, fmt.Errorf("%w: unsupported provider", ErrChannelInvalid)
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return nil, fmt.Errorf("%w: token is required", ErrChannelInvalid)
	}
	if len(s.cfg.EncryptionKey) != 32 {
		return nil, ErrChannelEncryptionUnavailable
	}
	mailboxID, err := s.mailboxSelection(ctx, workspaceID, req.MailboxID)
	if err != nil {
		return nil, err
	}
	info, err := s.validate(ctx, adapter, token)
	if err != nil {
		return nil, err
	}
	// Checked before setWebhook: registering would steal the other
	// workspace's webhook.
	if live, err := s.accounts.FindLive(ctx, adapter.Provider(), info.ExternalAccountID); err != nil {
		return nil, err
	} else if live != nil {
		return nil, ErrChannelAlreadyConnected
	}

	account := &model.SupportChannelAccount{
		ID: uuid.NewString(), WorkspaceID: workspaceID, Provider: adapter.Provider(),
		ExternalAccountID: info.ExternalAccountID, DisplayName: info.DisplayName, Handle: info.Handle,
		MailboxID: mailboxID, Status: model.SupportChannelAccountActive, ConnectedBy: nilIfEmpty(actorID),
	}
	if account.TokenEncrypted, err = crypto.EncryptStringWithAAD(token, s.cfg.EncryptionKey, channelTokenAAD(account.ID)); err != nil {
		return nil, fmt.Errorf("encrypt channel token: %w", err)
	}
	secret, hash, err := newWebhookSecret()
	if err != nil {
		return nil, err
	}
	account.WebhookSecretHash = hash
	webhookURL, err := s.webhookURL(adapter.Provider(), account.ID)
	if err != nil {
		return nil, err
	}
	if err := s.registerWebhook(ctx, adapter, token, webhookURL, secret); err != nil {
		return nil, err
	}
	if err := s.accounts.Create(ctx, account); err != nil {
		// The webhook now points at an account that was never stored; take it
		// back so the provider stops calling.
		if unregisterErr := adapter.UnregisterWebhook(context.WithoutCancel(ctx), token); unregisterErr != nil {
			s.logger.ErrorContext(ctx, "unregister webhook after failed connect", "error", unregisterErr, "workspace_id", workspaceID)
		}
		if isUniqueConstraintViolation(err) {
			return nil, ErrChannelAlreadyConnected
		}
		return nil, fmt.Errorf("store channel account: %w", err)
	}
	s.logger.InfoContext(ctx, "channel connected", "workspace_id", workspaceID, "account_id", account.ID, "provider", account.Provider)
	return account, nil
}

// Update changes an account's default mailbox or AI replies switch.
func (s *SupportChannelService) Update(ctx context.Context, workspaceID, id string, req model.UpdateSupportChannelRequest) (*model.SupportChannelAccount, error) {
	updates := map[string]any{}
	if req.MailboxID != nil {
		mailboxID, err := s.mailboxSelection(ctx, workspaceID, req.MailboxID)
		if err != nil {
			return nil, err
		}
		updates["mailbox_id"] = mailboxID
	}
	if req.AIRepliesDisabled != nil {
		updates["ai_replies_disabled"] = *req.AIRepliesDisabled
	}
	if len(updates) > 0 {
		if err := s.accounts.Update(ctx, workspaceID, id, updates); err != nil {
			return nil, err
		}
	}
	return s.accounts.Get(ctx, workspaceID, id)
}

// Reconnect registers a fresh webhook secret, optionally with a new token for
// the same bot. The old secret stops working immediately.
func (s *SupportChannelService) Reconnect(ctx context.Context, workspaceID, id string, req model.ReconnectSupportChannelRequest) (*model.SupportChannelAccount, error) {
	account, adapter, token, err := s.liveAccount(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if replacement := strings.TrimSpace(req.Token); replacement != "" {
		info, err := s.validate(ctx, adapter, replacement)
		if err != nil {
			return nil, err
		}
		if info.ExternalAccountID != account.ExternalAccountID {
			return nil, fmt.Errorf("%w: the new token belongs to a different bot; connect it separately", ErrChannelInvalid)
		}
		token = replacement
		if updates["token_encrypted"], err = crypto.EncryptStringWithAAD(token, s.cfg.EncryptionKey, channelTokenAAD(account.ID)); err != nil {
			return nil, fmt.Errorf("encrypt channel token: %w", err)
		}
		updates["display_name"], updates["handle"] = info.DisplayName, info.Handle
	}
	secret, hash, err := newWebhookSecret()
	if err != nil {
		return nil, err
	}
	webhookURL, err := s.webhookURL(adapter.Provider(), account.ID)
	if err != nil {
		return nil, err
	}
	if err := s.registerWebhook(ctx, adapter, token, webhookURL, secret); err != nil {
		return nil, err
	}
	updates["webhook_secret_hash"] = hash
	updates["status"] = model.SupportChannelAccountActive
	updates["auth_error_count"] = 0
	updates["last_error_code"], updates["last_error_message"] = nil, nil
	if err := s.accounts.Update(ctx, workspaceID, id, updates); err != nil {
		return nil, err
	}
	return s.accounts.Get(ctx, workspaceID, id)
}

// Disconnect removes the webhook on a best-effort basis and disables the
// account. Undelivered replies are cancelled; history stays.
func (s *SupportChannelService) Disconnect(ctx context.Context, workspaceID, id string) error {
	account, adapter, token, err := s.liveAccount(ctx, workspaceID, id)
	if err != nil && !errors.Is(err, ErrChannelEncryptionUnavailable) {
		return err
	}
	if adapter != nil && token != "" {
		if err := adapter.UnregisterWebhook(ctx, token); err != nil {
			s.logger.WarnContext(ctx, "unregister webhook on disconnect", "error", err, "workspace_id", workspaceID, "account_id", id)
		}
	}
	if account == nil {
		if account, err = s.accounts.Get(ctx, workspaceID, id); err != nil {
			return err
		}
	}
	if err := s.accounts.Disconnect(ctx, workspaceID, id, s.now()); err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "channel disconnected", "workspace_id", workspaceID, "account_id", id, "provider", account.Provider)
	return nil
}

// channelEventPayload is the stored inbound job payload.
type channelEventPayload struct {
	AccountID string         `json:"account_id"`
	Provider  string         `json:"provider"`
	Event     channels.Event `json:"event"`
}

// AcceptWebhook verifies a provider webhook and stores each event as an
// inbound job. Unknown or disconnected accounts are acknowledged and dropped;
// it returns nil only after the jobs are committed.
func (s *SupportChannelService) AcceptWebhook(ctx context.Context, provider, accountID string, header http.Header, body []byte) error {
	adapter, ok := s.adapters.Lookup(provider)
	if !ok || !isUUIDString(accountID) {
		return nil
	}
	account, err := s.accounts.GetForWebhook(ctx, provider, accountID)
	if err != nil {
		return err
	}
	if account == nil {
		return nil
	}
	presented := sha256.Sum256([]byte(adapter.PresentedSecret(header)))
	expected, err := hex.DecodeString(account.WebhookSecretHash)
	if err != nil || subtle.ConstantTimeCompare(presented[:], expected) != 1 {
		return ErrChannelWebhookUnauthorized
	}
	events, err := adapter.ParseWebhook(body)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrChannelWebhookInvalid, err)
	}
	now := s.now()
	for _, event := range events {
		if event.Kind == channels.EventIgnored {
			s.logger.DebugContext(ctx, "channel event ignored", "account_id", account.ID, "reason", event.IgnoredReason)
			continue
		}
		payload, err := json.Marshal(channelEventPayload{AccountID: account.ID, Provider: provider, Event: event})
		if err != nil {
			return err
		}
		subject := ChannelSubjectKey(account.ID, event.ExternalUserID)
		job := &model.SupportInboundJob{
			ID:               inboundStableID(provider + ":" + account.ID + ":" + event.EventKey),
			Kind:             SupportInboundKindChannelEvent,
			WorkspaceID:      &account.WorkspaceID,
			ChannelAccountID: &account.ID,
			SubjectKey:       &subject,
			Payload:          string(payload),
			Status:           "pending",
			AvailableAt:      now,
		}
		if err := s.jobs.Create(ctx, job); err != nil {
			return fmt.Errorf("store channel event: %w", err)
		}
	}
	if err := s.accounts.TouchEvent(ctx, account.ID, now); err != nil {
		s.logger.WarnContext(ctx, "record channel event time", "error", err, "account_id", account.ID)
	}
	return nil
}

// ChannelSubjectKey identifies a customer of a connected account without
// storing their provider id in clear on every job.
func ChannelSubjectKey(accountID, externalUserID string) string {
	sum := sha256.Sum256([]byte(accountID + ":" + externalUserID))
	return hex.EncodeToString(sum[:])
}

// liveAccount loads a connected account with its adapter and decrypted token.
func (s *SupportChannelService) liveAccount(ctx context.Context, workspaceID, id string) (*model.SupportChannelAccount, channels.Adapter, string, error) {
	account, err := s.accounts.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, "", err
	}
	if account.DisabledAt != nil {
		return nil, nil, "", repository.ErrSupportChannelAccountNotFound
	}
	adapter, ok := s.adapters.Lookup(account.Provider)
	if !ok {
		return account, nil, "", fmt.Errorf("%w: unsupported provider", ErrChannelInvalid)
	}
	token, err := s.decryptToken(account)
	if err != nil {
		return account, adapter, "", err
	}
	return account, adapter, token, nil
}

// decryptToken returns the account's provider token.
func (s *SupportChannelService) decryptToken(account *model.SupportChannelAccount) (string, error) {
	if len(s.cfg.EncryptionKey) != 32 {
		return "", ErrChannelEncryptionUnavailable
	}
	token, err := crypto.DecryptStringWithAAD(account.TokenEncrypted, s.cfg.EncryptionKey, channelTokenAAD(account.ID))
	if err != nil {
		return "", fmt.Errorf("decrypt channel token: %w", err)
	}
	return token, nil
}

func (s *SupportChannelService) validate(ctx context.Context, adapter channels.Adapter, token string) (channels.AccountInfo, error) {
	info, err := adapter.ValidateToken(ctx, token)
	switch {
	case errors.Is(err, channels.ErrAuth):
		return channels.AccountInfo{}, ErrChannelTokenRejected
	case err != nil:
		return channels.AccountInfo{}, fmt.Errorf("%w: %v", ErrChannelProviderUnavailable, err)
	}
	return info, nil
}

func (s *SupportChannelService) registerWebhook(ctx context.Context, adapter channels.Adapter, token, webhookURL, secret string) error {
	err := adapter.RegisterWebhook(ctx, token, webhookURL, secret)
	switch {
	case errors.Is(err, channels.ErrAuth):
		return ErrChannelTokenRejected
	case err != nil:
		return fmt.Errorf("%w: %v", ErrChannelProviderUnavailable, err)
	}
	return nil
}

func (s *SupportChannelService) mailboxSelection(ctx context.Context, workspaceID string, mailboxID *string) (*string, error) {
	if mailboxID == nil || strings.TrimSpace(*mailboxID) == "" {
		return nil, nil
	}
	id := strings.TrimSpace(*mailboxID)
	if !isUUIDString(id) {
		return nil, fmt.Errorf("%w: unknown inbox", ErrChannelInvalid)
	}
	ok, err := s.accounts.MailboxInWorkspace(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: unknown inbox", ErrChannelInvalid)
	}
	return &id, nil
}

func (s *SupportChannelService) webhookURL(provider, accountID string) (string, error) {
	base, err := url.Parse(s.cfg.PublicAPIBaseURL)
	if s.cfg.PublicAPIBaseURL == "" || err != nil || base.Host == "" {
		return "", ErrChannelPublicURLUnavailable
	}
	if s.cfg.RequireHTTPS && base.Scheme != "https" {
		return "", ErrChannelPublicURLUnavailable
	}
	return s.cfg.PublicAPIBaseURL + "/api/webhooks/channels/" + url.PathEscape(provider) + "/" + accountID, nil
}

func channelTokenAAD(accountID string) []byte {
	return []byte("channel-token:" + accountID)
}

// newWebhookSecret returns a secret for the provider and its stored hash.
// base64url fits Telegram's allowed characters (A-Z, a-z, 0-9, _ and -).
func newWebhookSecret() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate webhook secret: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(secret))
	return secret, hex.EncodeToString(sum[:]), nil
}

func isUUIDString(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil && len(value) == 36
}
