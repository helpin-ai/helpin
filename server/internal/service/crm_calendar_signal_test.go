package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type recordingCalendarSignalStarter struct {
	key      string
	payloads []model.SignalSourcePayload
}

func (r *recordingCalendarSignalStarter) StartSignalDetection(_ context.Context, key string, payloads []model.SignalSourcePayload) error {
	r.key, r.payloads = key, payloads
	return nil
}

func TestCRMCalendarServiceEnqueuesTargetedEventForSignalDetection(t *testing.T) {
	starter := &recordingCalendarSignalStarter{}
	calendarService := NewCRMCalendarService(nil).SetSignalDetection(starter, nil)
	contactID := "contact-1"
	event := &model.CRMCalendarEvent{
		ID: "event-1", WorkspaceID: "workspace-1", Title: "Implementation review",
		Status: model.CRMCalendarEventStatusConfirmed, StartTime: time.Now().UTC(),
		ContactIDs: model.CRMStringList{contactID},
		Attendees:  model.CRMCalendarAttendees{{Email: "buyer@example.com", ResponseStatus: "declined"}},
	}

	calendarService.enqueueSignalDetection(context.Background(), event)
	if !strings.HasPrefix(starter.key, "calendar-event-1-") || len(starter.payloads) != 1 {
		t.Fatalf("enqueue = key %q payloads %#v", starter.key, starter.payloads)
	}
	if !strings.Contains(starter.payloads[0].Body, "response: declined") {
		t.Fatalf("payload body = %q", starter.payloads[0].Body)
	}
}
