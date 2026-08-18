package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMActivityRepository handles DB operations for CRM activities.
type CRMActivityRepository struct {
	db *gorm.DB
}

// NewCRMActivityRepository creates a new CRMActivityRepository.
func NewCRMActivityRepository(db *gorm.DB) *CRMActivityRepository {
	return &CRMActivityRepository{db: db}
}

// WithTx returns a repository backed by tx.
func (r *CRMActivityRepository) WithTx(tx *gorm.DB) *CRMActivityRepository {
	return &CRMActivityRepository{db: tx}
}

// List returns activities in a workspace with optional filters.
func (r *CRMActivityRepository) List(ctx context.Context, workspaceID string, filters model.CRMActivityListFilters, pagination model.PMPagination) ([]model.CRMActivity, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMActivity{}).Where("workspace_id = ?", workspaceID)

	if filters.ActivityType != nil && *filters.ActivityType != "" {
		query = query.Where("activity_type = ?", *filters.ActivityType)
	}
	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where("contact_id = ?", *filters.ContactID)
	}
	if filters.CompanyID != nil && *filters.CompanyID != "" {
		query = query.Where("company_id = ?", *filters.CompanyID)
	}
	if filters.DealID != nil && *filters.DealID != "" {
		query = query.Where("deal_id = ?", *filters.DealID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count activities: %w", err)
	}

	var activities []model.CRMActivity
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("occurred_at DESC").Offset(offset).Limit(pagination.PerPage).Find(&activities).Error; err != nil {
		return nil, 0, fmt.Errorf("list activities: %w", err)
	}
	return activities, total, nil
}

// GetByID returns an activity by ID.
func (r *CRMActivityRepository) GetByID(ctx context.Context, id string) (*model.CRMActivity, error) {
	var activity model.CRMActivity
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&activity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get activity: %w", err)
	}
	return &activity, nil
}

// Create inserts an activity.
func (r *CRMActivityRepository) Create(ctx context.Context, activity *model.CRMActivity) error {
	if err := r.db.WithContext(ctx).Create(activity).Error; err != nil {
		return fmt.Errorf("create activity: %w", err)
	}
	return nil
}

// Update updates an activity.
func (r *CRMActivityRepository) Update(ctx context.Context, activity *model.CRMActivity) error {
	if err := r.db.WithContext(ctx).Save(activity).Error; err != nil {
		return fmt.Errorf("update activity: %w", err)
	}
	return nil
}

// Delete removes an activity.
func (r *CRMActivityRepository) Delete(ctx context.Context, id string) error {
	activity, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if activity != nil && activity.Metadata != nil && activity.Metadata["immutable"] == true {
		return fmt.Errorf("delete activity: system activity is immutable")
	}
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMActivity{}).Error; err != nil {
		return fmt.Errorf("delete activity: %w", err)
	}
	return nil
}
