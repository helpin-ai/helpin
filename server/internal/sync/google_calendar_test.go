package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGmailSyncClient_ListCalendarEventsNormalizesConferenceAndAttendees(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/calendar/v3/calendars/primary/events" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("singleEvents") != "true" || r.URL.Query().Get("showDeleted") != "true" {
			t.Fatalf("query = %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items":[{
				"id":"event-1",
				"recurringEventId":"series-1",
				"status":"confirmed",
				"summary":"Customer call",
				"description":"Join at https://zoom.us/j/123456789",
				"organizer":{"email":"owner@example.com"},
				"start":{"dateTime":"2026-08-18T10:00:00Z"},
				"end":{"dateTime":"2026-08-18T11:00:00Z"},
				"attendees":[
					{"email":"owner@example.com","self":true,"responseStatus":"accepted"},
					{"email":"BUYER@example.com","displayName":"Buyer","responseStatus":"accepted"}
				]
			}]
		}`))
	}))
	defer server.Close()

	client := &GmailSyncClient{
		httpClient:         server.Client(),
		calendarAPIBaseURL: server.URL + "/calendar/v3",
	}
	start := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	events, err := client.ListCalendarEvents(context.Background(), "token", start, start.AddDate(0, 0, 30))
	if err != nil {
		t.Fatalf("ListCalendarEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	event := events[0]
	if event.MeetingURL != "https://zoom.us/j/123456789" {
		t.Fatalf("meeting_url = %q", event.MeetingURL)
	}
	if event.RecurringSeriesID != "series-1" {
		t.Fatalf("recurring_series_id = %q", event.RecurringSeriesID)
	}
	if len(event.Attendees) != 2 || event.Attendees[1].Email != "buyer@example.com" {
		t.Fatalf("attendees = %+v", event.Attendees)
	}
}

func TestNormalizeGoogleCalendarEventSupportsGoogleConferenceData(t *testing.T) {
	var raw googleCalendarRawEvent
	raw.ID = "event-2"
	raw.Summary = "Demo"
	raw.Start.DateTime = "2026-08-18T10:00:00Z"
	raw.End.DateTime = "2026-08-18T10:30:00Z"
	raw.ConferenceData.EntryPoints = append(raw.ConferenceData.EntryPoints, struct {
		EntryPointType string `json:"entryPointType"`
		URI            string `json:"uri"`
	}{EntryPointType: "video", URI: "https://meet.google.com/abc-defg-hij"})

	event, ok := normalizeGoogleCalendarEvent(raw)
	if !ok {
		t.Fatal("event was not normalized")
	}
	if event.MeetingURL != "https://meet.google.com/abc-defg-hij" {
		t.Fatalf("meeting_url = %q", event.MeetingURL)
	}
}
