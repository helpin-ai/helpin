package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type WorkspaceSkillRepository struct {
	db *gorm.DB
}

func NewWorkspaceSkillRepository(db *gorm.DB) *WorkspaceSkillRepository {
	return &WorkspaceSkillRepository{db: db}
}

func (r *WorkspaceSkillRepository) ListByWorkspace(ctx context.Context, workspaceID string, includeArchived bool) ([]model.WorkspaceSkill, error) {
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if !includeArchived {
		query = query.Where("is_archived = ?", false)
	}
	var skills []model.WorkspaceSkill
	if err := query.Order("created_at DESC").Find(&skills).Error; err != nil {
		return nil, fmt.Errorf("list workspace skills: %w", err)
	}
	return skills, nil
}

func (r *WorkspaceSkillRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.WorkspaceSkill, error) {
	var skill model.WorkspaceSkill
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&skill).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace skill: %w", err)
	}
	return &skill, nil
}

func (r *WorkspaceSkillRepository) GetActiveByKey(ctx context.Context, workspaceID, key string) (*model.WorkspaceSkill, error) {
	var skill model.WorkspaceSkill
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND key = ? AND is_archived = ?", workspaceID, key, false).First(&skill).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace skill by key: %w", err)
	}
	return &skill, nil
}

func (r *WorkspaceSkillRepository) Create(ctx context.Context, skill *model.WorkspaceSkill) error {
	if err := r.db.WithContext(ctx).Create(skill).Error; err != nil {
		return fmt.Errorf("create workspace skill: %w", err)
	}
	return nil
}

func (r *WorkspaceSkillRepository) Update(ctx context.Context, skill *model.WorkspaceSkill) error {
	if err := r.db.WithContext(ctx).Save(skill).Error; err != nil {
		return fmt.Errorf("update workspace skill: %w", err)
	}
	return nil
}

func (r *WorkspaceSkillRepository) Archive(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).Model(&model.WorkspaceSkill{}).Where("workspace_id = ? AND id = ?", workspaceID, id).Update("is_archived", true).Error; err != nil {
		return fmt.Errorf("archive workspace skill: %w", err)
	}
	return nil
}
