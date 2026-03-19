package repository

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// FlowTemplateRepository provides data access for flow templates and their nodes.
type FlowTemplateRepository struct {
	db *gorm.DB
}

// NewFlowTemplateRepository creates a new FlowTemplateRepository.
func NewFlowTemplateRepository(db *gorm.DB) *FlowTemplateRepository {
	return &FlowTemplateRepository{db: db}
}

// CreateTemplate creates a new flow template.
func (r *FlowTemplateRepository) CreateTemplate(ctx context.Context, t *model.FlowTemplate) error {
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return fmt.Errorf("create flow template: %w", err)
	}
	return nil
}

// UpdateTemplate updates an existing flow template.
func (r *FlowTemplateRepository) UpdateTemplate(ctx context.Context, t *model.FlowTemplate) error {
	if err := r.db.WithContext(ctx).Save(t).Error; err != nil {
		return fmt.Errorf("update flow template: %w", err)
	}
	return nil
}

// GetByID returns a flow template by ID, scoped to workspace or builtin.
func (r *FlowTemplateRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.FlowTemplate, error) {
	var t model.FlowTemplate
	if err := r.db.WithContext(ctx).
		Where("id = ? AND (workspace_id = ? OR (workspace_id IS NULL AND is_builtin = TRUE))", id, workspaceID).
		Preload("Nodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get flow template: %w", err)
	}
	return &t, nil
}

// GetBySlug returns a flow template by slug, checking workspace-specific first, then builtin.
func (r *FlowTemplateRepository) GetBySlug(ctx context.Context, workspaceID, slug string) (*model.FlowTemplate, error) {
	// Try workspace-specific first.
	var t model.FlowTemplate
	err := r.db.WithContext(ctx).
		Where("template_slug = ? AND workspace_id = ? AND status = ?", slug, workspaceID, model.FlowTemplateStatusActive).
		Preload("Nodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&t).Error
	if err == nil {
		return &t, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("get flow template by slug: %w", err)
	}

	// Fall back to builtin.
	err = r.db.WithContext(ctx).
		Where("template_slug = ? AND workspace_id IS NULL AND is_builtin = TRUE AND status = ?", slug, model.FlowTemplateStatusActive).
		Preload("Nodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get builtin flow template by slug: %w", err)
	}
	return &t, nil
}

// ListByWorkspace returns all active templates visible to a workspace (workspace-specific + builtins).
func (r *FlowTemplateRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.FlowTemplate, error) {
	var items []model.FlowTemplate
	if err := r.db.WithContext(ctx).
		Where("(workspace_id = ? OR (workspace_id IS NULL AND is_builtin = TRUE)) AND status = ?", workspaceID, model.FlowTemplateStatusActive).
		Preload("Nodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Order("is_builtin DESC, name ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list flow templates: %w", err)
	}
	return items, nil
}

// DeleteTemplate soft-deletes by setting status to archived.
func (r *FlowTemplateRepository) DeleteTemplate(ctx context.Context, workspaceID, id string) error {
	result := r.db.WithContext(ctx).
		Model(&model.FlowTemplate{}).
		Where("id = ? AND workspace_id = ?", id, workspaceID).
		Update("status", model.FlowTemplateStatusArchived)
	if result.Error != nil {
		return fmt.Errorf("archive flow template: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("flow template not found")
	}
	return nil
}

// --- Node operations ---

// CreateNode creates a new template node.
func (r *FlowTemplateRepository) CreateNode(ctx context.Context, node *model.FlowTemplateNode) error {
	if err := r.db.WithContext(ctx).Create(node).Error; err != nil {
		return fmt.Errorf("create flow template node: %w", err)
	}
	return nil
}

// UpdateNode updates an existing template node.
func (r *FlowTemplateRepository) UpdateNode(ctx context.Context, node *model.FlowTemplateNode) error {
	if err := r.db.WithContext(ctx).Save(node).Error; err != nil {
		return fmt.Errorf("update flow template node: %w", err)
	}
	return nil
}

// DeleteNode deletes a template node.
func (r *FlowTemplateRepository) DeleteNode(ctx context.Context, templateID, nodeID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND template_id = ?", nodeID, templateID).
		Delete(&model.FlowTemplateNode{})
	if result.Error != nil {
		return fmt.Errorf("delete flow template node: %w", result.Error)
	}
	return nil
}

// ListNodesByTemplate returns all nodes for a template, ordered by position.
func (r *FlowTemplateRepository) ListNodesByTemplate(ctx context.Context, templateID string) ([]model.FlowTemplateNode, error) {
	var items []model.FlowTemplateNode
	if err := r.db.WithContext(ctx).
		Where("template_id = ?", templateID).
		Order("position ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list flow template nodes: %w", err)
	}
	return items, nil
}

// GetNodeBySlug returns a single node by template ID and slug.
func (r *FlowTemplateRepository) GetNodeBySlug(ctx context.Context, templateID, slug string) (*model.FlowTemplateNode, error) {
	var node model.FlowTemplateNode
	if err := r.db.WithContext(ctx).
		Where("template_id = ? AND node_slug = ?", templateID, slug).
		First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get flow template node: %w", err)
	}
	return &node, nil
}

// GetNodeByID returns a single node by ID.
func (r *FlowTemplateRepository) GetNodeByID(ctx context.Context, templateID, nodeID string) (*model.FlowTemplateNode, error) {
	var node model.FlowTemplateNode
	if err := r.db.WithContext(ctx).
		Where("template_id = ? AND id = ?", templateID, nodeID).
		First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get flow template node by id: %w", err)
	}
	return &node, nil
}
