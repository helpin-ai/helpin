package service

import (
	"context"
	"log/slog"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// stopPortalAITurn leaves a portal request for teammates after access is
// withdrawn. The ownership predicate prevents overwriting a teammate action.
func (s *SupportAIService) stopPortalAITurn(ctx context.Context, workspaceID, conversationID string) {
	result := s.conversationRepo.DB().WithContext(ctx).Model(&model.SupportConversation{}).
		Where("workspace_id = ? AND id = ? AND channel = ? AND (ai_state = ? OR flow_state = ? OR ai_active_run_id IS NOT NULL)", workspaceID, conversationID, "portal", "pending", model.SupportConversationFlowStateAIHandling).
		Where("assigned_user_id IS NULL AND opened_by_user_id IS NULL AND NOT coalesce(human_takeover, false)").
		Updates(map[string]any{"human_takeover": true, "assigned_agent_id": nil,
			"ai_state": "escalated", "flow_state": model.SupportConversationFlowStateWaitingForHuman,
			"ai_active_run_id": nil, "ai_control_version": gorm.Expr("ai_control_version + 1")})
	if result.Error != nil {
		slog.ErrorContext(ctx, "stop portal AI after access change", "conversation_id", conversationID, "error", result.Error)
	}
}
