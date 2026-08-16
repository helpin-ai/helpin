package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GetByCalendarEventID returns the capture record for one calendar event.
func (r *CRMMeetingRepository) GetByCalendarEventID(ctx context.Context, workspaceID, calendarEventID string) (*model.CRMMeeting, error) {
	var meeting model.CRMMeeting
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND calendar_event_id = ?", workspaceID, calendarEventID).
		First(&meeting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get CRM meeting by calendar event: %w", err)
	}
	return &meeting, nil
}

// ListByCalendarEventIDs returns meetings keyed by calendar event id.
func (r *CRMMeetingRepository) ListByCalendarEventIDs(ctx context.Context, workspaceID string, calendarEventIDs []string) (map[string]model.CRMMeeting, error) {
	result := make(map[string]model.CRMMeeting, len(calendarEventIDs))
	if len(calendarEventIDs) == 0 {
		return result, nil
	}
	var meetings []model.CRMMeeting
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND calendar_event_id IN ?", workspaceID, calendarEventIDs).
		Find(&meetings).Error; err != nil {
		return nil, fmt.Errorf("list CRM meetings by calendar events: %w", err)
	}
	for _, meeting := range meetings {
		if meeting.CalendarEventID != nil && *meeting.CalendarEventID != "" {
			result[*meeting.CalendarEventID] = meeting
		}
	}
	return result, nil
}
