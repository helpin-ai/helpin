package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// chatHistoryQuery excludes delivery failures and progress before pagination so
// repeated tool retries cannot displace substantive conversation history.
func (r *AgentRunMessageRepository) chatHistoryQuery(ctx context.Context, workspaceID, chatID string) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.AgentRunMessage{}).
		Select("id, workspace_id, run_id, dock_chat_id, dock_chat_sequence, role, content, message_type").
		Where("workspace_id = ? AND dock_chat_id = ? AND dock_chat_sequence IS NOT NULL AND delivery_status <> ?", workspaceID, chatID, "failed").
		Where("role = ? OR (role = ? AND message_type = ?)", "user", "assistant", "assistant_final")
}

// ListChatHistory returns substantive messages, newest page first but ordered
// chronologically within each page. The cursor spans backing runs.
func (r *AgentRunMessageRepository) ListChatHistory(ctx context.Context, workspaceID, chatID string, before int64, limit int) ([]model.AgentRunMessage, *int64, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	query := r.chatHistoryQuery(ctx, workspaceID, chatID)
	if before > 0 {
		query = query.Where("dock_chat_sequence < ?", before)
	}
	messages := make([]model.AgentRunMessage, 0)
	if err := query.Order("dock_chat_sequence DESC").Limit(limit + 1).Find(&messages).Error; err != nil {
		return nil, nil, fmt.Errorf("read chat history: %w", err)
	}
	var next *int64
	if len(messages) > limit {
		messages = messages[:limit]
		seq := *messages[len(messages)-1].DockChatSequence
		next = &seq
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, next, nil
}

func (r *AgentRunMessageRepository) FirstChatRequest(ctx context.Context, workspaceID, chatID string) (*model.AgentRunMessage, error) {
	var message model.AgentRunMessage
	err := r.chatHistoryQuery(ctx, workspaceID, chatID).Where("role = ?", "user").Order("dock_chat_sequence ASC").First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read first chat request: %w", err)
	}
	return &message, nil
}

// GetChatHistoryMessage binds retrieval to one chat and workspace, never a
// caller-supplied run or global message ID.
func (r *AgentRunMessageRepository) GetChatHistoryMessage(ctx context.Context, workspaceID, chatID string, sequence int64) (*model.AgentRunMessage, error) {
	var message model.AgentRunMessage
	err := r.chatHistoryQuery(ctx, workspaceID, chatID).Where("dock_chat_sequence = ?", sequence).First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read chat history message: %w", err)
	}
	return &message, nil
}

// ListRecentChatArtifactReferences reads only safe reference fields, not file
// contents, storage keys or arbitrary metadata. Handoff output stays bounded.
func (r *AgentRunArtifactRepository) ListRecentChatArtifactReferences(ctx context.Context, workspaceID, chatID string) ([]model.AgentRunArtifact, error) {
	var artifacts []model.AgentRunArtifact
	err := r.db.WithContext(ctx).Table("agent_run_artifacts AS artifact").Select("artifact.id, artifact.run_id, artifact.artifact_type").
		Joins("JOIN agent_runs AS run ON run.id = artifact.run_id AND run.workspace_id = artifact.workspace_id").
		Where("artifact.workspace_id = ? AND run.dock_chat_id = ? AND artifact.storage_mode = ?", workspaceID, chatID, "object").
		Order("artifact.created_at DESC, artifact.sequence_no DESC, artifact.id DESC").Limit(20).Scan(&artifacts).Error
	if err != nil {
		return nil, fmt.Errorf("read chat artifact references: %w", err)
	}
	return artifacts, nil
}
