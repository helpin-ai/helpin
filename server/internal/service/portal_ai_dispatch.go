package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

// PortalAIDispatchService publishes saved portal messages to the existing
// support AI stream. Publishing never blocks the customer's request response.
type PortalAIDispatchService struct {
	repo *repository.PortalAIDispatchRepository
	ai   *SupportAIService
}

func NewPortalAIDispatchService(repo *repository.PortalAIDispatchRepository, ai *SupportAIService) *PortalAIDispatchService {
	return &PortalAIDispatchService{repo: repo, ai: ai}
}

func (s *PortalAIDispatchService) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.tick(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "portal AI dispatch sweep failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *PortalAIDispatchService) tick(ctx context.Context) error {
	rows, err := s.repo.ClaimDue(ctx, time.Now(), 20)
	if err != nil {
		return err
	}
	for _, row := range rows {
		content, loadErr := s.repo.MessageContent(ctx, row)
		if loadErr == nil {
			loadErr = s.ai.PublishAIRequest(ctx, row.WorkspaceID, row.ConversationID, row.SourceMessageID, content)
		}
		if loadErr == nil {
			if err := s.repo.MarkPublished(ctx, row.SourceMessageID); err != nil {
				slog.ErrorContext(ctx, "mark portal AI dispatch published", "message_id", row.SourceMessageID, "error", err)
			}
			continue
		}
		failed := row.Attempts >= 5
		if err := s.repo.Retry(ctx, row.SourceMessageID, time.Now().Add(time.Duration(row.Attempts*row.Attempts)*5*time.Second), failed); err != nil {
			slog.ErrorContext(ctx, "retry portal AI dispatch", "message_id", row.SourceMessageID, "error", err)
		}
		if failed {
			if err := s.ai.EscalateToHumanForMessage(ctx, row.WorkspaceID, row.ConversationID, row.SourceMessageID, "ai_pipeline_error"); err != nil {
				slog.ErrorContext(ctx, "portal AI dispatch handoff failed", "message_id", row.SourceMessageID, "error", err)
			}
		}
		slog.ErrorContext(ctx, "portal AI dispatch failed", "message_id", row.SourceMessageID, "attempts", row.Attempts, "error", loadErr)
	}
	return nil
}
