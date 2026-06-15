package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type AgentVersionRepository struct {
	db *gorm.DB
}

func NewAgentVersionRepository(db *gorm.DB) *AgentVersionRepository {
	return &AgentVersionRepository{db: db}
}

func (r *AgentVersionRepository) ListByAgent(ctx context.Context, workspaceID, agentID string) ([]model.AgentVersion, error) {
	var versions []model.AgentVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ? AND deleted_at IS NULL", workspaceID, agentID).
		Order("created_at ASC, id ASC").
		Find(&versions).Error; err != nil {
		return nil, fmt.Errorf("list agent versions: %w", err)
	}
	return versions, nil
}

func (r *AgentVersionRepository) GetByID(ctx context.Context, workspaceID, agentID, id string) (*model.AgentVersion, error) {
	var version model.AgentVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ? AND id = ? AND deleted_at IS NULL", workspaceID, agentID, id).
		First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent version: %w", err)
	}
	return &version, nil
}

func (r *AgentVersionRepository) GetActive(ctx context.Context, agent *model.Agent) (*model.AgentVersion, error) {
	if agent == nil || agent.ActiveVersionID == nil || *agent.ActiveVersionID == "" {
		return nil, nil
	}
	return r.GetByID(ctx, agent.WorkspaceID, agent.ID, *agent.ActiveVersionID)
}

func (r *AgentVersionRepository) GetByVersionKey(ctx context.Context, workspaceID, agentID, versionKey string) (*model.AgentVersion, error) {
	var version model.AgentVersion
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND agent_id = ? AND version_key = ? AND deleted_at IS NULL", workspaceID, agentID, versionKey).
		First(&version).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent version by key: %w", err)
	}
	return &version, nil
}

func (r *AgentVersionRepository) Create(ctx context.Context, version *model.AgentVersion) error {
	if err := r.db.WithContext(ctx).Create(version).Error; err != nil {
		return fmt.Errorf("create agent version: %w", err)
	}
	return nil
}

func (r *AgentVersionRepository) Update(ctx context.Context, version *model.AgentVersion) error {
	if err := r.db.WithContext(ctx).Save(version).Error; err != nil {
		return fmt.Errorf("update agent version: %w", err)
	}
	return nil
}

func (r *AgentVersionRepository) Delete(ctx context.Context, workspaceID, agentID, id string) error {
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&model.AgentVersion{}).
		Where("workspace_id = ? AND agent_id = ? AND id = ? AND deleted_at IS NULL", workspaceID, agentID, id).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("delete agent version: %w", err)
	}
	return nil
}
