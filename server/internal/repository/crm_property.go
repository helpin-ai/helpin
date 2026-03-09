package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMPropertyRepository handles DB operations for CRM property definitions and groups.
type CRMPropertyRepository struct {
	db *gorm.DB
}

// NewCRMPropertyRepository creates a new CRMPropertyRepository.
func NewCRMPropertyRepository(db *gorm.DB) *CRMPropertyRepository {
	return &CRMPropertyRepository{db: db}
}

// ── Property Definitions ──

// ListDefinitions returns property definitions for a workspace and object type.
func (r *CRMPropertyRepository) ListDefinitions(ctx context.Context, workspaceID, objectType string) ([]model.CRMPropertyDefinition, error) {
	var defs []model.CRMPropertyDefinition
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if objectType != "" {
		query = query.Where("object_type = ?", objectType)
	}
	if err := query.Order("position ASC, created_at ASC").Find(&defs).Error; err != nil {
		return nil, fmt.Errorf("list property definitions: %w", err)
	}
	return defs, nil
}

// GetDefinitionByID returns a property definition by ID.
func (r *CRMPropertyRepository) GetDefinitionByID(ctx context.Context, id string) (*model.CRMPropertyDefinition, error) {
	var def model.CRMPropertyDefinition
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&def).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get property definition: %w", err)
	}
	return &def, nil
}

// GetDefinitionByInternalName returns a property definition by workspace, object type and internal name.
func (r *CRMPropertyRepository) GetDefinitionByInternalName(ctx context.Context, workspaceID, objectType, internalName string) (*model.CRMPropertyDefinition, error) {
	var def model.CRMPropertyDefinition
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND object_type = ? AND internal_name = ?", workspaceID, objectType, internalName).First(&def).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get property definition by name: %w", err)
	}
	return &def, nil
}

// CreateDefinition inserts a property definition.
func (r *CRMPropertyRepository) CreateDefinition(ctx context.Context, def *model.CRMPropertyDefinition) error {
	if err := r.db.WithContext(ctx).Create(def).Error; err != nil {
		return fmt.Errorf("create property definition: %w", err)
	}
	return nil
}

// UpdateDefinition updates a property definition.
func (r *CRMPropertyRepository) UpdateDefinition(ctx context.Context, def *model.CRMPropertyDefinition) error {
	if err := r.db.WithContext(ctx).Save(def).Error; err != nil {
		return fmt.Errorf("update property definition: %w", err)
	}
	return nil
}

// DeleteDefinition removes a property definition.
func (r *CRMPropertyRepository) DeleteDefinition(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMPropertyDefinition{}).Error; err != nil {
		return fmt.Errorf("delete property definition: %w", err)
	}
	return nil
}

// ── Property Groups ──

// ListGroups returns property groups for a workspace and object type.
func (r *CRMPropertyRepository) ListGroups(ctx context.Context, workspaceID, objectType string) ([]model.CRMPropertyGroup, error) {
	var groups []model.CRMPropertyGroup
	query := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if objectType != "" {
		query = query.Where("object_type = ?", objectType)
	}
	if err := query.Order("position ASC, created_at ASC").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("list property groups: %w", err)
	}
	return groups, nil
}

// GetGroupByID returns a property group by ID.
func (r *CRMPropertyRepository) GetGroupByID(ctx context.Context, id string) (*model.CRMPropertyGroup, error) {
	var group model.CRMPropertyGroup
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get property group: %w", err)
	}
	return &group, nil
}

// CreateGroup inserts a property group.
func (r *CRMPropertyRepository) CreateGroup(ctx context.Context, group *model.CRMPropertyGroup) error {
	if err := r.db.WithContext(ctx).Create(group).Error; err != nil {
		return fmt.Errorf("create property group: %w", err)
	}
	return nil
}

// UpdateGroup updates a property group.
func (r *CRMPropertyRepository) UpdateGroup(ctx context.Context, group *model.CRMPropertyGroup) error {
	if err := r.db.WithContext(ctx).Save(group).Error; err != nil {
		return fmt.Errorf("update property group: %w", err)
	}
	return nil
}

// DeleteGroup removes a property group.
func (r *CRMPropertyRepository) DeleteGroup(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMPropertyGroup{}).Error; err != nil {
		return fmt.Errorf("delete property group: %w", err)
	}
	return nil
}
