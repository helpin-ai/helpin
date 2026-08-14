package meetingcapture

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultVexaBaseURL = "https://api.cloud.vexa.ai"

// VexaConfig configures the Vexa adapter.
type VexaConfig struct {
	BaseURL       string
	APIKey        string
	WebhookSecret string
	HTTPClient    *http.Client
}

// VexaProvider implements meeting capture using hosted or self-hosted Vexa.
type VexaProvider struct {
	apiKey        string
	webhookSecret string
	http          *httpClient
}

// NewVexaProvider creates a Vexa provider adapter.
func NewVexaProvider(cfg VexaConfig) *VexaProvider {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultVexaBaseURL
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	return &VexaProvider{
		apiKey:        apiKey,
		webhookSecret: strings.TrimSpace(cfg.WebhookSecret),
		http: newHTTPClient(baseURL, cfg.HTTPClient, func(req *http.Request) {
			req.Header.Set("X-API-Key", apiKey)
		}),
	}
}

// Name returns the stable provider key.
func (p *VexaProvider) Name() string { return "vexa" }

// Configured reports whether Vexa is ready for end-to-end capture and callbacks.
func (p *VexaProvider) Configured() bool { return p != nil && p.apiKey != "" && p.webhookSecret != "" }

// Supports reports whether Vexa supports the meeting platform.
func (p *VexaProvider) Supports(platform string) bool {
	return platform == "google_meet" || platform == "zoom" || platform == "teams"
}

// StartCapture launches a Vexa bot.
func (p *VexaProvider) StartCapture(ctx context.Context, input StartCaptureInput) (*Capture, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	meetingURL, _ := url.Parse(input.MeetingURL)
	passcode := ""
	legacyTeamsURL := false
	if meetingURL != nil {
		passcode = firstNonBlank(meetingURL.Query().Get("p"), meetingURL.Query().Get("pwd"))
		legacyTeamsURL = input.Platform == "teams" && strings.Contains(meetingURL.Path, "/l/meetup-join/")
	}
	if input.Platform == "teams" && !legacyTeamsURL && passcode == "" {
		return nil, fmt.Errorf("Vexa Teams capture requires a meeting URL with a p passcode")
	}
	body := map[string]interface{}{
		"platform":            input.Platform,
		"native_meeting_id":   input.NativeMeetingID,
		"meeting_url":         input.MeetingURL,
		"bot_name":            input.BotName,
		"recording_enabled":   input.RecordAudio,
		"transcribe_enabled":  true,
		"voice_agent_enabled": false,
	}
	if passcode != "" {
		body["passcode"] = passcode
	}
	var response map[string]interface{}
	if err := p.http.doJSON(ctx, http.MethodPost, "/bots", body, &response); err != nil {
		return nil, err
	}
	providerStatus, _ := response["status"].(string)
	if providerStatus == "" {
		providerStatus = "requested"
	}
	return &Capture{
		ProviderCaptureID: vexaCaptureID(input.Platform, input.NativeMeetingID),
		ProviderStatus:    providerStatus,
		Status:            normalizeVexaStatus(providerStatus),
	}, nil
}

// StopCapture removes a Vexa bot from the meeting.
func (p *VexaProvider) StopCapture(ctx context.Context, captureID string) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	platform, nativeID, err := parseVexaCaptureID(captureID)
	if err != nil {
		return err
	}
	return p.http.doJSON(ctx, http.MethodDelete, "/bots/"+url.PathEscape(platform)+"/"+url.PathEscape(nativeID), nil, nil)
}

// GetStatus reconciles a Vexa meeting from its meeting list.
func (p *VexaProvider) GetStatus(ctx context.Context, captureID string) (*Capture, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	platform, nativeID, err := parseVexaCaptureID(captureID)
	if err != nil {
		return nil, err
	}
	var response struct {
		Meetings []struct {
			Platform        string `json:"platform"`
			NativeMeetingID string `json:"native_meeting_id"`
			Status          string `json:"status"`
		} `json:"meetings"`
	}
	if err := p.http.doJSON(ctx, http.MethodGet, "/meetings?limit=100&offset=0", nil, &response); err != nil {
		return nil, err
	}
	for _, meeting := range response.Meetings {
		if meeting.Platform == platform && meeting.NativeMeetingID == nativeID {
			return &Capture{
				ProviderCaptureID: captureID,
				ProviderStatus:    meeting.Status,
				Status:            normalizeVexaStatus(meeting.Status),
			}, nil
		}
	}
	return nil, fmt.Errorf("Vexa capture not found")
}

// GetTranscript retrieves Vexa's speaker-attributed transcript.
func (p *VexaProvider) GetTranscript(ctx context.Context, captureID string) (*Transcript, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	platform, nativeID, err := parseVexaCaptureID(captureID)
	if err != nil {
		return nil, err
	}
	var response struct {
		Segments []struct {
			ID         string   `json:"segment_id"`
			Speaker    string   `json:"speaker"`
			Text       string   `json:"text"`
			Start      *float64 `json:"start"`
			End        *float64 `json:"end"`
			StartTime  *float64 `json:"start_time"`
			EndTime    *float64 `json:"end_time"`
			Language   string   `json:"language"`
			Confidence float64  `json:"confidence"`
			Completed  bool     `json:"completed"`
		} `json:"segments"`
	}
	path := "/transcripts/" + url.PathEscape(platform) + "/" + url.PathEscape(nativeID)
	if err := p.http.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	segments := make([]TranscriptSegment, 0, len(response.Segments))
	language := ""
	for _, segment := range response.Segments {
		if !segment.Completed || strings.TrimSpace(segment.Text) == "" {
			continue
		}
		if language == "" {
			language = segment.Language
		}
		segments = append(segments, TranscriptSegment{
			ID:           segment.ID,
			SpeakerName:  firstNonBlank(segment.Speaker, "Unknown speaker"),
			Text:         strings.TrimSpace(segment.Text),
			StartSeconds: firstVexaTimestamp(segment.Start, segment.StartTime),
			EndSeconds:   firstVexaTimestamp(segment.End, segment.EndTime),
			Language:     segment.Language,
			Confidence:   segment.Confidence,
		})
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("Vexa transcript is not ready")
	}
	return &Transcript{
		ProviderTranscriptID: captureID,
		Language:             language,
		Segments:             segments,
	}, nil
}

// GetRecording resolves a Vexa recording download when the meeting has one.
func (p *VexaProvider) GetRecording(ctx context.Context, captureID string) (*Recording, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	platform, nativeID, err := parseVexaCaptureID(captureID)
	if err != nil {
		return nil, err
	}
	var response map[string]interface{}
	path := "/transcripts/" + url.PathEscape(platform) + "/" + url.PathEscape(nativeID)
	if err := p.http.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	recordingID, mediaID := findVexaRecording(response)
	if recordingID == "" || mediaID == "" {
		return nil, fmt.Errorf("Vexa recording is not ready")
	}
	return &Recording{
		ProviderRecordingID: recordingID,
		DownloadURL:         p.http.baseURL + "/recordings/" + url.PathEscape(recordingID) + "/media/" + url.PathEscape(mediaID) + "/raw",
		ContentType:         "audio/webm",
		Headers:             map[string]string{"X-API-Key": p.apiKey},
	}, nil
}

// DeleteArtifacts deletes the provider meeting artifacts where supported.
func (p *VexaProvider) DeleteArtifacts(ctx context.Context, captureID string) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	platform, nativeID, err := parseVexaCaptureID(captureID)
	if err != nil {
		return err
	}
	return p.http.doJSON(ctx, http.MethodDelete, "/meetings/"+url.PathEscape(platform)+"/"+url.PathEscape(nativeID), nil, nil)
}

// VerifyWebhook verifies the bearer secret configured through Vexa's user webhook API.
func (p *VexaProvider) VerifyWebhook(headers http.Header, payload []byte) error {
	if p == nil || p.webhookSecret == "" {
		return fmt.Errorf("Vexa webhook secret is not configured")
	}
	signature := strings.TrimSpace(headers.Get("X-Webhook-Signature"))
	timestamp := strings.TrimSpace(headers.Get("X-Webhook-Timestamp"))
	if signature != "" || timestamp != "" {
		if signature == "" || timestamp == "" {
			return fmt.Errorf("missing Vexa webhook signature headers")
		}
		unixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid Vexa webhook timestamp")
		}
		deliveredAt := time.Unix(unixTimestamp, 0)
		if delta := time.Since(deliveredAt); delta > 5*time.Minute || delta < -5*time.Minute {
			return fmt.Errorf("stale Vexa webhook timestamp")
		}
		mac := hmac.New(sha256.New, []byte(p.webhookSecret))
		mac.Write([]byte(timestamp + "."))
		mac.Write(payload)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(expected)) {
			return fmt.Errorf("invalid Vexa webhook signature")
		}
		return nil
	}
	expected := []byte("Bearer " + p.webhookSecret)
	received := []byte(strings.TrimSpace(headers.Get("Authorization")))
	if !hmac.Equal(received, expected) {
		return fmt.Errorf("invalid Vexa webhook authorization")
	}
	return nil
}

// NormalizeWebhook converts Vexa's meeting lifecycle event into Helpin status.
func (p *VexaProvider) NormalizeWebhook(headers http.Header, payload []byte) (*ProviderEvent, error) {
	var envelope vexaWebhookEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("decode Vexa webhook: %w", err)
	}
	meeting := envelope.Meeting
	if meeting.Platform == "" && meeting.NativeMeetingID == "" {
		meeting = envelope.Data.Meeting
	}
	statusChange := envelope.StatusChange
	if statusChange.To == "" {
		statusChange = envelope.Data.StatusChange
	}
	eventType := strings.TrimSpace(envelope.EventType)
	providerStatus := firstNonBlank(meeting.Status, statusChange.To)
	eventID := firstNonBlank(envelope.EventID, headers.Get("X-Vexa-Event-Id"))
	if eventID == "" {
		hash := sha256.Sum256(payload)
		eventID = hex.EncodeToString(hash[:])
	}
	failureCode, failureMessage := "", ""
	if providerStatus == "failed" {
		failureCode = firstNonBlank(meeting.FailureStage, jsonString(meeting.Data["failure_code"]), statusChange.Reason)
		failureMessage = firstNonBlank(jsonString(meeting.Data["failure_message"]), statusChange.Reason, meeting.CompletionReason)
	}
	transcriptReady := (eventType == "meeting.completed" && providerStatus == "completed") ||
		(eventType == "meeting.status_change" && providerStatus == "completed") ||
		eventType == "transcription.ready"
	event := &ProviderEvent{
		EventID:           eventID,
		EventType:         eventType,
		ProviderCaptureID: vexaCaptureID(meeting.Platform, meeting.NativeMeetingID),
		ProviderStatus:    providerStatus,
		Status:            normalizeVexaStatus(providerStatus),
		FailureCode:       failureCode,
		FailureMessage:    failureMessage,
		TranscriptReady:   transcriptReady,
	}
	if event.TranscriptReady {
		event.Status = "processing"
	}
	occurredAt := firstNonBlank(statusChange.Timestamp, meeting.UpdatedAt, envelope.CreatedAt)
	if parsed, err := time.Parse(time.RFC3339Nano, occurredAt); err == nil {
		event.OccurredAt = &parsed
	}
	return event, nil
}

type vexaWebhookMeeting struct {
	Platform         string                 `json:"platform"`
	NativeMeetingID  string                 `json:"native_meeting_id"`
	Status           string                 `json:"status"`
	CompletionReason string                 `json:"completion_reason"`
	FailureStage     string                 `json:"failure_stage"`
	UpdatedAt        string                 `json:"updated_at"`
	Data             map[string]interface{} `json:"data"`
}

type vexaWebhookStatusChange struct {
	To        string `json:"to"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

type vexaWebhookEnvelope struct {
	EventID      string                  `json:"event_id"`
	EventType    string                  `json:"event_type"`
	CreatedAt    string                  `json:"created_at"`
	Meeting      vexaWebhookMeeting      `json:"meeting"`
	StatusChange vexaWebhookStatusChange `json:"status_change"`
	Data         struct {
		Meeting      vexaWebhookMeeting      `json:"meeting"`
		StatusChange vexaWebhookStatusChange `json:"status_change"`
	} `json:"data"`
}

func firstVexaTimestamp(primary, legacy *float64) float64 {
	if primary != nil {
		return *primary
	}
	if legacy != nil {
		return *legacy
	}
	return 0
}

func normalizeVexaStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "idle", "scheduled":
		return "scheduled"
	case "requested", "joining", "needs_help":
		return "joining"
	case "awaiting_admission":
		return "waiting"
	case "active", "stopping":
		return "recording"
	case "completed":
		return "processing"
	case "failed":
		return "failed"
	default:
		return "joining"
	}
}

func vexaCaptureID(platform, nativeID string) string {
	return strings.TrimSpace(platform) + ":" + strings.TrimSpace(nativeID)
}

func parseVexaCaptureID(captureID string) (string, string, error) {
	parts := strings.SplitN(captureID, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid Vexa capture id")
	}
	return parts[0], parts[1], nil
}

func findVexaRecording(payload map[string]interface{}) (string, string) {
	recordings, _ := payload["recordings"].([]interface{})
	for _, value := range recordings {
		recording, _ := value.(map[string]interface{})
		recordingID := jsonString(recording["id"])
		files, _ := recording["media_files"].([]interface{})
		for _, fileValue := range files {
			file, _ := fileValue.(map[string]interface{})
			mediaID := jsonString(file["id"])
			if recordingID != "" && mediaID != "" {
				return recordingID, mediaID
			}
		}
	}
	return "", ""
}

func jsonString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case float32:
		return fmt.Sprintf("%.0f", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}
