package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ListActiveByTarget returns active runs newest first, allowing a product
// lifecycle to distinguish interactive work from bounded background work.
func (r *AgentRunRepository) ListActiveByTarget(ctx context.Context, workspaceID, targetType, targetID string) ([]model.AgentRun, error) {
	var rows []model.AgentRun
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND target_type = ? AND target_id = ? AND status IN ?", workspaceID, targetType, targetID, []string{"queued", "running", "paused"}).Order("created_at DESC,id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list active target runs: %w", err)
	}
	return rows, nil
}
