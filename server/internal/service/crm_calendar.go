package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmsignal"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMCalendarService contains CRM calendar business logic.
type CRMCalendarService struct {
	calendarRepo   *repository.CRMCalendarRepository
	summaryRefresh CompanySummaryRefreshRequester
	signalStarter  interface {
		StartSignalDetection(ctx context.Context, sourceKey string, payloads []model.SignalSourcePayload) error
	}
	signalRepo *repository.CRMSignalRepository
}

// SetCompanySummaryRefresh enables linked-account invalidation after calendar changes.
func (s *CRMCalendarService) SetCompanySummaryRefresh(refresh CompanySummaryRefreshRequester) *CRMCalendarService {
	s.summaryRefresh = refresh
	return s
}

// SetSignalDetection enables versioned buyer-signal analysis for calendar
// events and stale-signal cleanup when an event is removed.
func (s *CRMCalendarService) SetSignalDetection(starter interface {
	StartSignalDetection(ctx context.Context, sourceKey string, payloads []model.SignalSourcePayload) error
}, signalRepo *repository.CRMSignalRepository) *CRMCalendarService {
	s.signalStarter, s.signalRepo = starter, signalRepo
	return s
}

// NewCRMCalendarService creates a new CRMCalendarService.
func NewCRMCalendarService(calendarRepo *repository.CRMCalendarRepository) *CRMCalendarService {
	return &CRMCalendarService{calendarRepo: calendarRepo}
}

// List returns calendar events with filters and pagination.
func (s *CRMCalendarService) List(ctx context.Context, workspaceID string, filters model.CRMCalendarEventListFilters, pagination model.PMPagination) ([]model.CRMCalendarEvent, int64, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.calendarRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a workspace-scoped calendar event by ID.
func (s *CRMCalendarService) GetByID(ctx context.Context, workspaceID, id string) (*model.CRMCalendarEvent, error) {
	event, err := s.calendarRepo.GetByID(ctx, workspaceID, id)
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
	if strings.TrimSpace(req.WorkspaceID) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.EmailAccountID) == "" {
		return nil, fmt.Errorf("workspace_id, email_account_id, and title are required")
	}
	if err := validateCalendarEventWindow(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	event := &model.CRMCalendarEvent{
		WorkspaceID:    req.WorkspaceID,
		EmailAccountID: req.EmailAccountID,
		Title:          strings.TrimSpace(req.Title),
		Description:    trimStringPtr(req.Description),
		StartTime:      req.StartTime.UTC(),
		EndTime:        req.EndTime.UTC(),
		Location:       trimStringPtr(req.Location),
		MeetingURL:     trimStringPtr(req.MeetingURL),
		Status:         model.CRMCalendarEventStatusConfirmed,
		Visibility:     "default",
		Attendees:      req.Attendees,
		ContactIDs:     req.ContactIDs,
		DealID:         trimStringPtr(req.DealID),
	}

	if err := s.calendarRepo.Create(ctx, event); err != nil {
		return nil, err
	}
	s.requestCompanySummaryRefresh(ctx, event)
	s.enqueueSignalDetection(ctx, event)
	return event, nil
}

// Update updates a workspace-scoped calendar event.
func (s *CRMCalendarService) Update(ctx context.Context, workspaceID, id string, req model.UpdateCRMCalendarEventRequest) (*model.CRMCalendarEvent, error) {
	event, err := s.calendarRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fmt.Errorf("calendar event not found")
	}
	s.requestCompanySummaryRefresh(ctx, event)

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, fmt.Errorf("title cannot be empty")
		}
		event.Title = title
	}
	if req.Description != nil {
		event.Description = trimStringPtr(req.Description)
	}
	if req.StartTime != nil {
		event.StartTime = req.StartTime.UTC()
	}
	if req.EndTime != nil {
		event.EndTime = req.EndTime.UTC()
	}
	if err := validateCalendarEventWindow(event.StartTime, event.EndTime); err != nil {
		return nil, err
	}
	if req.Location != nil {
		event.Location = trimStringPtr(req.Location)
	}
	if req.MeetingURL != nil {
		event.MeetingURL = trimStringPtr(req.MeetingURL)
	}
	if req.Attendees != nil {
		event.Attendees = req.Attendees
	}
	if req.ContactIDs != nil {
		event.ContactIDs = req.ContactIDs
	}
	if req.DealID != nil {
		event.DealID = trimStringPtr(req.DealID)
	}

	if err := s.calendarRepo.Update(ctx, event); err != nil {
		return nil, err
	}
	s.requestCompanySummaryRefresh(ctx, event)
	s.enqueueSignalDetection(ctx, event)
	return event, nil
}

// Delete removes a workspace-scoped calendar event.
func (s *CRMCalendarService) Delete(ctx context.Context, workspaceID, id string) error {
	event, err := s.calendarRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if event == nil {
		return fmt.Errorf("calendar event not found")
	}
	if err := s.calendarRepo.Delete(ctx, workspaceID, id); err != nil {
		return err
	}
	s.requestCompanySummaryRefresh(ctx, event)
	s.reconcileDeletedSignals(ctx, event)
	return nil
}

func (s *CRMCalendarService) enqueueSignalDetection(ctx context.Context, event *model.CRMCalendarEvent) {
	if s == nil || s.signalStarter == nil {
		return
	}
	payload, eligible := crmsignal.CalendarPayload(event)
	if !eligible {
		return
	}
	if err := s.signalStarter.StartSignalDetection(ctx, crmsignal.CalendarWorkflowKey(*payload), []model.SignalSourcePayload{*payload}); err != nil {
		slog.WarnContext(ctx, "calendar buyer signal enqueue failed", "error", err, "calendar_event_id", event.ID)
	}
}

func (s *CRMCalendarService) reconcileDeletedSignals(ctx context.Context, event *model.CRMCalendarEvent) {
	if s == nil || s.signalRepo == nil || event == nil {
		return
	}
	if err := s.signalRepo.ReconcileAutomatedSignalsForSource(ctx, event.WorkspaceID, model.CRMSignalSourceMeeting, event.ID, nil); err != nil {
		slog.WarnContext(ctx, "deleted calendar event retained stale buyer signals", "error", err, "calendar_event_id", event.ID)
	}
}

func (s *CRMCalendarService) requestCompanySummaryRefresh(ctx context.Context, event *model.CRMCalendarEvent) {
	if s == nil || s.summaryRefresh == nil || event == nil {
		return
	}
	if event.DealID != nil && *event.DealID != "" {
		if err := s.summaryRefresh.RequestCompanyRefreshForObject(ctx, event.WorkspaceID, model.CRMObjectDeal, *event.DealID); err != nil {
			slog.WarnContext(ctx, "failed to request company summary refresh from calendar deal", "error", err, "calendar_event_id", event.ID, "deal_id", *event.DealID)
		}
	}
	for _, contactID := range event.ContactIDs {
		if err := s.summaryRefresh.RequestCompanyRefreshForObject(ctx, event.WorkspaceID, model.CRMObjectContact, contactID); err != nil {
			slog.WarnContext(ctx, "failed to request company summary refresh from calendar contact", "error", err, "calendar_event_id", event.ID, "contact_id", contactID)
		}
	}
}

func validateCalendarEventWindow(start, end time.Time) error {
	if start.IsZero() || end.IsZero() {
		return fmt.Errorf("start_time and end_time are required")
	}
	if !end.After(start) {
		return fmt.Errorf("end_time must be after start_time")
	}
	return nil
}
