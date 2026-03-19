package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentKnowledgeSourceRepository handles CRUD for agent-to-space knowledge links.
type AgentKnowledgeSourceRepository struct {
	db *gorm.DB
}

// NewAgentKnowledgeSourceRepository creates a new AgentKnowledgeSourceRepository.
func NewAgentKnowledgeSourceRepository(db *gorm.DB) *AgentKnowledgeSourceRepository {
	return &AgentKnowledgeSourceRepository{db: db}
}

// ListByAgentID returns all knowledge source links for an agent.
func (r *AgentKnowledgeSourceRepository) ListByAgentID(ctx context.Context, agentID string) ([]model.AgentKnowledgeSource, error) {
	var sources []model.AgentKnowledgeSource
	if err := r.db.WithContext(ctx).Where("agent_id = ?", agentID).Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// ListSpaceIDs returns just the space IDs linked to an agent.
func (r *AgentKnowledgeSourceRepository) ListSpaceIDs(ctx context.Context, agentID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.AgentKnowledgeSource{}).
		Where("agent_id = ?", agentID).
		Pluck("space_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// Set replaces all knowledge sources for an agent with the given space IDs.
// Validates that both agent and all spaces belong to the given workspace.
func (r *AgentKnowledgeSourceRepository) Set(ctx context.Context, workspaceID, agentID string, spaceIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Validate agent belongs to workspace.
		var agentCount int64
		if err := tx.Model(&model.Agent{}).Where("id = ? AND workspace_id = ?", agentID, workspaceID).Count(&agentCount).Error; err != nil {
			return err
		}
		if agentCount == 0 {
			return fmt.Errorf("agent not found in workspace")
		}

		// Validate all spaces belong to workspace.
		if len(spaceIDs) > 0 {
			var spaceCount int64
			if err := tx.Model(&model.DocsSpace{}).Where("id IN ? AND workspace_id = ?", spaceIDs, workspaceID).Count(&spaceCount).Error; err != nil {
				return err
			}
			if int(spaceCount) != len(spaceIDs) {
				return fmt.Errorf("one or more space_ids do not belong to this workspace")
			}
		}

		// Delete existing links.
		if err := tx.Where("agent_id = ?", agentID).Delete(&model.AgentKnowledgeSource{}).Error; err != nil {
			return err
		}

		// Insert new links.
		for _, spaceID := range spaceIDs {
			src := model.AgentKnowledgeSource{
				AgentID:     agentID,
				SpaceID:     spaceID,
				WorkspaceID: workspaceID,
			}
			if err := tx.Create(&src).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
