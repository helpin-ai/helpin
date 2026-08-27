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

func (r *CRMSuggestionRepository) hydrateSignals(ctx context.Context, suggestions []model.CRMSuggestion) error {
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, suggestion := range suggestions {
		for _, id := range suggestion.SignalIDs {
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var signals []model.CRMBuyerSignal
	workspaceID := suggestions[0].WorkspaceID
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id IN ?", workspaceID, ids).Find(&signals).Error; err != nil {
		return fmt.Errorf("load suggestion signals: %w", err)
	}
	byID := make(map[string]model.CRMBuyerSignal, len(signals))
	for _, signal := range signals {
		byID[signal.ID] = signal
	}
	for index := range suggestions {
		for _, id := range suggestions[index].SignalIDs {
			if signal, ok := byID[id]; ok && signal.WorkspaceID == suggestions[index].WorkspaceID {
				suggestions[index].Signals = append(suggestions[index].Signals, signal)
			}
		}
	}
	return nil
}

func (r *CRMSuggestionRepository) ValidateSignalIDs(ctx context.Context, workspaceID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMBuyerSignal{}).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).Count(&count).Error; err != nil {
		return fmt.Errorf("validate suggestion signals: %w", err)
	}
	if count != int64(len(ids)) {
		return fmt.Errorf("one or more signals were not found in workspace")
	}
	return nil
}

// CRMObjectExists verifies an execution target without allowing callers to
// select an arbitrary table name.
func (r *CRMSuggestionRepository) CRMObjectExists(ctx context.Context, workspaceID, objectType, id string) (bool, error) {
	table := ""
	switch objectType {
	case model.CRMObjectContact:
		table = "crm_contacts"
	case model.CRMObjectCompany:
		table = "crm_companies"
	case model.CRMObjectDeal:
		table = "crm_deals"
	case model.CRMObjectMeeting:
		table = "crm_meetings"
	default:
		return false, fmt.Errorf("unsupported CRM object type")
	}
	var count int64
	if err := r.db.WithContext(ctx).Table(table).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("validate CRM object: %w", err)
	}
	return count > 0, nil
}

// GetByID returns a suggestion by ID within a workspace.
func (r *CRMSuggestionRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.CRMSuggestion, error) {
	var suggestion model.CRMSuggestion
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&suggestion).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get suggestion: %w", err)
	}
	items := []model.CRMSuggestion{suggestion}
	if err := r.hydrateSignals(ctx, items); err != nil {
		return nil, err
	}
	suggestion = items[0]
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
	if err := r.hydrateSignals(ctx, suggestions); err != nil {
		return nil, 0, err
	}
	return suggestions, total, nil
}

// Update updates a suggestion.
func (r *CRMSuggestionRepository) Update(ctx context.Context, workspaceID string, suggestion *model.CRMSuggestion) error {
	result := r.db.WithContext(ctx).Model(&model.CRMSuggestion{}).
		Where("workspace_id = ? AND id = ?", workspaceID, suggestion.ID).
		Select("user_id", "suggestion_type", "object_type", "object_id", "title", "description", "context", "signal_ids", "status", "dismissal_reason", "confidence", "execution_status", "executed_at", "execution_error", "updated_at").
		Updates(suggestion)
	if result.Error != nil {
		return fmt.Errorf("update suggestion: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("suggestion not found")
	}
	return nil
}

// ClaimPending atomically moves one pending suggestion into accepted state so
// concurrent approval requests cannot execute the same CRM mutation twice.
func (r *CRMSuggestionRepository) ClaimPending(ctx context.Context, workspaceID string, suggestion *model.CRMSuggestion) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.CRMSuggestion{}).
		Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, suggestion.ID, model.CRMSuggestionStatusPending).
		Updates(map[string]interface{}{"status": model.CRMSuggestionStatusAccepted, "context": suggestion.Context, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if result.Error != nil {
		return false, fmt.Errorf("claim pending suggestion: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// Delete removes a suggestion.
func (r *CRMSuggestionRepository) Delete(ctx context.Context, workspaceID, id string) error {
	result := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.CRMSuggestion{})
	if result.Error != nil {
		return fmt.Errorf("delete suggestion: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("suggestion not found")
	}
	return nil
}
