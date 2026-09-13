package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// FindUnambiguousOpenGapByConversation links feedback only when a conversation
// has one open gap. Multiple topics need transcript analysis before attribution.
func (r *SupportCoverageRepository) FindUnambiguousOpenGapByConversation(ctx context.Context, workspaceID, conversationID string) (*model.SupportCoverageGap, error) {
	var gaps []model.SupportCoverageGap
	err := r.db.WithContext(ctx).
		Table("support_coverage_gaps g").
		Where("g.workspace_id = ? AND g.status = ?", workspaceID, model.SupportCoverageGapStatusOpen).
		Where("EXISTS (SELECT 1 FROM support_gap_evidence e WHERE e.gap_id = g.id AND e.workspace_id = ? AND e.conversation_id = ?)", workspaceID, conversationID).
		Limit(2).Select("g.*").Find(&gaps).Error
	if err != nil {
		return nil, fmt.Errorf("find unambiguous gap for feedback: %w", err)
	}
	if len(gaps) != 1 {
		return nil, nil
	}
	return &gaps[0], nil
}
