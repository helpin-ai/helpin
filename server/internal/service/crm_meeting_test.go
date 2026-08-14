package service

import "testing"

func TestParseMeetingURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		url       string
		platform  string
		nativeID  string
		wantError bool
	}{
		{name: "Google Meet", url: "https://meet.google.com/abc-defg-hij", platform: "google_meet", nativeID: "abc-defg-hij"},
		{name: "Zoom", url: "https://acme.zoom.us/j/123456789", platform: "zoom", nativeID: "123456789"},
		{name: "Teams", url: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_example", platform: "teams", nativeID: "19%3ameeting_example"},
		{name: "Webex", url: "https://acme.webex.com/meet/alex", platform: "webex", nativeID: "alex"},
		{name: "HTTP rejected", url: "http://meet.google.com/abc-defg-hij", wantError: true},
		{name: "Unknown host", url: "https://example.com/meeting", wantError: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			platform, nativeID, err := ParseMeetingURL(test.url)
			if test.wantError {
				if err == nil {
					t.Fatalf("ParseMeetingURL(%q) expected error", test.url)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMeetingURL(%q): %v", test.url, err)
			}
			if platform != test.platform || nativeID != test.nativeID {
				t.Fatalf("got (%q, %q), want (%q, %q)", platform, nativeID, test.platform, test.nativeID)
			}
		})
	}
}
