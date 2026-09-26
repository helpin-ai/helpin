package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// linkReviewedConversationTask retains canonical activity and realtime effects
// while fencing both the source evidence and the chosen target at commit time.
func (s *SupportInboxService) linkReviewedConversationTask(ctx context.Context, workspaceID, conversationID, taskID, actorID, sourceHash string, taskRevision time.Time) error {
	if _, err := s.loadConversationAccessible(ctx, workspaceID, conversationID); err != nil {
		return err
	}
	err := s.conversationRepo.LinkTaskReviewed(ctx, workspaceID, conversationID, taskID, func(conversation *model.SupportConversation, messages []model.SupportMessage, task *model.PMTask) error {
		_, currentHash := supportPMTriageEvidence(conversation, messages)
		if conversation.AnonymizedAt != nil || currentHash != sourceHash || task.Archived || !task.UpdatedAt.Equal(taskRevision) || !canAccessTeam(ctx, task.TeamID) {
			return ErrPMTriageStale
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.publishReviewedTaskLink(ctx, workspaceID, conversationID, taskID, actorID)
	return nil
}

func (s *SupportInboxService) publishReviewedTaskLink(ctx context.Context, workspaceID, conversationID, taskID, actorID string) {
	if s.activitySvc != nil {
		if err := s.activitySvc.Log(ctx, workspaceID, "support_conversation", conversationID, &actorID, "updated", strPtr("linked_task_id"), nil, &taskID, nil); err != nil {
			slog.ErrorContext(ctx, "failed to record reviewed support task link activity", "workspace_id", workspaceID, "conversation_id", conversationID, "error", err)
		}
	}
	s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: conversationID, WorkspaceID: workspaceID, ActorID: actorID})
}

// supportTaskCreateReview is internal-only; HTTP callers cannot supply a guard.
type supportTaskCreateReview struct {
	conversationID string
	sourceHash     string
}

func (review *supportTaskCreateReview) validate(conversation *model.SupportConversation, messages []model.SupportMessage) error {
	_, hash := supportPMTriageEvidence(conversation, messages)
	if conversation.AnonymizedAt != nil || hash != review.sourceHash {
		return ErrPMTriageStale
	}
	return nil
}
