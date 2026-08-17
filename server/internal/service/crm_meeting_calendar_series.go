package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// UpdateCalendarSeriesCapture applies one durable preference to all known
// future occurrences. New occurrences inherit it during calendar sync.
func (s *CRMMeetingService) UpdateCalendarSeriesCapture(
	ctx context.Context,
	workspaceID, actorID string,
	req model.UpdateCRMCalendarSeriesCaptureRequest,
) ([]model.CRMCalendarMeetingCandidate, error) {
	if s == nil || s.repo == nil || s.calendarRepo == nil {
		return nil, fmt.Errorf("calendar meeting integration is unavailable")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	req.EmailAccountID = strings.TrimSpace(req.EmailAccountID)
	req.SeriesExternalID = strings.TrimSpace(req.SeriesExternalID)
	if workspaceID == "" || req.EmailAccountID == "" || req.SeriesExternalID == "" {
		return nil, fmt.Errorf("workspace_id, email_account_id, and series_external_id are required")
	}

	var candidates []model.CRMCalendarMeetingCandidate
	lockKey := "calendar-series:" + req.EmailAccountID + ":" + req.SeriesExternalID
	err := s.repo.WithCaptureLaunchLock(ctx, workspaceID, lockKey, func(lockedRepo *repository.CRMMeetingRepository) error {
		lockedService := *s
		lockedService.repo = lockedRepo
		preference := &model.CRMCalendarSeriesPreference{
			WorkspaceID:      workspaceID,
			EmailAccountID:   req.EmailAccountID,
			SeriesExternalID: req.SeriesExternalID,
			AutoJoin:         req.Enabled,
			CreatedBy:        trimStringPtr(&actorID),
		}
		if err := lockedService.calendarRepo.UpsertSeriesPreference(ctx, preference); err != nil {
			return err
		}
		events, err := lockedService.calendarRepo.ListRecurringSeriesEvents(
			ctx,
			workspaceID,
			req.EmailAccountID,
			req.SeriesExternalID,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		candidates = make([]model.CRMCalendarMeetingCandidate, 0, len(events))
		for index := range events {
			event := &events[index]
			// A series-level choice intentionally resets earlier occurrence
			// exceptions; users can add new exceptions after this change.
			event.AutoJoinOverride = nil
			if err := lockedService.calendarRepo.Update(ctx, event); err != nil {
				return err
			}
			candidate, err := lockedService.updateCalendarMeetingCaptureLocked(
				ctx,
				workspaceID,
				event.ID,
				actorID,
				model.UpdateCRMCalendarMeetingCaptureRequest{Enabled: req.Enabled},
			)
			if err != nil {
				return err
			}
			seriesAutoJoin := req.Enabled
			candidate.SeriesAutoJoin = &seriesAutoJoin
			candidate.AutoJoinSource = "series"
			candidate.EffectiveAutoJoin = req.Enabled && candidate.Eligible
			candidates = append(candidates, *candidate)
		}
		return nil
	})
	return candidates, err
}

// ReconcileCalendarMeetingPolicy materializes the effective occurrence,
// recurring-series, or workspace policy after a calendar occurrence is synced.
func (s *CRMMeetingService) ReconcileCalendarMeetingPolicy(
	ctx context.Context,
	workspaceID, calendarEventID, actorID string,
) error {
	if s == nil || s.repo == nil || s.calendarRepo == nil {
		return nil
	}
	return s.repo.WithCaptureLaunchLock(ctx, workspaceID, "calendar-policy:"+calendarEventID, func(lockedRepo *repository.CRMMeetingRepository) error {
		lockedService := *s
		lockedService.repo = lockedRepo
		event, err := lockedService.calendarRepo.GetByID(ctx, workspaceID, calendarEventID)
		if err != nil || event == nil {
			return err
		}
		settings, err := lockedService.repo.GetSettings(ctx, workspaceID)
		if err != nil {
			return err
		}
		var preference *model.CRMCalendarSeriesPreference
		if event.RecurringSeriesID != nil {
			preference, err = lockedService.calendarRepo.GetSeriesPreference(ctx, workspaceID, event.EmailAccountID, *event.RecurringSeriesID)
			if err != nil {
				return err
			}
		}
		existing, err := lockedService.repo.GetByCalendarEventID(ctx, workspaceID, event.ID)
		if err != nil {
			return err
		}
		policyControlled := event.AutoJoinOverride != nil || preference != nil || settings.AutoJoinMode != "manual" || existing != nil
		if !policyControlled {
			return nil
		}
		candidate := calendarMeetingCandidate(*event, settings, preference, existing)
		_, err = lockedService.updateCalendarMeetingCaptureLocked(
			ctx,
			workspaceID,
			event.ID,
			actorID,
			model.UpdateCRMCalendarMeetingCaptureRequest{Enabled: candidate.EffectiveAutoJoin},
		)
		return err
	})
}
