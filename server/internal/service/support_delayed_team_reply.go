package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StartDelayedTeamReplyWorker reconciles persisted handoffs, including after restarts.
func (s *SupportInboxService) StartDelayedTeamReplyWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.ProcessDelayedTeamReplies(ctx, time.Now()); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "delayed team reply sweep failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessDelayedTeamReplies uses bounded pages. Assignment, notes and customer
// messages never affect the escalation timestamp or the delivery marker.
func (s *SupportInboxService) ProcessDelayedTeamReplies(ctx context.Context, now time.Time) error {
	db := s.conversationRepo.DB()
	cursor := "00000000-0000-0000-0000-000000000000"
	for {
		var conversations []model.SupportConversation
		err := db.WithContext(ctx).Where("id > ? AND ai_state = ? AND status = ? AND channel = ? AND ai_escalated_at <= ? AND (delayed_team_reply_sent_for IS NULL OR delayed_team_reply_sent_for < ai_escalated_at)", cursor, "escalated", "open", "widget", now.Add(-time.Minute)).Order("id").Limit(100).Find(&conversations).Error
		if err != nil {
			return err
		}
		if len(conversations) == 0 {
			return nil
		}
		for _, conv := range conversations {
			cursor = conv.ID
			settings := model.DefaultSupportInboxSettings()
			if s.installationRepo != nil {
				inst, err := s.installationRepo.GetByWorkspace(ctx, conv.WorkspaceID)
				if err != nil {
					return err
				}
				if inst != nil {
					settings = parseSettings(inst.Settings)
				}
			}
			if _, err := s.sendDelayedTeamReply(ctx, conv.ID, settings, now); err != nil {
				return err
			}
		}
	}
}

func (s *SupportInboxService) sendDelayedTeamReply(ctx context.Context, conversationID string, settings model.SupportInboxSettings, now time.Time) (*model.SupportMessage, error) {
	var sent *model.SupportMessage
	err := s.conversationRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv model.SupportConversation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", conversationID).First(&conv).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		minutes := settings.DelayedTeamReplyMinutes
		if minutes < 1 || minutes > 1440 {
			minutes = 5
		}
		if conv.Channel != "widget" || conv.Status != "open" || derefString(conv.AIState) != "escalated" || conv.AIEscalatedAt == nil || now.Before(conv.AIEscalatedAt.Add(time.Duration(minutes)*time.Minute)) {
			return nil
		}
		if conv.DelayedTeamReplySentFor != nil && !conv.DelayedTeamReplySentFor.Before(*conv.AIEscalatedAt) {
			return nil
		}
		var replies int64
		// Include removed replies: a real response still cancels this handoff's timer.
		if err := tx.Unscoped().Model(&model.SupportMessage{}).Where("conversation_id = ? AND workspace_id = ? AND created_at >= ? AND is_internal = ? AND message_type = ? AND sender_type IN ?", conv.ID, conv.WorkspaceID, *conv.AIEscalatedAt, false, "reply", []string{"user", "agent"}).Count(&replies).Error; err != nil {
			return err
		}
		if replies == 0 {
			canEmail := s.emailFallbackService != nil && s.emailFallbackService.emailClient != nil && s.emailFallbackService.redis != nil && !conv.EmailUnsubscribed
			capture := canEmail && strings.TrimSpace(derefString(conv.CustomerEmail)) == ""
			defaults := model.DefaultSupportInboxSettings()
			content := strings.TrimSpace(settings.DelayedTeamReplyMessage)
			if content == "" {
				content = defaults.DelayedTeamReplyMessage
			}
			if capture {
				content = strings.TrimSpace(settings.DelayedTeamReplyMessageNoEmail)
				if content == "" {
					content = defaults.DelayedTeamReplyMessageNoEmail
				}
			}
			if !canEmail {
				content = "Our team hasn’t been able to reply yet. Your conversation is still waiting for a teammate. You can return to this chat to check for a reply."
			}
			metadata, _ := json.Marshal(map[string]any{"delayed_team_reply": true, "capture_email": capture})
			sent = &model.SupportMessage{WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID, SenderType: "ai", MessageType: "system", SystemEventType: strPtr("delayed_team_reply"), SenderDisplayName: strPtr(helpinAIDisplayName), Content: content, Metadata: string(metadata), CreatedAt: now}
			if err := s.messageRepo.WithTx(tx).Create(ctx, sent); err != nil {
				return err
			}
		}
		return tx.Model(&conv).Update("delayed_team_reply_sent_for", conv.AIEscalatedAt).Error
	})
	if err != nil {
		return nil, err
	}
	if sent != nil && s.wsPublisher != nil {
		s.wsPublisher.Publish(websocket.SupportMessageEvent(sent.WorkspaceID, sent, "support:delayed_team_reply"))
	}
	return sent, nil
}
