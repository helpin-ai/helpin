package temporalapp

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	syncpkg "github.com/helpin-ai/helpin/server/internal/sync"
)

func TestCalendarEventExcludedHonorsPrivacySoloAndInternalSettings(t *testing.T) {
	account := &model.CRMEmailAccount{EmailAddress: "owner@helpin.example"}
	settings := model.DefaultEmailSyncSettings()

	privateEvent := &syncpkg.GoogleCalendarEvent{Visibility: "private", OrganizerEmail: account.EmailAddress}
	if !calendarEventExcluded(&settings, account, privateEvent) {
		t.Fatal("private events should be excluded by default")
	}
	settings.IncludePrivateMeetings = true
	if !calendarEventExcluded(&settings, account, privateEvent) {
		t.Fatal("a private solo event should remain excluded while solo meetings are disabled")
	}

	settings.IncludeSoloMeetings = true
	if calendarEventExcluded(&settings, account, privateEvent) {
		t.Fatal("private and solo event should be included when both settings are enabled")
	}

	settings.InternalExclusion = "exclude"
	internalEvent := &syncpkg.GoogleCalendarEvent{
		OrganizerEmail: account.EmailAddress,
		Attendees: []syncpkg.GoogleCalendarAttendee{
			{Email: account.EmailAddress, Self: true},
			{Email: "coworker@helpin.example"},
		},
	}
	if !calendarEventExcluded(&settings, account, internalEvent) {
		t.Fatal("internal-only event should be excluded")
	}
	internalEvent.Attendees = append(internalEvent.Attendees, syncpkg.GoogleCalendarAttendee{Email: "buyer@example.com"})
	if calendarEventExcluded(&settings, account, internalEvent) {
		t.Fatal("event with an external attendee should be included")
	}
}
