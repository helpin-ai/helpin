package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMSuggestionRepository handles DB operations for CRM suggestions.
type CRMSuggestionRepository struct {
	db *gorm.DB
}

// NewCRMSuggestionRepository creates a new CRMSuggestionRepository.
func NewCRMSuggestionRepository(db *gorm.DB) *CRMSuggestionRepository {
	return &CRMSuggestionRepository{db: db}
}

// Create inserts a suggestion.
func (r *CRMSuggestionRepository) Create(ctx context.Context, suggestion *model.CRMSuggestion) error {
	if err := r.db.WithContext(ctx).Create(suggestion).Error; err != nil {
		return fmt.Errorf("create suggestion: %w", err)
	}
	return nil
}

// GetByID returns a suggestion by ID.
func (r *CRMSuggestionRepository) GetByID(ctx context.Context, id string) (*model.CRMSuggestion, error) {
	var suggestion model.CRMSuggestion
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&suggestion).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get suggestion: %w", err)
	}
	return &suggestion, nil
}

// List returns suggestions with optional filters.
func (r *CRMSuggestionRepository) List(ctx context.Context, workspaceID string, filters model.CRMSuggestionListFilters, pagination model.PMPagination) ([]model.CRMSuggestion, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMSuggestion{}).Where("workspace_id = ?", workspaceID)

	if filters.UserID != nil && *filters.UserID != "" {
		query = query.Where("user_id = ?", *filters.UserID)
	}
	if filters.SuggestionType != nil && *filters.SuggestionType != "" {
		query = query.Where("suggestion_type = ?", *filters.SuggestionType)
	}
	if filters.ObjectType != nil && *filters.ObjectType != "" {
		query = query.Where("object_type = ?", *filters.ObjectType)
	}
	if filters.ObjectID != nil && *filters.ObjectID != "" {
		query = query.Where("object_id = ?", *filters.ObjectID)
	}
	if filters.Status != nil && *filters.Status != "" {
		query = query.Where("status = ?", *filters.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count suggestions: %w", err)
	}

	var suggestions []model.CRMSuggestion
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&suggestions).Error; err != nil {
		return nil, 0, fmt.Errorf("list suggestions: %w", err)
	}
	return suggestions, total, nil
}

// Update updates a suggestion.
func (r *CRMSuggestionRepository) Update(ctx context.Context, suggestion *model.CRMSuggestion) error {
	if err := r.db.WithContext(ctx).Save(suggestion).Error; err != nil {
		return fmt.Errorf("update suggestion: %w", err)
	}
	return nil
}

// Delete removes a suggestion.
func (r *CRMSuggestionRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMSuggestion{}).Error; err != nil {
		return fmt.Errorf("delete suggestion: %w", err)
	}
	return nil
}
