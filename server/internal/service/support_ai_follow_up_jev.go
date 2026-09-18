package service

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// assessJevFollowUp bypasses generative assessment for clear handoff/skip
// outcomes. Only waiting_customer continues to Runtime for customer-facing copy.
// A missing/uncertain result uses the original full Runtime assessment.
func (s *SupportFollowUpService) assessJevFollowUp(ctx context.Context, row model.SupportAIFollowUp, snapshot *model.SupportConversation, now time.Time) (bool, string, error) {
	jev := s.chat.jev
	if !jev.enabled(row.WorkspaceID) || jev.config.FollowUpMode == "off" {
		return false, "", nil
	}
	history, err := s.chat.messageRepo.ListByConversation(ctx, row.WorkspaceID, row.ConversationID, false)
	if err != nil {
		return false, "", err
	}
	classification, err := jev.classifyFollowUp(ctx, row, history)
	if err != nil {
		slog.WarnContext(ctx, "Jev follow-up assessment unavailable; using support agent", "workspace_id", row.WorkspaceID, "conversation_id", row.ConversationID, "error", err)
		return false, "", nil
	}
	if classification == "" {
		return false, "", nil
	}
	var sent *model.SupportMessage
	handled := true
	err = s.repo.WithEpisode(ctx, row.WorkspaceID, row.ID, func(tx *gorm.DB, inst *model.SupportWidgetInstallation, conv *model.SupportConversation, e *model.SupportAIFollowUp) error {
		if e.Status != "assessing" || e.RunID != row.RunID {
			return nil
		}
		settings := parseSettings(inst.Settings)
		channel, err := supportFollowUpReplyChannel(ctx, tx, conv)
		if err != nil {
			return err
		}
		if !inst.Active || conv.AIControlVersion != snapshot.AIControlVersion || !supportFollowUpEligible(conv, e, settings, channel) {
			return finishFollowUpRow(tx, e, "cancelled", "conversation_changed", now)
		}
		if now.Before(e.DueAt) || (e.StartedAt != nil && now.Sub(*e.StartedAt) > time.Hour) {
			return nil
		}
		busy, err := repository.SupportConversationBusy(ctx, tx, conv.WorkspaceID, conv.ID, "")
		if err != nil {
			return err
		}
		if busy {
			return finishFollowUpRow(tx, e, "cancelled", "conversation_busy", now)
		}
		latest, err := s.chat.messageRepo.WithTx(tx).ListByConversation(ctx, conv.WorkspaceID, conv.ID, false)
		if err != nil {
			return err
		}
		before, valid := supportJevLifecycleState(row.WorkspaceID, row.ConversationID, row.SourceMessageID, history)
		after, stillValid := supportJevLifecycleState(row.WorkspaceID, row.ConversationID, row.SourceMessageID, latest)
		if !valid || !stillValid || before != after {
			return finishFollowUpRow(tx, e, "cancelled", "conversation_changed", now)
		}
		switch classification {
		case "waiting_customer":
			handled = false
			return nil
		case "confirmed_resolved", "no_follow_up":
			// Recognizing confirmation suppresses needless reminders; it does not
			// claim an inactivity resolution or silently close a conversation.
			return finishFollowUpRow(tx, e, "skipped", supportJevFollowUpReason(classification), now)
		case "team_owes_work", "needs_human":
			sent, err = s.finishHandoff(ctx, tx, conv, e, settings, supportJevFollowUpReason(classification), now)
			return err
		default:
			handled = false
			return nil
		}
	})
	if err != nil {
		return false, "", err
	}
	if sent != nil {
		s.publishFollowUpMessage(row.WorkspaceID, sent)
	}
	if handled && s.chat.supportAIService.wsPublisher != nil {
		s.chat.supportAIService.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: row.ConversationID, WorkspaceID: row.WorkspaceID})
	}
	return handled, classification, nil
}

func supportJevFollowUpReason(classification string) string {
	switch classification {
	case "team_owes_work":
		return "The team owes the customer an answer or promised action."
	case "needs_human":
		return "The unresolved issue needs a teammate's help."
	case "confirmed_resolved":
		return "The customer explicitly confirmed the issue was resolved."
	case "no_follow_up":
		return "There is no substantive support issue to follow up."
	default:
		return "The conversation needs further assessment."
	}
}
