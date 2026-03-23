package repository

import (
	"context"
	"fmt"
	"time"

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
	if err := r.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("created_at ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// ListBySpaceID returns all knowledge sources linked to a docs space.
func (r *AgentKnowledgeSourceRepository) ListBySpaceID(ctx context.Context, workspaceID, spaceID string) ([]model.AgentKnowledgeSource, error) {
	var sources []model.AgentKnowledgeSource
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND space_id = ?", workspaceID, spaceID).
		Order("created_at ASC").
		Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// GetByID returns a knowledge source row by ID.
func (r *AgentKnowledgeSourceRepository) GetByID(ctx context.Context, id string) (*model.AgentKnowledgeSource, error) {
	var source model.AgentKnowledgeSource
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&source).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &source, nil
}

// Create inserts a knowledge source link.
func (r *AgentKnowledgeSourceRepository) Create(ctx context.Context, source *model.AgentKnowledgeSource) error {
	return r.db.WithContext(ctx).Create(source).Error
}

// DeleteByAgentAndSpace removes a single knowledge source link.
func (r *AgentKnowledgeSourceRepository) DeleteByAgentAndSpace(ctx context.Context, agentID, spaceID string) error {
	return r.db.WithContext(ctx).
		Where("agent_id = ? AND space_id = ?", agentID, spaceID).
		Delete(&model.AgentKnowledgeSource{}).Error
}

// UpdateSyncState updates user-visible sync progress for a knowledge source.
func (r *AgentKnowledgeSourceRepository) UpdateSyncState(
	ctx context.Context,
	id string,
	status string,
	progress int,
	documentCount int,
	chunkCount int,
	errMessage *string,
	startedAt *time.Time,
	completedAt *time.Time,
) error {
	updates := map[string]any{
		"sync_status":            status,
		"sync_progress":          progress,
		"indexed_documents":      documentCount,
		"indexed_chunks":         chunkCount,
		"last_sync_error":        errMessage,
		"last_sync_started_at":   startedAt,
		"last_sync_completed_at": completedAt,
		"updated_at":             time.Now(),
	}
	return r.db.WithContext(ctx).
		Model(&model.AgentKnowledgeSource{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// MarkSyncQueued moves a knowledge source into the queued state.
func (r *AgentKnowledgeSourceRepository) MarkSyncQueued(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Model(&model.AgentKnowledgeSource{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"sync_status":     model.KnowledgeSourceSyncQueued,
			"sync_progress":   0,
			"last_sync_error": nil,
			"updated_at":      time.Now(),
		}).Error
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
