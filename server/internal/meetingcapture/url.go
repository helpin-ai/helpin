package meetingcapture

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ParseMeetingURL validates a supported meeting URL and returns its platform
// and provider-native meeting identifier.
func ParseMeetingURL(rawURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return "", "", fmt.Errorf("enter a valid HTTPS meeting URL")
	}
	host := strings.ToLower(parsed.Hostname())
	decodedPath, decodeErr := url.PathUnescape(parsed.EscapedPath())
	if decodeErr != nil {
		return "", "", fmt.Errorf("enter a valid HTTPS meeting URL")
	}
	parts := meetingPathParts(decodedPath)
	switch {
	case host == "meet.google.com":
		if len(parts) != 1 || !validGoogleMeetCode(parts[0]) {
			return "", "", fmt.Errorf("enter a valid Google Meet URL")
		}
		return model.CRMMeetingPlatformGoogleMeet, parts[0], nil
	case strings.HasSuffix(host, ".zoom.us") || host == "zoom.us":
		meetingID := meetingPathValueAfter(parts, "j", "join")
		if !numericMeetingID(meetingID, 9, 13) {
			return "", "", fmt.Errorf("enter a valid Zoom meeting URL")
		}
		return model.CRMMeetingPlatformZoom, meetingID, nil
	case host == "teams.live.com" || host == "teams.microsoft.com":
		meetingID := meetingPathValueAfter(parts, "meetup-join")
		if meetingID == "" {
			meetingID = meetingPathValueAfter(parts, "meet")
		}
		if meetingID == "" || meetingID == "0" {
			return "", "", fmt.Errorf("enter a valid Microsoft Teams meeting URL")
		}
		return model.CRMMeetingPlatformTeams, meetingID, nil
	case strings.HasSuffix(host, ".webex.com") || host == "webex.com":
		meetingID := strings.TrimSpace(parsed.Query().Get("MTID"))
		if meetingID == "" {
			meetingID = meetingPathValueAfter(parts, "meet")
		}
		if meetingID == "" {
			return "", "", fmt.Errorf("enter a valid Webex meeting URL")
		}
		return model.CRMMeetingPlatformWebex, meetingID, nil
	default:
		return "", "", fmt.Errorf("meeting provider is not supported")
	}
}

func validGoogleMeetCode(value string) bool {
	parts := strings.Split(strings.ToLower(value), "-")
	return len(parts) == 3 && len(parts[0]) == 3 && len(parts[1]) == 4 && len(parts[2]) == 3
}

func meetingPathParts(path string) []string {
	rawParts := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func meetingPathValueAfter(parts []string, markers ...string) string {
	for index, part := range parts {
		for _, marker := range markers {
			if strings.EqualFold(part, marker) && index+1 < len(parts) {
				return strings.TrimSpace(parts[index+1])
			}
		}
	}
	return ""
}

func numericMeetingID(value string, minLength, maxLength int) bool {
	if len(value) < minLength || len(value) > maxLength {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
