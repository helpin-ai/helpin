package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupCalendarMeetingTestDB(t *testing.T) (*CRMMeetingService, *repository.CRMMeetingRepository) {
	t.Helper()
	db := setupMeetingLifecycleDB(t)
	statements := []string{
		`CREATE TABLE crm_calendar_events (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			external_event_id TEXT,
			recurring_series_id TEXT,
			auto_join_override BOOLEAN,
			title TEXT NOT NULL,
			description TEXT,
			start_time DATETIME NOT NULL,
			end_time DATETIME NOT NULL,
			location TEXT,
			meeting_url TEXT,
			organizer_email TEXT,
			status TEXT NOT NULL DEFAULT 'confirmed',
			visibility TEXT NOT NULL DEFAULT 'default',
			all_day BOOLEAN NOT NULL DEFAULT 0,
			attendees BLOB NOT NULL DEFAULT x'5b5d',
			contact_ids BLOB NOT NULL DEFAULT x'5b5d',
			deal_id TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE crm_associations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			from_object_type TEXT NOT NULL,
			from_object_id TEXT NOT NULL,
			to_object_type TEXT NOT NULL,
			to_object_id TEXT NOT NULL,
			association_label TEXT,
			created_at DATETIME
		)`,
		`CREATE TABLE crm_calendar_series_preferences (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			email_account_id TEXT NOT NULL,
			series_external_id TEXT NOT NULL,
			auto_join BOOLEAN NOT NULL DEFAULT 0,
			created_by TEXT,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create calendar meeting test table: %v", err)
		}
	}
	meetingRepo := repository.NewCRMMeetingRepository(db)
	calendarRepo := repository.NewCRMCalendarRepository(db)
	associationRepo := repository.NewCRMAssociationRepository(db)
	service := NewCRMMeetingService(meetingRepo, associationRepo, nil).
		SetCalendarIntegration(calendarRepo, nil).
		SetCaptureScheduler(&fakeMeetingCaptureScheduler{})
	return service, meetingRepo
}

func TestCalendarMeetingCaptureCreatesAssociationsAndCanBeDisabled(t *testing.T) {
	service, meetingRepo := setupCalendarMeetingTestDB(t)
	ctx := context.Background()
	workspaceID := "workspace-calendar"
	if err := meetingRepo.UpsertSettings(ctx, &model.CRMMeetingSettings{
		WorkspaceID:             workspaceID,
		Enabled:                 true,
		BotName:                 "Helpin.ai Notetaker",
		AutoJoinMode:            "manual",
		DefaultProvider:         model.CRMMeetingProviderRecall,
		DefaultVisibility:       model.CRMMeetingVisibilityWorkspace,
		AudioRetentionDays:      30,
		TranscriptRetentionDays: 365,
	}); err != nil {
		t.Fatalf("create settings: %v", err)
	}

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	end := start.Add(45 * time.Minute)
	meetingURL := "https://meet.google.com/abc-defg-hij"
	event := &model.CRMCalendarEvent{
		ID:             "calendar-event-1",
		WorkspaceID:    workspaceID,
		EmailAccountID: "email-account-1",
		Title:          "Customer check-in",
		StartTime:      start,
		EndTime:        end,
		MeetingURL:     &meetingURL,
		Status:         model.CRMCalendarEventStatusConfirmed,
		Visibility:     "default",
		Attendees:      model.CRMCalendarAttendees{},
		ContactIDs:     model.CRMStringList{"contact-1", "contact-2"},
	}
	if err := service.calendarRepo.Create(ctx, event); err != nil {
		t.Fatalf("create calendar event: %v", err)
	}

	candidate, err := service.UpdateCalendarMeetingCapture(ctx, workspaceID, event.ID, "member-1", model.UpdateCRMCalendarMeetingCaptureRequest{Enabled: true})
	if err != nil {
		t.Fatalf("enable capture: %v", err)
	}
	if candidate.Meeting == nil || candidate.Meeting.CalendarEventID == nil || *candidate.Meeting.CalendarEventID != event.ID {
		t.Fatalf("expected a meeting linked to %q, got %#v", event.ID, candidate.Meeting)
	}
	scheduler := service.captureScheduler.(*fakeMeetingCaptureScheduler)
	if scheduler.scheduledMeetingID != candidate.Meeting.ID || !scheduler.scheduledStart.Equal(start) {
		t.Fatalf("automatic join was not scheduled: %#v", scheduler)
	}
	for _, contactID := range []string{"contact-1", "contact-2"} {
		meetings, total, listErr := meetingRepo.List(ctx, workspaceID, model.CRMMeetingListFilters{ContactID: &contactID}, model.PMPagination{Page: 1, PerPage: 10})
		if listErr != nil {
			t.Fatalf("list meetings for %s: %v", contactID, listErr)
		}
		if total != 1 || len(meetings) != 1 || meetings[0].ID != candidate.Meeting.ID {
			t.Fatalf("expected linked meeting for %s, got total=%d meetings=%#v", contactID, total, meetings)
		}
	}

	disabled, err := service.UpdateCalendarMeetingCapture(ctx, workspaceID, event.ID, "member-1", model.UpdateCRMCalendarMeetingCaptureRequest{Enabled: false})
	if err != nil {
		t.Fatalf("disable capture: %v", err)
	}
	if disabled.Meeting != nil {
		t.Fatalf("expected disabled candidate without a meeting, got %#v", disabled.Meeting)
	}
	if scheduler.cancelledMeetingID != candidate.Meeting.ID {
		t.Fatalf("automatic join cancellation = %q, want %q", scheduler.cancelledMeetingID, candidate.Meeting.ID)
	}
	stored, err := meetingRepo.GetByCalendarEventID(ctx, workspaceID, event.ID)
	if err != nil {
		t.Fatalf("get disabled meeting: %v", err)
	}
	if stored != nil {
		t.Fatalf("expected scheduled meeting to be removed, got %#v", stored)
	}
}

func TestCalendarMeetingSeriesCaptureSchedulesEveryFutureOccurrence(t *testing.T) {
	service, meetingRepo := setupCalendarMeetingTestDB(t)
	ctx := context.Background()
	workspaceID := "workspace-series"
	if err := meetingRepo.UpsertSettings(ctx, &model.CRMMeetingSettings{
		WorkspaceID:             workspaceID,
		Enabled:                 true,
		BotName:                 "Helpin.ai Notetaker",
		AutoJoinMode:            "manual",
		DefaultProvider:         model.CRMMeetingProviderRecall,
		DefaultVisibility:       model.CRMMeetingVisibilityWorkspace,
		AudioRetentionDays:      30,
		TranscriptRetentionDays: 365,
	}); err != nil {
		t.Fatalf("create settings: %v", err)
	}

	seriesID := "google-series-1"
	meetingURL := "https://meet.google.com/abc-defg-hij"
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	eventIDs := []string{"series-event-1", "series-event-2"}
	for index, eventID := range eventIDs {
		eventStart := start.Add(time.Duration(index) * 7 * 24 * time.Hour)
		override := false
		event := &model.CRMCalendarEvent{
			ID:                eventID,
			WorkspaceID:       workspaceID,
			EmailAccountID:    "email-account-series",
			RecurringSeriesID: &seriesID,
			AutoJoinOverride:  &override,
			Title:             "Weekly customer sync",
			StartTime:         eventStart,
			EndTime:           eventStart.Add(30 * time.Minute),
			MeetingURL:        &meetingURL,
			Status:            model.CRMCalendarEventStatusConfirmed,
			Visibility:        "default",
			Attendees:         model.CRMCalendarAttendees{},
			ContactIDs:        model.CRMStringList{},
		}
		if err := service.calendarRepo.Create(ctx, event); err != nil {
			t.Fatalf("create series event: %v", err)
		}
	}

	candidates, err := service.UpdateCalendarSeriesCapture(ctx, workspaceID, "member-1", model.UpdateCRMCalendarSeriesCaptureRequest{
		EmailAccountID:   "email-account-series",
		SeriesExternalID: seriesID,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("enable recurring series: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %d, want 2", len(candidates))
	}
	meetings, err := meetingRepo.ListByCalendarEventIDs(ctx, workspaceID, eventIDs)
	if err != nil {
		t.Fatalf("list series meetings: %v", err)
	}
	if len(meetings) != 2 {
		t.Fatalf("scheduled meetings = %d, want 2", len(meetings))
	}
	for _, candidate := range candidates {
		if !candidate.EffectiveAutoJoin || candidate.AutoJoinSource != "series" || candidate.Event.AutoJoinOverride != nil {
			t.Fatalf("unexpected series candidate: %#v", candidate)
		}
	}
}

func TestCalendarMeetingCandidateRequiresSupportedMeetingLink(t *testing.T) {
	event := model.CRMCalendarEvent{Status: model.CRMCalendarEventStatusConfirmed}
	settings := &model.CRMMeetingSettings{Enabled: true, AutoJoinMode: "manual"}
	candidate := calendarMeetingCandidate(event, settings, nil, nil)
	if candidate.Eligible || candidate.IneligibilityReason == nil || *candidate.IneligibilityReason != "No supported meeting link" {
		t.Fatalf("expected missing-link ineligibility, got %#v", candidate)
	}
}

func TestCalendarMeetingCandidatePolicyPriority(t *testing.T) {
	meetingURL := "https://meet.google.com/abc-defg-hij"
	event := model.CRMCalendarEvent{
		Status:     model.CRMCalendarEventStatusConfirmed,
		MeetingURL: &meetingURL,
	}
	settings := &model.CRMMeetingSettings{Enabled: true, AutoJoinMode: "all"}
	preference := &model.CRMCalendarSeriesPreference{AutoJoin: false}

	candidate := calendarMeetingCandidate(event, settings, preference, nil)
	if candidate.EffectiveAutoJoin || candidate.AutoJoinSource != "series" {
		t.Fatalf("series preference was not applied: %#v", candidate)
	}

	override := true
	event.AutoJoinOverride = &override
	candidate = calendarMeetingCandidate(event, settings, preference, nil)
	if !candidate.EffectiveAutoJoin || candidate.AutoJoinSource != "occurrence" {
		t.Fatalf("occurrence override did not win: %#v", candidate)
	}
}
