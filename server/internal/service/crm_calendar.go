package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMCalendarService contains CRM calendar business logic.
type CRMCalendarService struct {
	calendarRepo *repository.CRMCalendarRepository
}

// NewCRMCalendarService creates a new CRMCalendarService.
func NewCRMCalendarService(calendarRepo *repository.CRMCalendarRepository) *CRMCalendarService {
	return &CRMCalendarService{calendarRepo: calendarRepo}
}

// List returns calendar events with filters and pagination.
func (s *CRMCalendarService) List(ctx context.Context, workspaceID string, filters model.CRMCalendarEventListFilters, pagination model.PMPagination) ([]model.CRMCalendarEvent, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.calendarRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a calendar event by ID.
func (s *CRMCalendarService) GetByID(ctx context.Context, id string) (*model.CRMCalendarEvent, error) {
	event, err := s.calendarRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fmt.Errorf("calendar event not found")
	}
	return event, nil
}

// Create creates a new calendar event.
func (s *CRMCalendarService) Create(ctx context.Context, req model.CreateCRMCalendarEventRequest) (*model.CRMCalendarEvent, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("workspace_id and title are required")
	}

	event := &model.CRMCalendarEvent{
		WorkspaceID:    req.WorkspaceID,
		EmailAccountID: req.EmailAccountID,
		Title:          strings.TrimSpace(req.Title),
		Description:    req.Description,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		Location:       req.Location,
		Attendees:      model.JSONB(req.Attendees),
		ContactIDs:     model.JSONB(req.ContactIDs),
		DealID:         req.DealID,
	}

	if err := s.calendarRepo.Create(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

// Update updates a calendar event.
func (s *CRMCalendarService) Update(ctx context.Context, id string, req model.UpdateCRMCalendarEventRequest) (*model.CRMCalendarEvent, error) {
	event, err := s.calendarRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fmt.Errorf("calendar event not found")
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, fmt.Errorf("title cannot be empty")
		}
		event.Title = title
	}
	if req.Description != nil {
		event.Description = req.Description
	}
	if req.StartTime != nil {
		event.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		event.EndTime = *req.EndTime
	}
	if req.Location != nil {
		event.Location = req.Location
	}
	if req.Attendees != nil {
		event.Attendees = model.JSONB(req.Attendees)
	}
	if req.ContactIDs != nil {
		event.ContactIDs = model.JSONB(req.ContactIDs)
	}
	if req.DealID != nil {
		event.DealID = req.DealID
	}

	if err := s.calendarRepo.Update(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

// Delete removes a calendar event.
func (s *CRMCalendarService) Delete(ctx context.Context, id string) error {
	event, err := s.calendarRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if event == nil {
		return fmt.Errorf("calendar event not found")
	}
	return s.calendarRepo.Delete(ctx, id)
}
