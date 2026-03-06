package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentHandoffRepository handles DB operations for agent handoffs.
type AgentHandoffRepository struct {
	db *gorm.DB
}

// NewAgentHandoffRepository creates a new AgentHandoffRepository.
func NewAgentHandoffRepository(db *gorm.DB) *AgentHandoffRepository {
	return &AgentHandoffRepository{db: db}
}

// Create creates a new handoff record.
func (r *AgentHandoffRepository) Create(ctx context.Context, handoff *model.AgentHandoff) error {
	if err := r.db.WithContext(ctx).Create(handoff).Error; err != nil {
		return fmt.Errorf("create agent handoff: %w", err)
	}
	return nil
}

// ListByEpic returns handoffs for an epic.
func (r *AgentHandoffRepository) ListByEpic(ctx context.Context, workspaceID, epicID string) ([]model.AgentHandoff, error) {
	var handoffs []model.AgentHandoff
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND epic_id = ?", workspaceID, epicID).Order("created_at DESC").Find(&handoffs).Error; err != nil {
		return nil, fmt.Errorf("list epic handoffs: %w", err)
	}
	return handoffs, nil
}

// ListByStory returns handoffs for a story.
func (r *AgentHandoffRepository) ListByStory(ctx context.Context, workspaceID, storyID string) ([]model.AgentHandoff, error) {
	var handoffs []model.AgentHandoff
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND story_id = ?", workspaceID, storyID).Order("created_at DESC").Find(&handoffs).Error; err != nil {
		return nil, fmt.Errorf("list story handoffs: %w", err)
	}
	return handoffs, nil
}
