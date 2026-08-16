package temporalapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func setupCalendarMeetingReconcileTest(t *testing.T) (*EmailSyncActivities, *repository.CRMMeetingRepository) {
	t.Helper()
	dsn := fmt.Sprintf("file:calendar-meeting-reconcile-%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE crm_meetings (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		calendar_event_id TEXT,
		activity_id TEXT,
		owner_member_id TEXT,
		title TEXT NOT NULL,
		meeting_url TEXT NOT NULL,
		platform TEXT NOT NULL,
		native_meeting_id TEXT NOT NULL,
		status TEXT NOT NULL,
		summary_status TEXT NOT NULL,
		visibility TEXT NOT NULL,
		record_audio BOOLEAN NOT NULL DEFAULT 0,
		scheduled_start_at DATETIME,
		scheduled_end_at DATETIME,
		actual_start_at DATETIME,
		actual_end_at DATETIME,
		duration_seconds INTEGER NOT NULL DEFAULT 0,
		participants BLOB NOT NULL DEFAULT x'5b5d',
		failure_code TEXT,
		failure_message TEXT,
		recording_object_key TEXT,
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create meetings table: %v", err)
	}
	repo := repository.NewCRMMeetingRepository(db)
	return (&EmailSyncActivities{}).SetMeetingRepository(repo), repo
}

func TestReconcileScheduledCalendarMeetingAppliesRescheduleAndLinkChange(t *testing.T) {
	activities, repo := setupCalendarMeetingReconcileTest(t)
	ctx := context.Background()
	eventID := "calendar-event-1"
	originalStart := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	meeting := &model.CRMMeeting{
		ID:               "meeting-1",
		WorkspaceID:      "workspace-1",
		CalendarEventID:  &eventID,
		Title:            "Old title",
		MeetingURL:       "https://zoom.us/j/123456789",
		Platform:         model.CRMMeetingPlatformZoom,
		NativeMeetingID:  "123456789",
		Status:           model.CRMMeetingStatusScheduled,
		SummaryStatus:    model.CRMMeetingSummaryPending,
		Visibility:       model.CRMMeetingVisibilityWorkspace,
		ScheduledStartAt: &originalStart,
		Participants:     model.JSONBlob("[]"),
	}
	if err := repo.Create(ctx, meeting); err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	newStart := originalStart.Add(24 * time.Hour)
	newEnd := newStart.Add(30 * time.Minute)
	newURL := "https://meet.google.com/abc-defg-hij"
	event := &model.CRMCalendarEvent{
		ID:          eventID,
		WorkspaceID: meeting.WorkspaceID,
		Title:       "Updated customer call",
		MeetingURL:  &newURL,
		Status:      model.CRMCalendarEventStatusConfirmed,
		StartTime:   newStart,
		EndTime:     newEnd,
	}
	if err := activities.reconcileScheduledCalendarMeeting(ctx, event); err != nil {
		t.Fatalf("reconcile meeting: %v", err)
	}

	updated, err := repo.GetByID(ctx, meeting.WorkspaceID, meeting.ID)
	if err != nil {
		t.Fatalf("get meeting: %v", err)
	}
	if updated.Title != event.Title || updated.MeetingURL != newURL || updated.Platform != model.CRMMeetingPlatformGoogleMeet || updated.NativeMeetingID != "abc-defg-hij" {
		t.Fatalf("meeting metadata was not reconciled: %#v", updated)
	}
	if updated.ScheduledStartAt == nil || !updated.ScheduledStartAt.Equal(newStart) || updated.ScheduledEndAt == nil || !updated.ScheduledEndAt.Equal(newEnd) {
		t.Fatalf("meeting schedule was not reconciled: %#v", updated)
	}
}

func TestReconcileScheduledCalendarMeetingCancelsOnlyBeforeStart(t *testing.T) {
	activities, repo := setupCalendarMeetingReconcileTest(t)
	ctx := context.Background()
	eventID := "calendar-event-2"
	meeting := &model.CRMMeeting{
		ID:              "meeting-2",
		WorkspaceID:     "workspace-1",
		CalendarEventID: &eventID,
		Title:           "Customer call",
		MeetingURL:      "https://meet.google.com/abc-defg-hij",
		Platform:        model.CRMMeetingPlatformGoogleMeet,
		NativeMeetingID: "abc-defg-hij",
		Status:          model.CRMMeetingStatusScheduled,
		SummaryStatus:   model.CRMMeetingSummaryPending,
		Visibility:      model.CRMMeetingVisibilityWorkspace,
		Participants:    model.JSONBlob("[]"),
	}
	if err := repo.Create(ctx, meeting); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	event := &model.CRMCalendarEvent{
		ID:          eventID,
		WorkspaceID: meeting.WorkspaceID,
		Title:       meeting.Title,
		Status:      model.CRMCalendarEventStatusCancelled,
		StartTime:   time.Now().UTC().Add(time.Hour),
		EndTime:     time.Now().UTC().Add(2 * time.Hour),
	}
	if err := activities.reconcileScheduledCalendarMeeting(ctx, event); err != nil {
		t.Fatalf("cancel meeting: %v", err)
	}
	updated, err := repo.GetByID(ctx, meeting.WorkspaceID, meeting.ID)
	if err != nil {
		t.Fatalf("get meeting: %v", err)
	}
	if updated.Status != model.CRMMeetingStatusCancelled {
		t.Fatalf("status = %q, want cancelled", updated.Status)
	}
}
