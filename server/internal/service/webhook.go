package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/netguard"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// ErrWebhookInvalid wraps a validation message for an endpoint.
var ErrWebhookInvalid = errors.New("invalid webhook")

// ErrWebhookSecretsUnavailable means no encryption key is configured.
var ErrWebhookSecretsUnavailable = errors.New("webhook signing secrets need WEBHOOK_ENCRYPTION_KEY or CRM_ENCRYPTION_KEY")

func invalidWebhook(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrWebhookInvalid, fmt.Sprintf(format, args...))
}

// WebhookConfig configures outbound webhooks.
type WebhookConfig struct {
	// EncryptionKey is a 32-byte AES key for signing secrets.
	EncryptionKey []byte
	// AllowPrivateNetworks permits private addresses and plain HTTP.
	AllowPrivateNetworks bool
	// AppBaseURL builds conversation links in payloads.
	AppBaseURL string
}

// WebhookService manages workspace webhook endpoints and delivers signed
// support events to them. Database triggers and the server record events in
// an outbox; a worker turns them into durable deliveries and sends them with
// retries for about a day.
type WebhookService struct {
	repo   *repository.WebhookRepository
	inbox  *SupportInboxService
	sla    *SupportSLAService
	cfg    WebhookConfig
	client *http.Client
	now    func() time.Time
	// concurrency bounds parallel sends in one pass.
	concurrency int
}

func NewWebhookService(repo *repository.WebhookRepository, inbox *SupportInboxService, cfg WebhookConfig) *WebhookService {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           netguard.DialContext(cfg.AllowPrivateNetworks),
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       60 * time.Second,
		MaxIdleConnsPerHost:   4,
	}
	return &WebhookService{
		repo: repo, inbox: inbox, cfg: cfg,
		client: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
			// A redirect could point anywhere; receivers must answer directly.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now:         func() time.Time { return time.Now().UTC() },
		concurrency: 8,
	}
}

// SetSLAService adds SLA status to conversation payloads.
func (s *WebhookService) SetSLAService(sla *SupportSLAService) *WebhookService {
	s.sla = sla
	return s
}

// ValidateWebhookURL checks an endpoint URL before it is stored. DNS is
// checked again on every connection.
func ValidateWebhookURL(raw string, allowPrivate bool) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || raw == "" || len(raw) > 2048 || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return "", invalidWebhook("enter a valid URL")
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && allowPrivate:
	default:
		return "", invalidWebhook("the URL must use HTTPS")
	}
	host := u.Hostname()
	if !allowPrivate {
		if netguard.IsLocalHostname(host) {
			return "", invalidWebhook("the URL must be reachable on the public Internet")
		}
		if ip := net.ParseIP(host); ip != nil && netguard.IsNonPublicIP(ip) {
			return "", invalidWebhook("the URL must be reachable on the public Internet")
		}
	}
	return u.String(), nil
}

func (s *WebhookService) normalize(ctx context.Context, workspaceID string, req model.WebhookEndpointRequest, endpoint *model.WebhookEndpoint) error {
	address, err := ValidateWebhookURL(req.URL, s.cfg.AllowPrivateNetworks)
	if err != nil {
		return err
	}
	description := strings.Join(strings.Fields(req.Description), " ")
	if len([]rune(description)) > 200 {
		return invalidWebhook("the description can be at most 200 characters")
	}
	events := model.DocsStringArray{}
	for _, event := range req.Events {
		event = strings.TrimSpace(event)
		if event == "" || slices.Contains([]string(events), event) {
			continue
		}
		if !model.IsWebhookEventType(event) {
			return invalidWebhook("unknown event %q", event)
		}
		events = append(events, event)
	}
	mailboxes := model.DocsStringArray{}
	if len(req.MailboxIDs) > 0 {
		known := map[string]bool{"shared": true}
		// Inboxes the endpoint already uses stay valid after archiving, so
		// the endpoint can still be edited or turned off.
		for _, id := range endpoint.MailboxIDs {
			known[id] = true
		}
		if s.inbox != nil && s.inbox.mailboxRepo != nil {
			active, err := s.inbox.mailboxRepo.ListByWorkspace(ctx, workspaceID, false)
			if err != nil {
				return err
			}
			for _, mailbox := range active {
				known[mailbox.ID] = true
			}
		}
		for _, id := range req.MailboxIDs {
			id = strings.TrimSpace(id)
			if id == "" || slices.Contains([]string(mailboxes), id) {
				continue
			}
			if !known[id] {
				return invalidWebhook("unknown inbox %q", id)
			}
			mailboxes = append(mailboxes, id)
		}
	}
	endpoint.URL, endpoint.Description, endpoint.Events, endpoint.MailboxIDs = address, description, events, mailboxes
	return nil
}

func webhookSecretAAD(endpointID string) []byte { return []byte("webhook-secret:" + endpointID) }

// newSecret creates a signing secret and stores it encrypted on endpoint.
func (s *WebhookService) newSecret(endpoint *model.WebhookEndpoint) (string, error) {
	if len(s.cfg.EncryptionKey) != 32 {
		return "", ErrWebhookSecretsUnavailable
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}
	secret := "whsec_" + base64.RawURLEncoding.EncodeToString(raw)
	encrypted, err := appcrypto.EncryptStringWithAAD(secret, s.cfg.EncryptionKey, webhookSecretAAD(endpoint.ID))
	if err != nil {
		return "", fmt.Errorf("encrypt webhook secret: %w", err)
	}
	endpoint.EncryptedSecret, endpoint.SecretHint = encrypted, secret[len(secret)-4:]
	return secret, nil
}

func (s *WebhookService) secret(endpoint model.WebhookEndpoint) (string, error) {
	if len(s.cfg.EncryptionKey) != 32 {
		return "", ErrWebhookSecretsUnavailable
	}
	secret, err := appcrypto.DecryptStringWithAAD(endpoint.EncryptedSecret, s.cfg.EncryptionKey, webhookSecretAAD(endpoint.ID))
	if err != nil {
		return "", fmt.Errorf("decrypt webhook secret: %w", err)
	}
	return secret, nil
}

// ListEndpoints returns the workspace's endpoints.
func (s *WebhookService) ListEndpoints(ctx context.Context, workspaceID string) ([]model.WebhookEndpoint, error) {
	endpoints, err := s.repo.ListEndpoints(ctx, workspaceID)
	if endpoints == nil {
		endpoints = []model.WebhookEndpoint{}
	}
	return endpoints, err
}

// CreateEndpoint adds an endpoint and returns its signing secret once.
func (s *WebhookService) CreateEndpoint(ctx context.Context, workspaceID, actorID string, req model.WebhookEndpointRequest) (*model.WebhookEndpointSecret, error) {
	if count, err := s.repo.CountEndpoints(ctx, workspaceID); err != nil {
		return nil, err
	} else if count >= model.WebhookEndpointLimit {
		return nil, invalidWebhook("a workspace can have up to %d webhooks", model.WebhookEndpointLimit)
	}
	endpoint := &model.WebhookEndpoint{ID: uuid.NewString(), WorkspaceID: workspaceID, Enabled: req.Enabled == nil || *req.Enabled}
	if strings.TrimSpace(actorID) != "" {
		endpoint.CreatedByID = &actorID
	}
	if err := s.normalize(ctx, workspaceID, req, endpoint); err != nil {
		return nil, err
	}
	secret, err := s.newSecret(endpoint)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}
	return &model.WebhookEndpointSecret{Endpoint: endpoint, Secret: secret}, nil
}

// UpdateEndpoint replaces an endpoint's settings. Turning it back on
// clears the failure history that turned it off.
func (s *WebhookService) UpdateEndpoint(ctx context.Context, workspaceID, endpointID string, req model.WebhookEndpointRequest) (*model.WebhookEndpoint, error) {
	endpoint, err := s.repo.GetEndpoint(ctx, workspaceID, endpointID)
	if err != nil {
		return nil, err
	}
	if err := s.normalize(ctx, workspaceID, req, endpoint); err != nil {
		return nil, err
	}
	fields := map[string]any{"url": endpoint.URL, "description": endpoint.Description, "events": endpoint.Events, "mailbox_ids": endpoint.MailboxIDs}
	if req.Enabled != nil && *req.Enabled != endpoint.Enabled {
		endpoint.Enabled = *req.Enabled
		fields["enabled"] = endpoint.Enabled
		if endpoint.Enabled {
			endpoint.DisabledReason, endpoint.FailureCount = "", 0
			fields["disabled_reason"], fields["failure_count"] = "", 0
		}
	}
	if err := s.repo.UpdateEndpoint(ctx, workspaceID, endpointID, fields); err != nil {
		return nil, err
	}
	if !endpoint.Enabled {
		if err := s.repo.FailPendingDeliveries(ctx, endpointID, "The endpoint was turned off."); err != nil {
			return nil, err
		}
	}
	return s.repo.GetEndpoint(ctx, workspaceID, endpointID)
}

// RotateSecret replaces an endpoint's signing secret and returns it once.
func (s *WebhookService) RotateSecret(ctx context.Context, workspaceID, endpointID string) (*model.WebhookEndpointSecret, error) {
	endpoint, err := s.repo.GetEndpoint(ctx, workspaceID, endpointID)
	if err != nil {
		return nil, err
	}
	secret, err := s.newSecret(endpoint)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateEndpoint(ctx, workspaceID, endpointID, map[string]any{
		"encrypted_secret": endpoint.EncryptedSecret, "secret_hint": endpoint.SecretHint,
	}); err != nil {
		return nil, err
	}
	return &model.WebhookEndpointSecret{Endpoint: endpoint, Secret: secret}, nil
}

// DeleteEndpoint removes an endpoint and its delivery log.
func (s *WebhookService) DeleteEndpoint(ctx context.Context, workspaceID, endpointID string) error {
	return s.repo.DeleteEndpoint(ctx, workspaceID, endpointID)
}

// ListDeliveries returns an endpoint's recent deliveries.
func (s *WebhookService) ListDeliveries(ctx context.Context, workspaceID, endpointID string, limit int) ([]model.WebhookDelivery, error) {
	if _, err := s.repo.GetEndpoint(ctx, workspaceID, endpointID); err != nil {
		return nil, err
	}
	deliveries, err := s.repo.ListDeliveries(ctx, workspaceID, endpointID, limit)
	if deliveries == nil {
		deliveries = []model.WebhookDelivery{}
	}
	return deliveries, err
}

// SendTest sends a ping event right away and returns its delivery.
func (s *WebhookService) SendTest(ctx context.Context, workspaceID, endpointID string) (*model.WebhookDelivery, error) {
	endpoint, err := s.repo.GetEndpoint(ctx, workspaceID, endpointID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	eventID := uuid.NewString()
	payload, err := marshalWebhookEnvelope(model.WebhookEnvelope{
		ID: eventID, Type: model.WebhookEventPing, CreatedAt: now, WorkspaceID: workspaceID,
		Data: map[string]any{"message": "This is a test event from Helpin."},
	})
	if err != nil {
		return nil, err
	}
	delivery := &model.WebhookDelivery{
		ID: uuid.NewString(), WorkspaceID: workspaceID, EndpointID: endpointID, EventID: eventID,
		EventType: model.WebhookEventPing, Payload: payload, Status: model.WebhookDeliveryPending,
		NextAttemptAt: now.Add(webhookDeliveryLease), CreatedAt: now,
	}
	if err := s.repo.CreateDelivery(ctx, delivery); err != nil {
		return nil, err
	}
	// A test never retries: its result is what the user wants to see now.
	s.attempt(ctx, *endpoint, *delivery, false)
	return s.repo.GetDelivery(ctx, workspaceID, endpointID, delivery.ID)
}

// Redeliver queues another copy of a delivery with the same event ID, so
// receivers that deduplicate on Helpin-Event-Id process it once.
func (s *WebhookService) Redeliver(ctx context.Context, workspaceID, endpointID, deliveryID string) (*model.WebhookDelivery, error) {
	endpoint, err := s.repo.GetEndpoint(ctx, workspaceID, endpointID)
	if err != nil {
		return nil, err
	}
	if !endpoint.Enabled {
		return nil, invalidWebhook("turn the webhook on before resending events")
	}
	original, err := s.repo.GetDelivery(ctx, workspaceID, endpointID, deliveryID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	again := &model.WebhookDelivery{
		ID: uuid.NewString(), WorkspaceID: workspaceID, EndpointID: endpointID, EventID: original.EventID,
		EventType: original.EventType, Payload: original.Payload, Status: model.WebhookDeliveryPending,
		NextAttemptAt: now, CreatedAt: now,
	}
	if err := s.repo.CreateDelivery(ctx, again); err != nil {
		return nil, err
	}
	return again, nil
}
