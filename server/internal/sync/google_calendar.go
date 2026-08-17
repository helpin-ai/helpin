package sync

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const googleCalendarAPIBase = "https://www.googleapis.com/calendar/v3"

var calendarURLPattern = regexp.MustCompile(`https://[^\s<>"']+`)

// GoogleCalendarAttendee is a normalized Google Calendar participant.
type GoogleCalendarAttendee struct {
	Email          string
	Name           string
	ResponseStatus string
	Organizer      bool
	Self           bool
}

// GoogleCalendarEvent is the provider-neutral subset needed by CRM meeting capture.
type GoogleCalendarEvent struct {
	ID                string
	RecurringSeriesID string
	Title             string
	Description       string
	StartTime         time.Time
	EndTime           time.Time
	Location          string
	MeetingURL        string
	OrganizerEmail    string
	Status            string
	Visibility        string
	AllDay            bool
	Attendees         []GoogleCalendarAttendee
}

// ListCalendarEvents returns the primary calendar window, including cancelled
// events so local records can be updated instead of becoming stale.
func (c *GmailSyncClient) ListCalendarEvents(
	ctx context.Context,
	accessToken string,
	start, end time.Time,
) ([]GoogleCalendarEvent, error) {
	if c == nil {
		return nil, fmt.Errorf("google calendar client is not configured")
	}
	params := url.Values{}
	params.Set("singleEvents", "true")
	params.Set("orderBy", "startTime")
	params.Set("showDeleted", "true")
	params.Set("maxResults", "2500")
	params.Set("conferenceDataVersion", "1")
	params.Set("timeMin", start.UTC().Format(time.RFC3339))
	params.Set("timeMax", end.UTC().Format(time.RFC3339))

	events := make([]GoogleCalendarEvent, 0)
	pageToken := ""
	for {
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		endpoint := c.calendarBaseURL() + "/calendars/primary/events?" + params.Encode()
		var response googleCalendarEventsResponse
		if err := c.apiGet(ctx, accessToken, endpoint, &response); err != nil {
			return nil, fmt.Errorf("list google calendar events: %w", err)
		}
		for _, raw := range response.Items {
			event, ok := normalizeGoogleCalendarEvent(raw)
			if ok {
				events = append(events, event)
			}
		}
		pageToken = strings.TrimSpace(response.NextPageToken)
		if pageToken == "" {
			break
		}
	}
	return events, nil
}

func (c *GmailSyncClient) calendarBaseURL() string {
	if strings.TrimSpace(c.calendarAPIBaseURL) != "" {
		return strings.TrimRight(c.calendarAPIBaseURL, "/")
	}
	return googleCalendarAPIBase
}

type googleCalendarEventsResponse struct {
	NextPageToken string                   `json:"nextPageToken"`
	Items         []googleCalendarRawEvent `json:"items"`
}

type googleCalendarRawEvent struct {
	ID               string `json:"id"`
	RecurringEventID string `json:"recurringEventId"`
	Status           string `json:"status"`
	Summary          string `json:"summary"`
	Description      string `json:"description"`
	Location         string `json:"location"`
	Visibility       string `json:"visibility"`
	HangoutLink      string `json:"hangoutLink"`
	Organizer        struct {
		Email string `json:"email"`
	} `json:"organizer"`
	Start     googleCalendarDateTime `json:"start"`
	End       googleCalendarDateTime `json:"end"`
	Attendees []struct {
		Email          string `json:"email"`
		DisplayName    string `json:"displayName"`
		ResponseStatus string `json:"responseStatus"`
		Organizer      bool   `json:"organizer"`
		Self           bool   `json:"self"`
	} `json:"attendees"`
	ConferenceData struct {
		EntryPoints []struct {
			EntryPointType string `json:"entryPointType"`
			URI            string `json:"uri"`
		} `json:"entryPoints"`
	} `json:"conferenceData"`
}

type googleCalendarDateTime struct {
	DateTime string `json:"dateTime"`
	Date     string `json:"date"`
}

func normalizeGoogleCalendarEvent(raw googleCalendarRawEvent) (GoogleCalendarEvent, bool) {
	status := strings.TrimSpace(raw.Status)
	start, startAllDay, err := parseGoogleCalendarTime(raw.Start)
	if err != nil {
		if strings.EqualFold(status, "cancelled") {
			id := strings.TrimSpace(raw.ID)
			return GoogleCalendarEvent{ID: id, Status: status}, id != ""
		}
		return GoogleCalendarEvent{}, false
	}
	end, endAllDay, err := parseGoogleCalendarTime(raw.End)
	if err != nil {
		return GoogleCalendarEvent{}, false
	}
	title := strings.TrimSpace(raw.Summary)
	if title == "" {
		title = "Untitled meeting"
	}
	attendees := make([]GoogleCalendarAttendee, 0, len(raw.Attendees))
	for _, attendee := range raw.Attendees {
		email := strings.ToLower(strings.TrimSpace(attendee.Email))
		if email == "" {
			continue
		}
		attendees = append(attendees, GoogleCalendarAttendee{
			Email:          email,
			Name:           strings.TrimSpace(attendee.DisplayName),
			ResponseStatus: strings.TrimSpace(attendee.ResponseStatus),
			Organizer:      attendee.Organizer,
			Self:           attendee.Self,
		})
	}
	return GoogleCalendarEvent{
		ID:                strings.TrimSpace(raw.ID),
		RecurringSeriesID: strings.TrimSpace(raw.RecurringEventID),
		Title:             title,
		Description:       strings.TrimSpace(raw.Description),
		StartTime:         start,
		EndTime:           end,
		Location:          strings.TrimSpace(raw.Location),
		MeetingURL:        calendarMeetingURL(raw),
		OrganizerEmail:    strings.ToLower(strings.TrimSpace(raw.Organizer.Email)),
		Status:            status,
		Visibility:        strings.TrimSpace(raw.Visibility),
		AllDay:            startAllDay || endAllDay,
		Attendees:         attendees,
	}, strings.TrimSpace(raw.ID) != ""
}

func parseGoogleCalendarTime(value googleCalendarDateTime) (time.Time, bool, error) {
	if strings.TrimSpace(value.DateTime) != "" {
		parsed, err := time.Parse(time.RFC3339, value.DateTime)
		return parsed, false, err
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value.Date))
	return parsed.UTC(), true, err
}

func calendarMeetingURL(raw googleCalendarRawEvent) string {
	candidates := []string{raw.HangoutLink}
	for _, entry := range raw.ConferenceData.EntryPoints {
		if strings.EqualFold(entry.EntryPointType, "video") {
			candidates = append(candidates, entry.URI)
		}
	}
	for _, text := range []string{raw.Location, html.UnescapeString(raw.Description)} {
		candidates = append(candidates, calendarURLPattern.FindAllString(text, -1)...)
	}
	for _, candidate := range candidates {
		candidate = strings.TrimRight(strings.TrimSpace(candidate), ".,;:!?)]}")
		if supportedCalendarMeetingHost(candidate) {
			return candidate
		}
	}
	return ""
}

func supportedCalendarMeetingHost(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "meet.google.com" || host == "zoom.us" || strings.HasSuffix(host, ".zoom.us") ||
		host == "teams.live.com" || host == "teams.microsoft.com" || host == "webex.com" || strings.HasSuffix(host, ".webex.com")
}
