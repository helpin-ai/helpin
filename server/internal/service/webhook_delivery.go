package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/netguard"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// webhookDeliveryLease hides a claimed delivery from other workers.
const webhookDeliveryLease = 2 * time.Minute

// webhookRetryDelays follow a failed attempt; together they span about a day.
var webhookRetryDelays = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 15 * time.Hour}

// webhookDisableAfterFailures consecutive failed attempts, with no success
// for a day, turn an endpoint off.
const webhookDisableAfterFailures = 20

// webhookDeliveryRetention keeps the delivery log for 30 days.
const webhookDeliveryRetention = 30 * 24 * time.Hour

func marshalWebhookEnvelope(envelope model.WebhookEnvelope) (json.RawMessage, error) {
	raw, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode webhook payload: %w", err)
	}
	return raw, nil
}

// WebhookSignature signs a payload as Helpin-Signature: t=<unix>,v1=<hex>,
// where v1 is HMAC-SHA256 of "<unix>.<body>" with the endpoint secret.
func WebhookSignature(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

func webhookSubscribed(endpoint model.WebhookEndpoint, eventType string) bool {
	return endpoint.Enabled && (len(endpoint.Events) == 0 || slices.Contains([]string(endpoint.Events), eventType))
}

func webhookMailboxMatches(endpoint model.WebhookEndpoint, conversation *model.SupportConversation) bool {
	if len(endpoint.MailboxIDs) == 0 {
		return true
	}
	mailbox := "shared"
	if conversation != nil && conversation.MailboxID != nil && *conversation.MailboxID != "" {
		mailbox = *conversation.MailboxID
	}
	return slices.Contains([]string(endpoint.MailboxIDs), mailbox)
}

// Emit records an event the server raises itself, such as a missed SLA or a
// rating. It never fails the caller.
func (s *WebhookService) Emit(ctx context.Context, workspaceID, eventType, conversationID string, data map[string]any) {
	if s == nil {
		return
	}
	endpoints, err := s.repo.EnabledEndpoints(ctx, workspaceID)
	if err != nil {
		slog.ErrorContext(ctx, "list webhook endpoints", "workspace_id", workspaceID, "error", err)
		return
	}
	if !slices.ContainsFunc(endpoints, func(endpoint model.WebhookEndpoint) bool { return webhookSubscribed(endpoint, eventType) }) {
		return
	}
	raw, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "encode webhook event", "event_type", eventType, "error", err)
		return
	}
	now := s.now()
	event := &model.WebhookEvent{ID: uuid.NewString(), WorkspaceID: workspaceID, EventType: eventType, Data: raw, AvailableAt: now, CreatedAt: now}
	if conversationID != "" {
		event.ConversationID = &conversationID
	}
	if err := s.repo.InsertEvent(ctx, event); err != nil {
		slog.ErrorContext(ctx, "record webhook event", "workspace_id", workspaceID, "event_type", eventType, "error", err)
	}
}

// webhookBatch caches lookups across one dispatch pass.
type webhookBatch struct {
	endpoints map[string][]model.WebhookEndpoint
	slugs     map[string]string
}

// ProcessEvents turns available outbox events into deliveries.
func (s *WebhookService) ProcessEvents(ctx context.Context, limit int) (int, error) {
	batch := &webhookBatch{endpoints: map[string][]model.WebhookEndpoint{}, slugs: map[string]string{}}
	now := s.now()
	return s.repo.DispatchEvents(ctx, now, limit, func(events []model.WebhookEvent) ([]model.WebhookDelivery, error) {
		var deliveries []model.WebhookDelivery
		for _, event := range events {
			endpoints, ok := batch.endpoints[event.WorkspaceID]
			if !ok {
				var err error
				if endpoints, err = s.repo.EnabledEndpoints(ctx, event.WorkspaceID); err != nil {
					return nil, err
				}
				batch.endpoints[event.WorkspaceID] = endpoints
			}
			subscribed := slices.DeleteFunc(slices.Clone(endpoints), func(endpoint model.WebhookEndpoint) bool {
				return !webhookSubscribed(endpoint, event.EventType)
			})
			if len(subscribed) == 0 {
				continue
			}
			envelope, conversation, err := s.buildEnvelope(ctx, batch, event)
			if err != nil {
				return nil, err
			}
			if envelope == nil {
				continue // The conversation or message is gone.
			}
			payload, err := marshalWebhookEnvelope(*envelope)
			if err != nil {
				return nil, err
			}
			for _, endpoint := range subscribed {
				if !webhookMailboxMatches(endpoint, conversation) {
					continue
				}
				deliveries = append(deliveries, model.WebhookDelivery{
					ID: uuid.NewString(), WorkspaceID: event.WorkspaceID, EndpointID: endpoint.ID, EventID: event.ID,
					EventType: event.EventType, Payload: payload, Status: model.WebhookDeliveryPending,
					NextAttemptAt: now, CreatedAt: now,
				})
			}
		}
		return deliveries, nil
	})
}

// buildEnvelope loads the current conversation (and message) for an event.
// It returns nil when they no longer exist or were anonymized.
func (s *WebhookService) buildEnvelope(ctx context.Context, batch *webhookBatch, event model.WebhookEvent) (*model.WebhookEnvelope, *model.SupportConversation, error) {
	data := map[string]any{}
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &data); err != nil {
			slog.WarnContext(ctx, "skip webhook event with invalid data", "event_id", event.ID, "error", err)
			return nil, nil, nil
		}
	}
	envelope := &model.WebhookEnvelope{ID: event.ID, Type: event.EventType, CreatedAt: event.CreatedAt.UTC(), WorkspaceID: event.WorkspaceID, Data: data}
	if event.ConversationID == nil {
		return envelope, nil, nil
	}
	conversation, err := s.inbox.conversationRepo.GetByID(ctx, event.WorkspaceID, *event.ConversationID, "", model.RoleOwner)
	if err != nil {
		return nil, nil, err
	}
	if conversation == nil || conversation.AnonymizedAt != nil {
		return nil, nil, nil
	}
	snapshot, err := s.conversationSnapshot(ctx, batch, conversation)
	if err != nil {
		return nil, nil, err
	}
	data["conversation"] = snapshot
	if event.MessageID != nil {
		message, err := s.inbox.messageRepo.GetByID(ctx, *event.MessageID)
		if err != nil {
			return nil, nil, err
		}
		if message == nil || message.ConversationID != conversation.ID {
			return nil, nil, nil // Deleted inside its undo window.
		}
		data["message"] = model.WebhookMessage{
			ID: message.ID, SenderType: message.SenderType, SenderID: message.SenderUserID,
			SenderName: message.SenderDisplayName, Content: message.Content, CreatedAt: message.CreatedAt.UTC(),
		}
	}
	return envelope, conversation, nil
}

func (s *WebhookService) conversationSnapshot(ctx context.Context, batch *webhookBatch, conversation *model.SupportConversation) (model.WebhookConversation, error) {
	snapshot := model.WebhookConversation{
		ID: conversation.ID, Number: conversation.DisplayID, Subject: conversation.Subject, Status: conversation.Status,
		Priority: conversation.Priority, Channel: conversation.Channel, InboxID: conversation.MailboxID,
		AssigneeID: conversation.AssignedUserID, SnoozedUntil: conversation.SnoozedUntil, MergedIntoID: conversation.MergedIntoID,
		Customer:  model.WebhookCustomer{Name: conversation.CustomerName, Email: conversation.CustomerEmail, CRMContactID: conversation.CRMContactID},
		CreatedAt: conversation.CreatedAt.UTC(), UpdatedAt: conversation.UpdatedAt.UTC(), ResolvedAt: conversation.ResolvedAt,
		Tags: []string{},
	}
	if s.inbox.tagRepo != nil {
		tags, err := s.inbox.tagRepo.ListByConversationIDs(ctx, conversation.WorkspaceID, []string{conversation.ID})
		if err != nil {
			return snapshot, err
		}
		for _, tag := range tags[conversation.ID] {
			snapshot.Tags = append(snapshot.Tags, tag.Name)
		}
	}
	if s.sla != nil {
		hydrated := []model.SupportConversation{{ID: conversation.ID}}
		if err := s.sla.HydrateConversations(ctx, conversation.WorkspaceID, hydrated); err != nil {
			return snapshot, err
		}
		snapshot.SLA = hydrated[0].SLA
	}
	if base := strings.TrimRight(s.cfg.AppBaseURL, "/"); base != "" && s.inbox.workspaceRepo != nil {
		slug, ok := batch.slugs[conversation.WorkspaceID]
		if !ok {
			if workspace, err := s.inbox.workspaceRepo.GetByID(ctx, conversation.WorkspaceID); err == nil && workspace != nil {
				slug = workspace.Slug
			}
			batch.slugs[conversation.WorkspaceID] = slug
		}
		if slug != "" {
			snapshot.URL = base + "/w/" + slug + "/support/" + conversation.ID
		}
	}
	return snapshot, nil
}

// ProcessDeliveries sends due deliveries and returns how many it attempted.
func (s *WebhookService) ProcessDeliveries(ctx context.Context, limit int) (int, error) {
	now := s.now()
	claimed, err := s.repo.ClaimDeliveries(ctx, now, now.Add(webhookDeliveryLease), limit)
	if err != nil || len(claimed) == 0 {
		return 0, err
	}
	endpoints := map[string]*model.WebhookEndpoint{}
	for _, delivery := range claimed {
		if _, ok := endpoints[delivery.EndpointID]; ok {
			continue
		}
		endpoint, err := s.repo.GetEndpoint(ctx, delivery.WorkspaceID, delivery.EndpointID)
		if err != nil && !errors.Is(err, repository.ErrWebhookEndpointNotFound) {
			return 0, err
		}
		endpoints[delivery.EndpointID] = endpoint
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(max(1, s.concurrency))
	for _, delivery := range claimed {
		endpoint := endpoints[delivery.EndpointID]
		if endpoint == nil || !endpoint.Enabled {
			if err := s.repo.FailPendingDeliveries(ctx, delivery.EndpointID, "The endpoint was turned off."); err != nil {
				slog.ErrorContext(ctx, "fail webhook deliveries", "endpoint_id", delivery.EndpointID, "error", err)
			}
			continue
		}
		group.Go(func() (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					slog.ErrorContext(groupCtx, "webhook delivery panicked", "delivery_id", delivery.ID, "panic", recovered)
				}
			}()
			s.attempt(groupCtx, *endpoint, delivery, true)
			return nil
		})
	}
	return len(claimed), group.Wait()
}

// attempt sends one delivery and records the outcome. A send interrupted by
// shutdown is not recorded; its lease returns it to the queue.
func (s *WebhookService) attempt(ctx context.Context, endpoint model.WebhookEndpoint, delivery model.WebhookDelivery, retry bool) {
	start := s.now()
	statusCode, sendErr := s.send(ctx, endpoint, delivery)
	if sendErr != nil && ctx.Err() != nil {
		return
	}
	outcome := repository.WebhookAttempt{At: start, DurationMS: int(time.Since(start) / time.Millisecond), StatusCode: statusCode}
	outcome.Succeeded = sendErr == nil
	if sendErr != nil {
		outcome.Error = truncateWebhookText(sendErr.Error(), 500)
		attempts := delivery.Attempts + 1
		gone := statusCode != nil && *statusCode == http.StatusGone
		if retry && !gone && attempts <= len(webhookRetryDelays) {
			next := start.Add(webhookRetryDelays[attempts-1])
			outcome.NextAttemptAt = &next
		}
	}
	recordCtx := context.WithoutCancel(ctx)
	failures, err := s.repo.RecordAttempt(recordCtx, delivery, outcome)
	if err != nil {
		slog.ErrorContext(ctx, "record webhook attempt", "delivery_id", delivery.ID, "error", err)
		return
	}
	if outcome.Succeeded {
		return
	}
	reason := ""
	switch {
	case statusCode != nil && *statusCode == http.StatusGone:
		reason = "Turned off because the endpoint answered 410 Gone."
	case failures >= webhookDisableAfterFailures && (endpoint.LastSuccessAt == nil || start.Sub(*endpoint.LastSuccessAt) > 24*time.Hour):
		reason = fmt.Sprintf("Turned off after %d failed deliveries in a row.", failures)
	}
	if reason != "" {
		if err := s.repo.DisableEndpoint(recordCtx, endpoint.ID, reason, start); err != nil {
			slog.ErrorContext(ctx, "disable webhook endpoint", "endpoint_id", endpoint.ID, "error", err)
		} else {
			slog.WarnContext(ctx, "webhook endpoint turned off", "workspace_id", endpoint.WorkspaceID, "endpoint_id", endpoint.ID, "reason", reason)
		}
	}
}

func (s *WebhookService) send(ctx context.Context, endpoint model.WebhookEndpoint, delivery model.WebhookDelivery) (*int, error) {
	secret, err := s.secret(endpoint)
	if err != nil {
		return nil, errors.New("the signing secret could not be read")
	}
	address, err := ValidateWebhookURL(endpoint.URL, s.cfg.AllowPrivateNetworks)
	if err != nil {
		return nil, err
	}
	body := []byte(delivery.Payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address, strings.NewReader(string(body)))
	if err != nil {
		return nil, errors.New("the request could not be built")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Helpin-Webhooks/1.0")
	request.Header.Set("Helpin-Event-Id", delivery.EventID)
	request.Header.Set("Helpin-Event-Type", delivery.EventType)
	request.Header.Set("Helpin-Delivery-Id", delivery.ID)
	request.Header.Set("Helpin-Signature", WebhookSignature(secret, s.now().Unix(), body))
	response, err := s.client.Do(request)
	if err != nil {
		return nil, webhookTransportError(err)
	}
	defer response.Body.Close()
	excerpt, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	status := response.StatusCode
	if status >= 200 && status < 300 {
		return &status, nil
	}
	message := fmt.Sprintf("The endpoint answered HTTP %d.", status)
	if text := strings.TrimSpace(string(excerpt)); text != "" {
		message += " " + truncateWebhookText(text, 200)
	}
	return &status, errors.New(message)
}

// webhookTransportError explains a failed connection without internals.
func webhookTransportError(err error) error {
	switch {
	case errors.Is(err, netguard.ErrNonPublicAddress):
		return errors.New("the host resolves to a private network address")
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout"):
		return errors.New("the endpoint did not answer within 15 seconds")
	case strings.Contains(err.Error(), "could not be resolved"):
		return errors.New("the host name could not be resolved")
	case strings.Contains(err.Error(), "certificate") || strings.Contains(err.Error(), "tls:"):
		return errors.New("the TLS certificate was not accepted")
	default:
		return errors.New("the endpoint could not be reached")
	}
}

// Run dispatches events and sends deliveries until ctx is cancelled.
func (s *WebhookService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastPurge := time.Time{}
	for {
		for i := 0; i < 20 && ctx.Err() == nil; i++ {
			processed, err := s.ProcessEvents(ctx, 200)
			if err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "dispatch webhook events", "error", err)
			}
			if err != nil || processed < 200 {
				break
			}
		}
		for i := 0; i < 20 && ctx.Err() == nil; i++ {
			sent, err := s.ProcessDeliveries(ctx, 50)
			if err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "send webhook deliveries", "error", err)
			}
			if err != nil || sent < 50 {
				break
			}
		}
		if now := s.now(); now.Sub(lastPurge) > time.Hour {
			lastPurge = now
			if err := s.repo.PurgeDeliveries(ctx, now.Add(-webhookDeliveryRetention)); err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "purge webhook deliveries", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func truncateWebhookText(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
