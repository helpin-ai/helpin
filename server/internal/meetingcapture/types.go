// Package meetingcapture contains provider-specific meeting bot adapters.
package meetingcapture

import (
	"errors"
	"net/http"
	"time"
)

// ErrNotConfigured is returned when a selected provider has no usable credentials.
var ErrNotConfigured = errors.New("meeting capture provider is not configured")

// StartCaptureInput is the provider-neutral input for launching a meeting bot.
type StartCaptureInput struct {
	WorkspaceID     string
	MeetingID       string
	MeetingURL      string
	Platform        string
	NativeMeetingID string
	BotName         string
	JoinAt          *time.Time
	RecordAudio     bool
}

// Capture is the normalized result of starting or retrieving a provider bot.
type Capture struct {
	ProviderCaptureID string
	ProviderStatus    string
	Status            string
	RecordingID       string
	TranscriptID      string
	StartedAt         *time.Time
	EndedAt           *time.Time
	FailureCode       string
	FailureMessage    string
}

// TranscriptSegment is one provider-normalized utterance.
type TranscriptSegment struct {
	ID           string
	SpeakerID    string
	SpeakerName  string
	Text         string
	StartSeconds float64
	EndSeconds   float64
	Language     string
	Confidence   float64
}

// Transcript is a provider-neutral completed transcript.
type Transcript struct {
	ProviderTranscriptID string
	Language             string
	Segments             []TranscriptSegment
}

// Recording points to an expiring provider recording download.
type Recording struct {
	ProviderRecordingID string
	DownloadURL         string
	ContentType         string
	Headers             map[string]string
}

// ProviderEvent is a verified and normalized provider webhook.
type ProviderEvent struct {
	EventID              string
	EventType            string
	ProviderCaptureID    string
	ProviderStatus       string
	Status               string
	ProviderRecordingID  string
	ProviderTranscriptID string
	FailureCode          string
	FailureMessage       string
	OccurredAt           *time.Time
	TranscriptReady      bool
}

// Headers is the webhook header subset used by provider verification.
type Headers http.Header
