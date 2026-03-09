package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMCalendarRepository handles DB operations for CRM calendar events.
type CRMCalendarRepository struct {
	db *gorm.DB
}

// NewCRMCalendarRepository creates a new CRMCalendarRepository.
func NewCRMCalendarRepository(db *gorm.DB) *CRMCalendarRepository {
	return &CRMCalendarRepository{db: db}
}

// Create inserts a calendar event.
func (r *CRMCalendarRepository) Create(ctx context.Context, event *model.CRMCalendarEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create calendar event: %w", err)
	}
	return nil
}

// GetByID returns a calendar event by ID.
func (r *CRMCalendarRepository) GetByID(ctx context.Context, id string) (*model.CRMCalendarEvent, error) {
	var event model.CRMCalendarEvent
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get calendar event: %w", err)
	}
	return &event, nil
}

// List returns calendar events with optional filters and pagination.
func (r *CRMCalendarRepository) List(ctx context.Context, workspaceID string, filters model.CRMCalendarEventListFilters, pagination model.PMPagination) ([]model.CRMCalendarEvent, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMCalendarEvent{}).Where("workspace_id = ?", workspaceID)

	if filters.EmailAccountID != nil && *filters.EmailAccountID != "" {
		query = query.Where("email_account_id = ?", *filters.EmailAccountID)
	}
	if filters.ContactID != nil && *filters.ContactID != "" {
		query = query.Where("contact_ids @> ?::jsonb", fmt.Sprintf(`["%s"]`, *filters.ContactID))
	}
	if filters.DealID != nil && *filters.DealID != "" {
		query = query.Where("deal_id = ?", *filters.DealID)
	}
	if filters.StartAfter != nil {
		query = query.Where("start_time >= ?", *filters.StartAfter)
	}
	if filters.StartBefore != nil {
		query = query.Where("start_time <= ?", *filters.StartBefore)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count calendar events: %w", err)
	}

	var events []model.CRMCalendarEvent
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("start_time DESC").Offset(offset).Limit(pagination.PerPage).Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("list calendar events: %w", err)
	}
	return events, total, nil
}

// Update updates a calendar event.
func (r *CRMCalendarRepository) Update(ctx context.Context, event *model.CRMCalendarEvent) error {
	if err := r.db.WithContext(ctx).Save(event).Error; err != nil {
		return fmt.Errorf("update calendar event: %w", err)
	}
	return nil
}

// Delete removes a calendar event.
func (r *CRMCalendarRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMCalendarEvent{}).Error; err != nil {
		return fmt.Errorf("delete calendar event: %w", err)
	}
	return nil
}
