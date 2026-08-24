package crmsignal

import (
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPreferredBodySanitizesHTMLBeforePrompting(t *testing.T) {
	htmlBody := `<p>Please send pri<strong>cing</strong> &amp; security details.</p>`
	got := preferredBody(nil, &htmlBody)
	if got != "Please send pricing & security details." {
		t.Fatalf("preferredBody() = %q", got)
	}
}

func TestCalendarPayloadIsTargetedAndContentVersioned(t *testing.T) {
	contactID := "contact-1"
	event := &model.CRMCalendarEvent{
		ID: "event-1", WorkspaceID: "workspace-1", Title: "Review",
		Status: model.CRMCalendarEventStatusConfirmed, StartTime: time.Now().UTC(),
		ContactIDs: model.CRMStringList{contactID},
		Attendees:  model.CRMCalendarAttendees{{Email: "buyer@example.com", ResponseStatus: "accepted"}},
	}
	payload, eligible := CalendarPayload(event)
	if !eligible || payload == nil {
		t.Fatal("targeted calendar event should be eligible")
	}
	firstKey := CalendarWorkflowKey(*payload)
	event.Attendees[0].ResponseStatus = "declined"
	updated, eligible := CalendarPayload(event)
	if !eligible || updated == nil {
		t.Fatal("updated calendar event should be eligible")
	}
	secondKey := CalendarWorkflowKey(*updated)
	if firstKey == secondKey || !strings.HasPrefix(secondKey, "calendar-event-1-") {
		t.Fatalf("workflow keys = %q and %q, want distinct versioned keys", firstKey, secondKey)
	}
}
