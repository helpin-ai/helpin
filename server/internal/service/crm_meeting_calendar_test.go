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
		BotName:                 "Helpin Notetaker",
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

func TestCalendarMeetingCandidateRequiresSupportedMeetingLink(t *testing.T) {
	event := model.CRMCalendarEvent{Status: model.CRMCalendarEventStatusConfirmed}
	candidate := calendarMeetingCandidate(event, true)
	if candidate.Eligible || candidate.IneligibilityReason == nil || *candidate.IneligibilityReason != "No supported meeting link" {
		t.Fatalf("expected missing-link ineligibility, got %#v", candidate)
	}
}
