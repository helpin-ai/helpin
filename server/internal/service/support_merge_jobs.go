package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"gorm.io/gorm"
)

// RunMergeReconciliation recovers post-commit work without giving stale AI runs
// publication rights while runtime cancellation is being retried.
func (s *SupportInboxService) RunMergeReconciliation(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for i := 0; i < 50 && ctx.Err() == nil; i++ {
			job, err := s.mergeRepository().ClaimJob(ctx)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			if err != nil {
				slog.ErrorContext(ctx, "claim merge reconciliation", "error", err)
				break
			}
			taskCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			err = s.reconcileMerge(taskCtx, job)
			cancel()
			if err != nil {
				slog.WarnContext(ctx, "merge reconciliation will retry", "error", err, "workspace_id", job.WorkspaceID, "conversation_id", job.ConversationID)
			}
			if finishErr := s.mergeRepository().FinishJob(ctx, job, err == nil); finishErr != nil {
				slog.ErrorContext(ctx, "save merge reconciliation", "error", finishErr, "job_id", job.ID)
			}
		}
	}
}

func (s *SupportInboxService) reconcileMerge(ctx context.Context, job *model.SupportMergeJob) error {
	if job.RunID != nil && *job.RunID != "" {
		run, err := repository.NewAgentRunRepository(s.conversationRepo.DB()).GetByID(ctx, job.WorkspaceID, *job.RunID)
		if err != nil {
			return err
		}
		if run != nil && model.IsAgentRunActiveStatus(run.Status) {
			if s.supportAIService == nil || s.supportAIService.runCloser == nil {
				return errors.New("agent runtime cancellation is unavailable")
			}
			if _, err := s.supportAIService.runCloser.CancelRun(ctx, job.WorkspaceID, *job.RunID, job.ActorID); err != nil {
				return err
			}
		}
	}
	if s.wsPublisher != nil {
		records, err := s.mergeRepository().Conversations(ctx, job.WorkspaceID, []string{job.ConversationID})
		if err != nil {
			return err
		}
		for _, c := range records {
			if c.AnonymousID != nil {
				s.pushVisitorConversationsRefresh(ctx, job.WorkspaceID, *c.AnonymousID)
			}
		}
		s.wsPublisher.Publish(websocket.Event{Action: "updated", Entity: "support_conversation", EntityID: job.ConversationID, WorkspaceID: job.WorkspaceID, ActorID: job.ActorID})
	}
	return nil
}
