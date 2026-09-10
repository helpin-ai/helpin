package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

// advanceWaiting preserves legacy single-reminder episodes while new episodes
// send a separate final notice before starting their closure clock.
func (s *SupportFollowUpService) advanceWaiting(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, e *model.SupportAIFollowUp, settings model.SupportInboxSettings, now time.Time) (*model.SupportMessage, error) {
	if e.SequenceVersion < 2 {
		return nil, s.closeIfDue(ctx, tx, conv, e, now)
	}
	if e.SentMessageID == nil || e.SentAt == nil || now.Before(e.DueAt) {
		return nil, nil
	}
	busy, err := repository.SupportConversationBusy(ctx, tx, conv.WorkspaceID, conv.ID, "")
	if err != nil || busy {
		return nil, err
	}
	messageID, sentAt, waitHours := *e.SentMessageID, *e.SentAt, e.SecondDelayHours
	if e.SecondMessageID != nil && e.SecondSentAt != nil {
		messageID, sentAt, waitHours = *e.SecondMessageID, *e.SecondSentAt, e.CloseHours
	}
	acceptedAt, failure, err := supportFollowUpDeliveryTime(ctx, tx, conv, messageID, sentAt)
	if err != nil {
		return nil, err
	}
	if failure != "" {
		return nil, s.failAndHandoff(ctx, tx, conv, e, settings, failure, now)
	}
	deadline := acceptedAt.Add(time.Duration(waitHours) * time.Hour)
	if now.Before(deadline) {
		fields := map[string]any{"due_at": deadline, "lease_until": nil, "updated_at": now}
		if e.SecondMessageID != nil {
			fields["close_at"] = deadline
		}
		return nil, tx.Model(e).Updates(fields).Error
	}
	if e.SecondMessageID != nil {
		if err := finishFollowUpRow(tx, e, "resolved", "no_reply_after_follow_up", now); err != nil {
			return nil, err
		}
		return nil, resolveSupportAIConversation(ctx, tx, conv, "assumed", now)
	}
	if e.ClosingNotice == "" {
		return nil, s.failAndHandoff(ctx, tx, conv, e, settings, "closing_notice_missing", now)
	}
	id := uuid.NewString()
	closeAt := now.Add(time.Duration(e.CloseHours) * time.Hour)
	// Write our second ID before the projection trigger observes the new message.
	if err := tx.Model(e).Updates(map[string]any{"second_message_id": id, "second_sent_at": now, "close_at": closeAt, "due_at": closeAt, "lease_until": nil, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	agentID := derefString(conv.AssignedAgentID)
	metadata, _ := json.Marshal(map[string]any{"ai_auto_reply": true, "ai_agent_id": agentID, "ai_model": "agent-runtime", "ai_reply_kind": "inactivity_follow_up", "support_follow_up_id": e.ID, "follow_up_number": 2})
	msg := &model.SupportMessage{ID: id, WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "ai", SenderAgentID: &agentID, SenderDisplayName: strPtr(helpinAIDisplayName), Content: e.ClosingNotice, MessageType: "reply", Metadata: string(metadata), CreatedAt: now}
	if err := s.chat.messageRepo.WithTx(tx).Create(ctx, msg); err != nil {
		return nil, err
	}
	if err := repository.NewSupportConversationRepository(tx).UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{"ai_turn_count": gorm.Expr("ai_turn_count + 1")}); err != nil {
		return nil, err
	}
	return msg, nil
}

func supportFollowUpDeliveryTime(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, messageID string, sentAt time.Time) (time.Time, string, error) {
	var logs []model.SupportEmailLog
	if tx.Dialector.Name() == "postgres" {
		if err := tx.WithContext(ctx).Where("workspace_id = ? AND conversation_id = ? AND direction = 'outbound' AND ? = ANY(message_ids)", conv.WorkspaceID, conv.ID, messageID).Find(&logs).Error; err != nil {
			return sentAt, "", err
		}
	}
	accepted := conv.Channel == "widget"
	for _, log := range logs {
		if log.Status == "failed" || log.Status == "bounced" || log.Status == "spam_complaint" || log.BouncedAt != nil {
			return sentAt, "follow_up_delivery_failed", nil
		}
		if log.Status == "sent" || log.Status == "delivered" || log.Status == "opened" {
			accepted = true
			if log.CreatedAt.After(sentAt) {
				sentAt = log.CreatedAt
			}
			if log.DeliveredAt != nil && log.DeliveredAt.After(sentAt) {
				sentAt = *log.DeliveredAt
			}
		}
	}
	if !accepted {
		return sentAt, "follow_up_delivery_unconfirmed", nil
	}
	return sentAt, "", nil
}

// Failed assessments retry with a new run ID; stale callbacks cannot send.
func (s *SupportFollowUpService) retryOrHandoff(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, e *model.SupportAIFollowUp, settings model.SupportInboxSettings, reason string, now time.Time) error {
	if e.AssessmentAttempts >= 3 {
		return s.failAndHandoff(ctx, tx, conv, e, settings, reason, now)
	}
	return tx.Model(e).Updates(map[string]any{"run_id": uuid.NewString(), "assessment_attempts": e.AssessmentAttempts + 1, "status": "scheduled", "started_at": nil, "due_at": now.Add(5 * time.Minute), "lease_until": nil, "reason": reason, "updated_at": now}).Error
}
func (s *SupportFollowUpService) failAndHandoff(ctx context.Context, tx *gorm.DB, conv *model.SupportConversation, e *model.SupportAIFollowUp, settings model.SupportInboxSettings, reason string, now time.Time) error {
	if err := finishFollowUpRow(tx, e, "failed", reason, now); err != nil {
		return err
	}
	return s.handoff(ctx, tx, conv, settings, now)
}
