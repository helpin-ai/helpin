package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Explicit emails are sent independently. This prevents a later reply's channel
// or CC/BCC selection from exposing an earlier reply to a different audience.
// Legacy replies retain their existing conversation batching and presence rules.
func (s *EmailFallbackService) fireEmailWithOptions(ctx context.Context, conversationID string, messageIDs []string, opts emailFallbackFireOptions) error {
	messages, err := s.messageRepo.GetByIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	legacy := make([]string, 0, len(messages))
	found := make(map[string]bool, len(messages))
	var firstErr error
	for _, msg := range messages {
		found[msg.ID] = true
		if msg.ConversationID != conversationID || msg.DeliveryMode() == model.SupportDeliveryChatOnly || explicitEmailBlocked(msg) || msg.EmailNotifiedAt != nil {
			if err := s.finishEmailBatch(ctx, conversationID, []string{msg.ID}, opts.cleanupRedis); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if msg.CancellableUntil != nil && s.now().Before(*msg.CancellableUntil) {
			continue
		}
		if !msg.ExplicitEmailDelivery() {
			legacy = append(legacy, msg.ID)
			continue
		}
		if opts.cleanupRedis {
			if err := s.renewEmailBatchClaim(ctx, conversationID); err != nil {
				return err
			}
		}
		explicitOpts := opts
		explicitOpts.explicitEmail = true
		if err := s.fireEmailBatch(ctx, conversationID, []string{msg.ID}, explicitOpts); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(legacy) > 0 {
		if err := s.fireEmailBatch(ctx, conversationID, legacy, opts); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	missing := make([]string, 0)
	for _, id := range messageIDs {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	if err := s.finishEmailBatch(ctx, conversationID, missing, opts.cleanupRedis); err != nil && firstErr == nil {
		firstErr = err
	}
	if firstErr == nil && opts.cleanupRedis {
		// Only release the processing reservation after the complete pass. Each
		// next pass also checks individual undo deadlines before sending.
		if err := s.postpone(ctx, conversationID, emailFallbackOnlineRetry); err != nil {
			return err
		}
	}
	return firstErr
}

// Remove only the processed IDs. Replies enqueued during a send, online legacy
// replies, and explicit replies still inside their undo window remain queued.
func (s *EmailFallbackService) finishEmailBatch(ctx context.Context, conversationID string, ids []string, cleanupRedis bool) error {
	if !cleanupRedis || s.redis == nil {
		return nil
	}
	args := []any{conversationID, s.now().Add(emailFallbackOnlineRetry).Unix()}
	for _, id := range ids {
		args = append(args, id)
	}
	const script = `
for i = 3, #ARGV do redis.call('LREM', KEYS[1], 0, ARGV[i]) end
if redis.call('LLEN', KEYS[1]) == 0 then
  redis.call('DEL', KEYS[1])
  redis.call('ZREM', KEYS[2], ARGV[1])
else
  redis.call('ZADD', KEYS[2], 'NX', ARGV[2], ARGV[1])
end
return 1`
	return s.redis.Eval(ctx, script, []string{s.msgListKey(conversationID), emailFallbackOutboxKey}, args...).Err()
}

type supportExplicitDeliveryMetadata struct {
	Status  string `json:"email_delivery_status"`
	Error   string `json:"email_delivery_error"`
	To      string `json:"delivery_to_email"`
	Subject string `json:"email_subject"`
}

func explicitDeliveryMetadata(msg model.SupportMessage) supportExplicitDeliveryMetadata {
	var meta supportExplicitDeliveryMetadata
	if err := json.Unmarshal([]byte(msg.Metadata), &meta); err != nil {
		return supportExplicitDeliveryMetadata{}
	}
	return meta
}

func explicitEmailBlocked(msg model.SupportMessage) bool {
	return msg.ExplicitEmailDelivery() && explicitDeliveryMetadata(msg).Status == "blocked"
}

func explicitRecipientChanged(msg model.SupportMessage, conv *model.SupportConversation) string {
	to := explicitDeliveryMetadata(msg).To
	if to != "" && !strings.EqualFold(strings.TrimSpace(to), strings.TrimSpace(derefString(conv.CustomerEmail))) {
		return "The primary recipient changed after this reply was queued. Send a new reply to the intended recipient."
	}
	return ""
}

func (s *EmailFallbackService) setExplicitEmailStatus(ctx context.Context, conv *model.SupportConversation, ids []string, status, reason string) error {
	if err := s.messageRepo.UpdateExplicitEmailStatus(ctx, ids, status, reason); err != nil {
		return err
	}
	for _, id := range ids {
		s.publishMessageUpdated(conv.WorkspaceID, conv.ID, id, "email:"+status)
	}
	return nil
}

func (s *EmailFallbackService) blockExplicitEmail(ctx context.Context, conv *model.SupportConversation, ids []string, reason string, cleanupRedis bool) error {
	if err := s.setExplicitEmailStatus(ctx, conv, ids, "blocked", reason); err != nil {
		return err
	}
	return s.finishEmailBatch(ctx, conv.ID, ids, cleanupRedis)
}

func (s *EmailFallbackService) renewEmailBatchClaim(ctx context.Context, conversationID string) error {
	if s.redis == nil {
		return nil
	}
	const script = `if redis.call('ZSCORE', KEYS[1], ARGV[1]) then redis.call('ZADD', KEYS[1], 'GT', ARGV[2], ARGV[1]) end; return 1`
	return s.redis.Eval(ctx, script, []string{emailFallbackOutboxKey}, conversationID, s.now().Add(s.processingTTL).Unix()).Err()
}
