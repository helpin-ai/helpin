package service

import (
	"context"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// attachVisitorAnswerFeedback enriches an existing gap without manufacturing one
// from dissatisfaction alone. The daily analyzer reads durable message feedback.
func (s *SupportCoverageService) attachVisitorAnswerFeedback(ctx context.Context, event *model.SupportEvent) error {
	if event.SourceSignal != "not_helpful" || event.ConversationID == nil || event.MessageID == nil {
		return nil
	}
	gap, err := s.coverageRepo.FindUnambiguousOpenGapByConversation(ctx, event.WorkspaceID, *event.ConversationID)
	if err != nil {
		return err
	}
	if gap == nil {
		return nil
	}
	now := time.Now().UTC()
	inserted, err := s.coverageRepo.CreateEvidenceIfAbsent(ctx, &model.SupportGapEvidence{
		GapID: gap.ID, WorkspaceID: event.WorkspaceID, EvidenceType: model.SupportEventAIAnswerFeedback,
		ConversationID: event.ConversationID, MessageID: event.MessageID, SourceSignal: event.SourceSignal,
		SourceKey: "answer-feedback:" + *event.MessageID, Excerpt: "Visitor marked this AI answer unhelpful. Review the answer and conversation before attributing this to a knowledge gap.", CreatedAt: now,
	})
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}
	return s.coverageRepo.IncrementGapEvidenceAfterInsert(ctx, event.WorkspaceID, gap.ID, now)
}
