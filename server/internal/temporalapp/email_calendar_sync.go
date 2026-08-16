package temporalapp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/model"
	syncpkg "github.com/helpin-ai/helpin/server/internal/sync"
)

const (
	calendarSyncPastWindow   = 24 * time.Hour
	calendarSyncFutureWindow = 30 * 24 * time.Hour
)

type googleCalendarSyncClient interface {
	ListCalendarEvents(ctx context.Context, accessToken string, start, end time.Time) ([]syncpkg.GoogleCalendarEvent, error)
}

func (a *EmailSyncActivities) syncCalendarEvents(
	ctx context.Context,
	account *model.CRMEmailAccount,
	accessToken string,
	settings *model.CRMEmailSyncSettings,
) error {
	if a == nil || a.calendarRepo == nil || account == nil {
		return nil
	}
	client, ok := a.gmailClient.(googleCalendarSyncClient)
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	events, err := client.ListCalendarEvents(ctx, accessToken, now.Add(-calendarSyncPastWindow), now.Add(calendarSyncFutureWindow))
	if err != nil {
		return err
	}
	for index := range events {
		if err := a.storeCalendarEvent(ctx, account, settings, &events[index]); err != nil {
			slog.WarnContext(ctx, "failed to store synced calendar event", "error", err, "workspace_id", account.WorkspaceID, "account_id", account.ID, "external_event_id", events[index].ID)
		}
	}
	slog.InfoContext(ctx, "crm calendar sync complete", "workspace_id", account.WorkspaceID, "account_id", account.ID, "events_seen", len(events))
	return nil
}

func (a *EmailSyncActivities) storeCalendarEvent(
	ctx context.Context,
	account *model.CRMEmailAccount,
	settings *model.CRMEmailSyncSettings,
	event *syncpkg.GoogleCalendarEvent,
) error {
	if event == nil || strings.TrimSpace(event.ID) == "" {
		return nil
	}
	existing, err := a.calendarRepo.GetByExternalID(ctx, account.ID, event.ID)
	if err != nil {
		return err
	}
	if strings.EqualFold(event.Status, model.CRMCalendarEventStatusCancelled) {
		if existing == nil {
			return nil
		}
		existing.Status = model.CRMCalendarEventStatusCancelled
		if err := a.calendarRepo.Update(ctx, existing); err != nil {
			return err
		}
		return a.reconcileScheduledCalendarMeeting(ctx, existing)
	}
	if calendarEventExcluded(settings, account, event) {
		if existing == nil {
			return nil
		}
		existing.Status = model.CRMCalendarEventStatusCancelled
		if err := a.reconcileScheduledCalendarMeeting(ctx, existing); err != nil {
			return err
		}
		return a.calendarRepo.Delete(ctx, account.WorkspaceID, existing.ID)
	}

	resolution, err := a.resolveCalendarAttendees(ctx, account, settings, event)
	if err != nil {
		return err
	}
	attendees := make(model.CRMCalendarAttendees, 0, len(event.Attendees))
	for _, attendee := range event.Attendees {
		attendees = append(attendees, model.CRMCalendarAttendee{
			Email:          attendee.Email,
			Name:           attendee.Name,
			ResponseStatus: attendee.ResponseStatus,
			Organizer:      attendee.Organizer,
			Self:           attendee.Self,
		})
	}
	externalID := strings.TrimSpace(event.ID)
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = model.CRMCalendarEventStatusConfirmed
	}
	visibility := strings.TrimSpace(event.Visibility)
	if visibility == "" {
		visibility = "default"
	}
	record := &model.CRMCalendarEvent{
		WorkspaceID:     account.WorkspaceID,
		EmailAccountID:  account.ID,
		ExternalEventID: &externalID,
		Title:           strings.TrimSpace(event.Title),
		Description:     calendarStringPointer(event.Description),
		StartTime:       event.StartTime.UTC(),
		EndTime:         event.EndTime.UTC(),
		Location:        calendarStringPointer(event.Location),
		MeetingURL:      calendarStringPointer(event.MeetingURL),
		OrganizerEmail:  calendarStringPointer(strings.ToLower(event.OrganizerEmail)),
		Status:          status,
		Visibility:      visibility,
		AllDay:          event.AllDay,
		Attendees:       attendees,
		ContactIDs:      model.CRMStringList(resolution.ContactIDs),
	}
	if existing != nil {
		record.ID = existing.ID
		record.DealID = existing.DealID
		record.CreatedAt = existing.CreatedAt
	}
	if err := a.calendarRepo.UpsertSyncedEvent(ctx, record); err != nil {
		return fmt.Errorf("upsert calendar event: %w", err)
	}
	if err := a.reconcileScheduledCalendarMeeting(ctx, record); err != nil {
		return fmt.Errorf("reconcile scheduled calendar meeting: %w", err)
	}
	return nil
}

func calendarEventExcluded(settings *model.CRMEmailSyncSettings, account *model.CRMEmailAccount, event *syncpkg.GoogleCalendarEvent) bool {
	if settings == nil || account == nil || event == nil {
		return false
	}
	if !settings.IncludePrivateMeetings && strings.EqualFold(strings.TrimSpace(event.Visibility), "private") {
		return true
	}
	externalAttendees := make([]string, 0, len(event.Attendees))
	allAttendees := make([]string, 0, len(event.Attendees))
	for _, attendee := range event.Attendees {
		email := strings.ToLower(strings.TrimSpace(attendee.Email))
		if email == "" {
			continue
		}
		allAttendees = append(allAttendees, email)
		if attendee.Self || strings.EqualFold(email, account.EmailAddress) {
			continue
		}
		externalAttendees = append(externalAttendees, email)
	}
	organizerIsExternal := strings.TrimSpace(event.OrganizerEmail) != "" && !strings.EqualFold(event.OrganizerEmail, account.EmailAddress)
	if !settings.IncludeSoloMeetings && len(externalAttendees) == 0 && !organizerIsExternal {
		return true
	}
	return model.IsInternalEmail(settings, event.OrganizerEmail, allAttendees, nil, account.EmailAddress)
}

func (a *EmailSyncActivities) resolveCalendarAttendees(
	ctx context.Context,
	account *model.CRMEmailAccount,
	settings *model.CRMEmailSyncSettings,
	event *syncpkg.GoogleCalendarEvent,
) (*crmemail.ResolveResult, error) {
	direction := model.CRMEmailDirectionInbound
	if strings.EqualFold(event.OrganizerEmail, account.EmailAddress) {
		direction = model.CRMEmailDirectionOutbound
	}
	participants := make([]crmemail.Participant, 0, len(event.Attendees))
	for _, attendee := range event.Attendees {
		participants = append(participants, crmemail.Participant{
			Email: attendee.Email,
			Name:  attendee.Name,
			Role:  model.CRMEmailParticipantRoleTo,
		})
	}
	return a.resolver.Resolve(ctx, crmemail.ResolveInput{
		WorkspaceID: account.WorkspaceID,
		Direction:   direction,
		Settings:    settings,
		SelfEmails:  []string{account.EmailAddress},
		From: crmemail.Participant{
			Email: event.OrganizerEmail,
			Role:  model.CRMEmailParticipantRoleFrom,
		},
		To: participants,
	})
}

func calendarStringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
