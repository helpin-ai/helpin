package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AutomationRuleRepository handles DB operations for automation rules.
type AutomationRuleRepository struct {
	db *gorm.DB
}

// NewAutomationRuleRepository creates a new AutomationRuleRepository.
func NewAutomationRuleRepository(db *gorm.DB) *AutomationRuleRepository {
	return &AutomationRuleRepository{db: db}
}

// Create inserts a new automation rule.
func (r *AutomationRuleRepository) Create(ctx context.Context, rule *model.AutomationRule) error {
	if err := r.db.WithContext(ctx).Create(rule).Error; err != nil {
		return fmt.Errorf("create automation rule: %w", err)
	}
	return nil
}

// Update saves changes to an existing automation rule.
func (r *AutomationRuleRepository) Update(ctx context.Context, rule *model.AutomationRule) error {
	if err := r.db.WithContext(ctx).Save(rule).Error; err != nil {
		return fmt.Errorf("update automation rule: %w", err)
	}
	return nil
}

// Delete removes an automation rule by workspace and rule ID.
func (r *AutomationRuleRepository) Delete(ctx context.Context, workspaceID, ruleID string) error {
	result := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, ruleID).
		Delete(&model.AutomationRule{})
	if result.Error != nil {
		return fmt.Errorf("delete automation rule: %w", result.Error)
	}
	return nil
}

// GetByID returns a single automation rule.
func (r *AutomationRuleRepository) GetByID(ctx context.Context, workspaceID, ruleID string) (*model.AutomationRule, error) {
	var rule model.AutomationRule
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, ruleID).
		First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get automation rule: %w", err)
	}
	return &rule, nil
}

// ListByWorkspace returns all automation rules for a workspace, ordered by position.
func (r *AutomationRuleRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.AutomationRule, error) {
	var rules []model.AutomationRule
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("trigger_type ASC, position ASC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list automation rules: %w", err)
	}
	return rules, nil
}

// ListByWorkflow returns all automation rules scoped to a specific workflow.
func (r *AutomationRuleRepository) ListByWorkflow(ctx context.Context, workspaceID, workflowID string) ([]model.AutomationRule, error) {
	var rules []model.AutomationRule
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND workflow_id = ?", workspaceID, workflowID).
		Order("trigger_type ASC, position ASC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list automation rules by workflow: %w", err)
	}
	return rules, nil
}

// ListMatchingRules returns enabled rules matching a trigger type for a workspace, ordered by position.
func (r *AutomationRuleRepository) ListMatchingRules(ctx context.Context, workspaceID, triggerType string) ([]model.AutomationRule, error) {
	var rules []model.AutomationRule
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND trigger_type = ? AND enabled = true", workspaceID, triggerType).
		Order("position ASC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("list matching automation rules: %w", err)
	}
	return rules, nil
}
