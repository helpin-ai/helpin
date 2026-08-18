package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// UpdateCalendarMeetingCapture serializes configuration for one calendar
// event so retries and simultaneous clients remain idempotent across replicas.
func (s *CRMMeetingService) UpdateCalendarMeetingCapture(
	ctx context.Context,
	workspaceID, calendarEventID, actorID string,
	req model.UpdateCRMCalendarMeetingCaptureRequest,
) (*model.CRMCalendarMeetingCandidate, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("meeting integration is unavailable")
	}
	var candidate *model.CRMCalendarMeetingCandidate
	err := s.repo.WithCaptureLaunchLock(ctx, workspaceID, "calendar-event:"+calendarEventID, func(lockedRepo *repository.CRMMeetingRepository) error {
		lockedService := *s
		lockedService.repo = lockedRepo
		var innerErr error
		candidate, innerErr = lockedService.updateCalendarMeetingCaptureLocked(ctx, workspaceID, calendarEventID, actorID, req)
		if innerErr != nil {
			return innerErr
		}
		event, eventErr := lockedService.calendarRepo.GetByID(ctx, workspaceID, calendarEventID)
		if eventErr != nil {
			return eventErr
		}
		if event == nil {
			return fmt.Errorf("calendar event not found")
		}
		enabled := req.Enabled
		event.AutoJoinOverride = &enabled
		if eventErr := lockedService.calendarRepo.Update(ctx, event); eventErr != nil {
			return eventErr
		}
		candidate.Event = *event
		candidate.EffectiveAutoJoin = req.Enabled && candidate.Eligible
		candidate.AutoJoinSource = "occurrence"
		return innerErr
	})
	return candidate, err
}
