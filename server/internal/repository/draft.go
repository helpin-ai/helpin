package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// RewardDraftRepository handles database operations for goal_drafts.
type RewardDraftRepository struct {
	db *gorm.DB
}

// NewRewardDraftRepository creates a new RewardDraftRepository.
func NewRewardDraftRepository(db *gorm.DB) *RewardDraftRepository {
	return &RewardDraftRepository{db: db}
}

// Create inserts a new goal draft.
func (r *RewardDraftRepository) Create(ctx context.Context, workspaceID, quarterID string, createdBy string, draftData json.RawMessage) (*model.RewardGoalDraft, error) {
	d := &model.RewardGoalDraft{
		WorkspaceID: workspaceID,
		QuarterID:   quarterID,
		CreatedBy:   &createdBy,
		DraftData:   draftData,
	}
	if err := r.db.WithContext(ctx).Create(d).Error; err != nil {
		return nil, fmt.Errorf("create draft: %w", err)
	}
	return d, nil
}

// List returns all goal drafts for a workspace/quarter.
func (r *RewardDraftRepository) List(ctx context.Context, workspaceID, quarterID string) ([]model.RewardGoalDraft, error) {
	var drafts []model.RewardGoalDraft
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND quarter_id = ?", workspaceID, quarterID).
		Order("created_at DESC").
		Find(&drafts).Error
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	return drafts, nil
}

// Get returns a single goal draft by ID.
func (r *RewardDraftRepository) Get(ctx context.Context, id string) (*model.RewardGoalDraft, error) {
	d := &model.RewardGoalDraft{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get draft: %w", err)
	}
	return d, nil
}

// Update modifies a draft's data and/or status.
func (r *RewardDraftRepository) Update(ctx context.Context, id string, draftData json.RawMessage, status *string) (*model.RewardGoalDraft, error) {
	updates := map[string]interface{}{}
	if draftData != nil {
		updates["draft_data"] = draftData
	}
	if status != nil {
		updates["status"] = *status
	}

	if err := r.db.WithContext(ctx).Model(&model.RewardGoalDraft{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update draft: %w", err)
	}

	d := &model.RewardGoalDraft{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(d).Error; err != nil {
		return nil, fmt.Errorf("update draft: %w", err)
	}
	return d, nil
}

// Delete removes a draft by ID.
func (r *RewardDraftRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.RewardGoalDraft{}).Error; err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	return nil
}
