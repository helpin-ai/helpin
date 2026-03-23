package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentContentSourceRepository handles agent-to-content-source links.
type AgentContentSourceRepository struct {
	db *gorm.DB
}

func NewAgentContentSourceRepository(db *gorm.DB) *AgentContentSourceRepository {
	return &AgentContentSourceRepository{db: db}
}

func (r *AgentContentSourceRepository) ListByAgentID(ctx context.Context, agentID string) ([]model.AgentContentSource, error) {
	var rows []model.AgentContentSource
	if err := r.db.WithContext(ctx).
		Where("agent_id = ?", agentID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list agent content sources: %w", err)
	}
	return rows, nil
}

func (r *AgentContentSourceRepository) ListByContentSourceID(ctx context.Context, workspaceID, contentSourceID string) ([]model.AgentContentSource, error) {
	var rows []model.AgentContentSource
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND content_source_id = ?", workspaceID, contentSourceID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list content source links: %w", err)
	}
	return rows, nil
}

func (r *AgentContentSourceRepository) Create(ctx context.Context, row *model.AgentContentSource) error {
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create agent content source link: %w", err)
	}
	return nil
}

func (r *AgentContentSourceRepository) DeleteByAgentAndSource(ctx context.Context, agentID, contentSourceID string) error {
	if err := r.db.WithContext(ctx).
		Where("agent_id = ? AND content_source_id = ?", agentID, contentSourceID).
		Delete(&model.AgentContentSource{}).Error; err != nil {
		return fmt.Errorf("delete agent content source link: %w", err)
	}
	return nil
}

func (r *AgentContentSourceRepository) DeleteByContentSourceID(ctx context.Context, contentSourceID string) error {
	if err := r.db.WithContext(ctx).
		Where("content_source_id = ?", contentSourceID).
		Delete(&model.AgentContentSource{}).Error; err != nil {
		return fmt.Errorf("delete content source links: %w", err)
	}
	return nil
}

func (r *AgentContentSourceRepository) ListContentSourceIDs(ctx context.Context, agentID string) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.AgentContentSource{}).
		Where("agent_id = ?", agentID).
		Pluck("content_source_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list agent content source ids: %w", err)
	}
	return ids, nil
}

func (r *AgentContentSourceRepository) GetByID(ctx context.Context, id string) (*model.AgentContentSource, error) {
	var row model.AgentContentSource
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent content source link: %w", err)
	}
	return &row, nil
}
