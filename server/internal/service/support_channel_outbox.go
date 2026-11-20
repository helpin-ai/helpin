package service

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

const (
	_outboxLease       = 2 * time.Minute
	_outboxBatch       = 25
	_outboxMaxAttempts = 8
	_outboxMaxBackoff  = time.Hour
	// Two rejected credentials in a row mark the bot as needing a new token;
	// one could be a transient Telegram error.
	_channelAuthErrorThreshold = 2
)

// SetOutboxRepository enables reply delivery.
func (s *SupportChannelService) SetOutboxRepository(outbox *repository.SupportChannelOutboxRepository) *SupportChannelService {
	s.outbox = outbox
	return s
}

// RunOutbox delivers queued replies until ctx ends.
func (s *SupportChannelService) RunOutbox(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.ProcessOutbox(ctx); err != nil && ctx.Err() == nil {
				s.logger.ErrorContext(ctx, "channel outbox failed", "error", err)
			}
		}
	}
}

// ProcessOutbox sends one batch of due replies and reports how many it
// handled.
func (s *SupportChannelService) ProcessOutbox(ctx context.Context) (int, error) {
	if s.outbox == nil {
		return 0, nil
	}
	rows, err := s.outbox.ClaimDue(ctx, s.now(), _outboxLease, _outboxBatch)
	if err != nil {
		return 0, err
	}
	for i := range rows {
		if ctx.Err() != nil {
			// The lease expires and another worker picks the rest up.
			return i, ctx.Err()
		}
		s.deliver(ctx, &rows[i])
	}
	return len(rows), nil
}

// deliver sends one reply, part by part, resuming after parts already sent.
func (s *SupportChannelService) deliver(ctx context.Context, row *model.SupportChannelOutbox) {
	outcome := s.dispatch(ctx, row)
	if err := s.settle(ctx, row, outcome); err != nil {
		s.logger.ErrorContext(ctx, "record channel delivery", "error", err, "outbox_id", row.ID)
		return
	}
	if s.inbox != nil {
		s.inbox.wsPublisher.Publish(websocket.Event{
			Action: "updated", Entity: "support_conversation_message", EntityID: row.MessageID, WorkspaceID: row.WorkspaceID,
			ActorID: "channel-delivery", ParentType: "support_conversation", ParentID: row.ConversationID,
		})
	}
}

// dispatchOutcome is what one delivery attempt established.
type dispatchOutcome struct {
	status string // accepted, retry, rejected, uncertain, cancelled
	code   string
	detail string
	retry  time.Duration
}

func (s *SupportChannelService) dispatch(ctx context.Context, row *model.SupportChannelOutbox) dispatchOutcome {
	account, err := s.accounts.Get(ctx, row.WorkspaceID, row.ChannelAccountID)
	if err != nil {
		return dispatchOutcome{status: "cancelled", code: "disconnected", detail: "the bot is no longer connected"}
	}
	// Everything is checked again right before sending: the account, message
	// and conversation may have changed while the reply waited.
	switch {
	case account.DisabledAt != nil:
		return dispatchOutcome{status: "cancelled", code: "disconnected", detail: "the bot was disconnected"}
	case account.Status == model.SupportChannelAccountReauthRequired:
		return dispatchOutcome{status: "rejected", code: channels.CodeAuth, detail: "The channel rejected its credentials"}
	}
	msg, err := s.inbox.messageRepo.GetByID(ctx, row.MessageID)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "database", detail: err.Error()}
	}
	if msg == nil || msg.DeletedAt.Valid {
		return dispatchOutcome{status: "cancelled", code: "deleted", detail: "the reply was deleted"}
	}
	conv, err := s.inbox.conversationRepo.GetByID(ctx, row.WorkspaceID, row.ConversationID, "", model.RoleOwner)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "database", detail: err.Error()}
	}
	if conv == nil || conv.AnonymizedAt != nil {
		return dispatchOutcome{status: "cancelled", code: "deleted", detail: "the conversation was deleted"}
	}
	var identity model.SupportChannelIdentity
	if err := s.inbox.conversationRepo.DB().WithContext(ctx).Where("id = ?", row.ChannelIdentityID).Limit(1).Find(&identity).Error; err != nil {
		return dispatchOutcome{status: "retry", code: "database", detail: err.Error()}
	}
	if identity.ID == "" {
		return dispatchOutcome{status: "cancelled", code: "deleted", detail: "the customer's details were deleted"}
	}
	if row.ProviderOriginID != nil && *row.ProviderOriginID != conv.ID {
		merges := repository.NewSupportMergeRepository(s.inbox.conversationRepo.DB())
		root, err := merges.Root(ctx, row.WorkspaceID, *row.ProviderOriginID)
		if err != nil {
			return dispatchOutcome{status: "retry", code: "database"}
		}
		if root != conv.ID {
			return dispatchOutcome{status: "cancelled", code: "audience_changed"}
		}
		conv, err = s.inbox.conversationRepo.GetByID(ctx, row.WorkspaceID, *row.ProviderOriginID, "", model.RoleOwner)
		if err != nil {
			return dispatchOutcome{status: "retry", code: "database"}
		}
		if conv == nil || conv.AnonymizedAt != nil {
			return dispatchOutcome{status: "cancelled", code: "deleted"}
		}
	}
	if account.Provider == "slack" && s.slack != nil {
		return s.slack.dispatchSlack(ctx, account, conv, msg, row)
	}
	adapter, ok := s.adapters.Lookup(account.Provider)
	if !ok {
		return dispatchOutcome{status: "rejected", code: "unsupported", detail: "unsupported provider"}
	}
	token, err := s.decryptToken(account)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "credentials", detail: err.Error()}
	}
	files, err := s.replyFiles(ctx, msg, adapter.Capabilities().MaxUploadSize)
	if err != nil {
		return dispatchOutcome{status: "retry", code: "attachments", detail: err.Error()}
	}
	parts := adapter.Render(channels.Outgoing{Text: msg.Content, Files: files})
	if len(parts) == 0 {
		return dispatchOutcome{status: "cancelled", code: "empty", detail: "nothing to send"}
	}
	for i := row.PartsSent; i < len(parts); i++ {
		result := adapter.SendPart(ctx, token, identity.ExternalUserID, parts[i], false)
		if result.Outcome == channels.Rejected && result.Code == channels.CodeParse {
			// Telegram refused the formatting; the same text without it is safe.
			result = adapter.SendPart(ctx, token, identity.ExternalUserID, parts[i], true)
		}
		switch result.Outcome {
		case channels.Accepted:
			if err := s.outbox.RecordPart(ctx, row, i+1, result.ExternalKey); err != nil {
				// The part is out; stopping here could only resend it.
				s.logger.ErrorContext(ctx, "record sent part", "error", err, "outbox_id", row.ID)
			}
			row.PartsSent = i + 1
		case channels.NotSent:
			return dispatchOutcome{status: "retry", code: result.Code, detail: result.Message, retry: result.RetryAfter}
		case channels.Rejected:
			if result.Code == channels.CodeAuth {
				if err := s.accounts.RecordAuthError(ctx, account.ID, _channelAuthErrorThreshold, result.Message); err != nil {
					s.logger.ErrorContext(ctx, "record channel auth error", "error", err, "account_id", account.ID)
				}
			}
			return dispatchOutcome{status: "rejected", code: result.Code, detail: result.Message}
		default:
			return dispatchOutcome{status: "uncertain", code: result.Code, detail: result.Message}
		}
	}
	return dispatchOutcome{status: "accepted"}
}

// settle records an attempt's outcome on the row.
func (s *SupportChannelService) settle(ctx context.Context, row *model.SupportChannelOutbox, outcome dispatchOutcome) error {
	switch outcome.status {
	case "accepted":
		return s.outbox.MarkAccepted(ctx, row)
	case "retry":
		if row.Attempts >= _outboxMaxAttempts {
			return s.outbox.Finish(ctx, row, "rejected", "retries_exhausted", outcome.detail)
		}
		return s.outbox.ScheduleRetry(ctx, row, s.now().Add(outboxBackoff(row.Attempts, outcome.retry)), outcome.code, outcome.detail)
	default:
		return s.outbox.Finish(ctx, row, outcome.status, outcome.code, outcome.detail)
	}
}

// outboxBackoff doubles from one second with up to 25% jitter, capped at an
// hour; the provider's own retry-after wins when it is longer.
func outboxBackoff(attempts int, retryAfter time.Duration) time.Duration {
	shift := attempts - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 12 {
		shift = 12
	}
	delay := time.Second << shift
	delay += time.Duration(rand.Int64N(int64(delay)/4 + 1))
	if retryAfter > delay {
		delay = retryAfter
	}
	if delay > _outboxMaxBackoff {
		delay = _outboxMaxBackoff
	}
	return delay
}

// replyFiles loads the reply's attachments for upload.
func (s *SupportChannelService) replyFiles(ctx context.Context, msg *model.SupportMessage, maxSize int64) ([]channels.OutgoingFile, error) {
	if s.inbox.attachmentService == nil {
		return nil, nil
	}
	messages := []model.SupportMessage{*msg}
	if err := s.inbox.attachmentService.HydrateMessages(ctx, messages); err != nil {
		return nil, err
	}
	var files []channels.OutgoingFile
	for _, attachment := range messages[0].Attachments {
		if maxSize > 0 && attachment.FileSize > maxSize {
			return nil, errors.New("an attachment is larger than Telegram accepts")
		}
		data, err := s.inbox.attachmentService.DownloadContent(ctx, attachment)
		if err != nil {
			return nil, err
		}
		files = append(files, channels.OutgoingFile{Name: attachment.FileName, ContentType: attachment.FileType, Data: data})
	}
	return files, nil
}

// RetryChannelDelivery queues a reply that was not delivered, or whose
// delivery was not confirmed, again.
func (s *SupportInboxService) RetryChannelDelivery(ctx context.Context, workspaceID, messageID string) error {
	if s.channelOutbox == nil {
		return ErrChannelNothingToRetry
	}
	conversationID, err := s.channelOutbox.Retry(ctx, workspaceID, messageID)
	if err != nil {
		return err
	}
	if conversationID == "" {
		return ErrChannelNothingToRetry
	}
	s.wsPublisher.Publish(websocket.Event{
		Action: "updated", Entity: "support_conversation_message", EntityID: messageID, WorkspaceID: workspaceID,
		ActorID: "channel-delivery", ParentType: "support_conversation", ParentID: conversationID,
	})
	return nil
}

// ErrChannelNothingToRetry means the reply has no failed or unconfirmed
// delivery.
var ErrChannelNothingToRetry = errors.New("this reply has no delivery to retry")

// channelDeliveryView maps an outbox row to the bubble's delivery state.
func channelDeliveryView(row model.SupportChannelOutbox) *model.SupportChannelDelivery {
	view := &model.SupportChannelDelivery{Provider: row.Provider, ErrorCode: derefString(row.LastErrorCode)}
	switch row.DispatchStatus {
	case "pending", "dispatching":
		view.Status = "sending"
	case "accepted":
		view.Status = "sent"
	case "rejected":
		view.Status, view.Retryable = "failed", true
	case "uncertain":
		view.Status, view.Retryable = "unconfirmed", true
	default:
		view.Status = "cancelled"
	}
	return view
}

// hydrateChannelDelivery attaches provider delivery state to replies.
func (s *SupportInboxService) hydrateChannelDelivery(ctx context.Context, workspaceID string, conv *model.SupportConversation, messages []model.SupportMessage) {
	if s.channelOutbox == nil || len(messages) == 0 || (conv.ChannelIdentityID == nil && conv.MergeRevision == 0) {
		return
	}
	ids := make([]string, 0, len(messages))
	for _, m := range messages {
		if m.SenderType != "customer" {
			ids = append(ids, m.ID)
		}
	}
	rows, err := s.channelOutbox.ListByMessageIDs(ctx, workspaceID, ids)
	if err != nil {
		slog.ErrorContext(ctx, "hydrate channel delivery", "error", err, "conversation_id", conv.ID)
		return
	}
	byMessage := make(map[string]model.SupportChannelOutbox, len(rows))
	for _, row := range rows {
		byMessage[row.MessageID] = row
	}
	for i := range messages {
		if row, ok := byMessage[messages[i].ID]; ok {
			messages[i].ChannelDelivery = channelDeliveryView(row)
		}
	}
}
