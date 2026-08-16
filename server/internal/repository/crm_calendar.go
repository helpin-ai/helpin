package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// UpsertSyncedEvent inserts or refreshes a provider-owned event idempotently.
// A PostgreSQL advisory lock closes the race without requiring a destructive
// uniqueness migration against existing customer data.
func (r *CRMCalendarRepository) UpsertSyncedEvent(ctx context.Context, event *model.CRMCalendarEvent) error {
	if event == nil || event.ExternalEventID == nil || strings.TrimSpace(*event.ExternalEventID) == "" {
		return fmt.Errorf("external_event_id is required for calendar sync")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			lockKey := event.EmailAccountID + ":" + strings.TrimSpace(*event.ExternalEventID)
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey).Error; err != nil {
				return fmt.Errorf("lock calendar event upsert: %w", err)
			}
		}
		var existing model.CRMCalendarEvent
		err := tx.Where("email_account_id = ? AND external_event_id = ?", event.EmailAccountID, *event.ExternalEventID).
			Order("created_at ASC, id ASC").First(&existing).Error
		switch {
		case err == nil:
			event.ID = existing.ID
			event.CreatedAt = existing.CreatedAt
			if event.DealID == nil {
				event.DealID = existing.DealID
			}
			if err := tx.Save(event).Error; err != nil {
				return fmt.Errorf("update synced calendar event: %w", err)
			}
			return nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return fmt.Errorf("lookup synced calendar event: %w", err)
		default:
			if err := tx.Create(event).Error; err != nil {
				return fmt.Errorf("create synced calendar event: %w", err)
			}
			return nil
		}
	})
}

// GetByID returns a workspace-scoped calendar event.
func (r *CRMCalendarRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.CRMCalendarEvent, error) {
	var event model.CRMCalendarEvent
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get calendar event: %w", err)
	}
	return &event, nil
}

// GetByExternalID resolves one provider event within an email account.
func (r *CRMCalendarRepository) GetByExternalID(ctx context.Context, accountID, externalEventID string) (*model.CRMCalendarEvent, error) {
	var event model.CRMCalendarEvent
	err := r.db.WithContext(ctx).
		Where("email_account_id = ? AND external_event_id = ?", accountID, externalEventID).
		Order("created_at ASC, id ASC").
		First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get calendar event by external id: %w", err)
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
	if filters.Status != nil && *filters.Status != "" {
		query = query.Where("status = ?", *filters.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count calendar events: %w", err)
	}

	var events []model.CRMCalendarEvent
	page, perPage := normalizedPagination(pagination)
	order := "start_time DESC"
	if filters.Ascending {
		order = "start_time ASC"
	}
	if err := query.Order(order).Offset((page - 1) * perPage).Limit(perPage).Find(&events).Error; err != nil {
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

// Delete removes a workspace-scoped calendar event.
func (r *CRMCalendarRepository) Delete(ctx context.Context, workspaceID, id string) error {
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).Delete(&model.CRMCalendarEvent{}).Error; err != nil {
		return fmt.Errorf("delete calendar event: %w", err)
	}
	return nil
}
