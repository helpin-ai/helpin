package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type AgentTemplateRepository struct {
	db *gorm.DB
}

func NewAgentTemplateRepository(db *gorm.DB) *AgentTemplateRepository {
	return &AgentTemplateRepository{db: db}
}

func (r *AgentTemplateRepository) WithTx(tx *gorm.DB) *AgentTemplateRepository {
	if tx == nil {
		return r
	}
	return &AgentTemplateRepository{db: tx}
}

func (r *AgentTemplateRepository) ListVisible(ctx context.Context, workspaceID string) ([]model.AgentTemplate, error) {
	query := r.db.WithContext(ctx).
		Where("deleted_at IS NULL AND is_enabled = ?", true)
	if workspaceID == "" {
		query = query.Where("workspace_id IS NULL")
	} else {
		query = query.Where("workspace_id IS NULL OR workspace_id = ?", workspaceID)
	}
	var templates []model.AgentTemplate
	if err := query.Order("workspace_id IS NOT NULL DESC, created_at ASC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list agent templates: %w", err)
	}
	return templates, nil
}

func (r *AgentTemplateRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.AgentTemplate, error) {
	query := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL AND is_enabled = ?", id, true)
	if workspaceID == "" {
		query = query.Where("workspace_id IS NULL")
	} else {
		query = query.Where("workspace_id IS NULL OR workspace_id = ?", workspaceID)
	}
	var template model.AgentTemplate
	if err := query.First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent template by id: %w", err)
	}
	return &template, nil
}

func (r *AgentTemplateRepository) GetByKey(ctx context.Context, workspaceID *string, key string) (*model.AgentTemplate, error) {
	query := r.db.WithContext(ctx).Where("key = ? AND deleted_at IS NULL", key)
	if workspaceID == nil || *workspaceID == "" {
		query = query.Where("workspace_id IS NULL")
	} else {
		query = query.Where("workspace_id = ?", *workspaceID)
	}
	var template model.AgentTemplate
	if err := query.First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent template: %w", err)
	}
	return &template, nil
}

func (r *AgentTemplateRepository) Create(ctx context.Context, template *model.AgentTemplate) error {
	if err := r.db.WithContext(ctx).Create(template).Error; err != nil {
		return fmt.Errorf("create agent template: %w", err)
	}
	return nil
}

func (r *AgentTemplateRepository) Update(ctx context.Context, template *model.AgentTemplate) error {
	if err := r.db.WithContext(ctx).Save(template).Error; err != nil {
		return fmt.Errorf("update agent template: %w", err)
	}
	return nil
}
