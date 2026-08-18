package meetingcapture

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultRecallBaseURL = "https://us-east-1.recall.ai"

//go:embed helpin-logo.png
var helpinLogoPNG []byte

var helpinRecallBotImage = buildRecallBotImage()

func buildRecallBotImage() string {
	source, err := png.Decode(bytes.NewReader(helpinLogoPNG))
	if err != nil {
		return ""
	}

	const (
		canvasWidth  = 1280
		canvasHeight = 720
		logoBoxSize  = 288
	)
	canvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{R: 248, G: 248, B: 247, A: 255}), image.Point{}, draw.Src)

	sourceBounds := source.Bounds()
	logoWidth := logoBoxSize
	logoHeight := sourceBounds.Dy() * logoWidth / sourceBounds.Dx()
	if logoHeight > logoBoxSize {
		logoHeight = logoBoxSize
		logoWidth = sourceBounds.Dx() * logoHeight / sourceBounds.Dy()
	}
	scaled := image.NewRGBA(image.Rect(0, 0, logoWidth, logoHeight))
	for y := 0; y < logoHeight; y++ {
		for x := 0; x < logoWidth; x++ {
			sourceX := sourceBounds.Min.X + x*sourceBounds.Dx()/logoWidth
			sourceY := sourceBounds.Min.Y + y*sourceBounds.Dy()/logoHeight
			scaled.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	left := (canvasWidth - logoWidth) / 2
	top := (canvasHeight - logoHeight) / 2
	draw.Draw(canvas, image.Rect(left, top, left+logoWidth, top+logoHeight), scaled, image.Point{}, draw.Over)

	var output bytes.Buffer
	if err := jpeg.Encode(&output, canvas, &jpeg.Options{Quality: 90}); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(output.Bytes())
}

// RecallConfig configures the Recall.ai adapter.
type RecallConfig struct {
	BaseURL       string
	APIKey        string
	WebhookSecret string
	HTTPClient    *http.Client
}

// RecallProvider implements meeting capture using Recall.ai API v1.11.
type RecallProvider struct {
	apiKey        string
	webhookSecret string
	http          *httpClient
}

// NewRecallProvider creates a Recall.ai provider adapter.
func NewRecallProvider(cfg RecallConfig) *RecallProvider {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultRecallBaseURL
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	return &RecallProvider{
		apiKey:        apiKey,
		webhookSecret: strings.TrimSpace(cfg.WebhookSecret),
		http: newHTTPClient(baseURL, cfg.HTTPClient, func(req *http.Request) {
			req.Header.Set("Authorization", apiKey)
		}),
	}
}

// Name returns the stable provider key.
func (p *RecallProvider) Name() string { return "recall" }

// Configured reports whether Recall is ready for end-to-end capture and callbacks.
func (p *RecallProvider) Configured() bool {
	return p != nil && p.apiKey != "" && p.webhookSecret != ""
}

// Supports reports whether Recall supports the meeting platform.
func (p *RecallProvider) Supports(platform string) bool {
	return platform == "google_meet" || platform == "zoom" || platform == "teams" || platform == "webex"
}

// StartCapture launches or schedules a Recall bot.
func (p *RecallProvider) StartCapture(ctx context.Context, input StartCaptureInput) (*Capture, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	recordingConfig := map[string]interface{}{
		"video_mixed_mp4": nil,
		"transcript": map[string]interface{}{
			"provider": map[string]interface{}{
				"recallai_streaming": map[string]interface{}{
					"mode":          "prioritize_accuracy",
					"language_code": "auto",
				},
			},
			"diarization": map[string]interface{}{
				"use_separate_streams_when_available": true,
			},
		},
	}
	if input.RecordAudio {
		recordingConfig["video_mixed_mp4"] = map[string]interface{}{}
	}
	body := map[string]interface{}{
		"meeting_url": input.MeetingURL,
		"bot_name":    input.BotName,
		"metadata": map[string]string{
			"helpin_workspace_id": input.WorkspaceID,
			"helpin_meeting_id":   input.MeetingID,
		},
		"recording_config": recordingConfig,
	}
	if helpinRecallBotImage != "" {
		imageOutput := map[string]interface{}{
			"kind":     "jpeg",
			"b64_data": helpinRecallBotImage,
		}
		body["automatic_video_output"] = map[string]interface{}{
			"in_call_not_recording": imageOutput,
			"in_call_recording":     imageOutput,
		}
	}
	if input.JoinAt != nil {
		body["join_at"] = input.JoinAt.UTC().Format(time.RFC3339)
	}
	var response recallBot
	if err := p.http.doJSON(ctx, http.MethodPost, "/api/v1/bot/", body, &response); err != nil {
		return nil, err
	}
	status, providerStatus := normalizeRecallBot(&response)
	return &Capture{
		ProviderCaptureID: response.ID,
		ProviderStatus:    providerStatus,
		Status:            status,
	}, nil
}

// StopCapture removes an active Recall bot from the call.
func (p *RecallProvider) StopCapture(ctx context.Context, captureID string) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	return p.http.doJSON(ctx, http.MethodPost, "/api/v1/bot/"+captureID+"/leave_call/", map[string]interface{}{}, nil)
}

// GetStatus retrieves current Recall bot state for reconciliation and diagnostics.
func (p *RecallProvider) GetStatus(ctx context.Context, captureID string) (*Capture, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	var bot recallBot
	if err := p.http.doJSON(ctx, http.MethodGet, "/api/v1/bot/"+captureID+"/", nil, &bot); err != nil {
		return nil, err
	}
	status, providerStatus := normalizeRecallBot(&bot)
	capture := &Capture{ProviderCaptureID: bot.ID, ProviderStatus: providerStatus, Status: status}
	if len(bot.Recordings) > 0 {
		capture.RecordingID = bot.Recordings[0].ID
		capture.TranscriptID = bot.Recordings[0].MediaShortcuts.Transcript.ID
	}
	return capture, nil
}

// GetTranscript downloads and normalizes Recall's finalized JSON transcript.
func (p *RecallProvider) GetTranscript(ctx context.Context, captureID string) (*Transcript, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	var bot recallBot
	if err := p.http.doJSON(ctx, http.MethodGet, "/api/v1/bot/"+captureID+"/", nil, &bot); err != nil {
		return nil, err
	}
	transcriptArtifact := firstRecallTranscript(bot.Recordings)
	if transcriptArtifact.Data.DownloadURL == "" {
		return nil, fmt.Errorf("recall transcript is not ready")
	}
	var entries []recallTranscriptEntry
	if err := p.http.downloadJSON(ctx, transcriptArtifact.Data.DownloadURL, &entries); err != nil {
		return nil, err
	}
	segments := make([]TranscriptSegment, 0, len(entries))
	language := ""
	for index, entry := range entries {
		if len(entry.Words) == 0 {
			continue
		}
		words := make([]string, 0, len(entry.Words))
		for _, word := range entry.Words {
			words = append(words, word.Text)
		}
		if language == "" {
			language = entry.LanguageCode
		}
		segments = append(segments, TranscriptSegment{
			ID:           fmt.Sprintf("recall-%d", index),
			SpeakerID:    strconv.Itoa(entry.Participant.ID),
			SpeakerName:  firstNonBlank(entry.Participant.Name, "Unknown speaker"),
			Text:         strings.TrimSpace(strings.Join(words, " ")),
			StartSeconds: entry.Words[0].StartTimestamp.Relative,
			EndSeconds:   entry.Words[len(entry.Words)-1].EndTimestamp.Relative,
			Language:     entry.LanguageCode,
		})
	}
	return &Transcript{
		ProviderTranscriptID: transcriptArtifact.ID,
		Language:             language,
		Segments:             segments,
	}, nil
}

// GetRecording prefers Recall's mixed video MP4 and supports legacy mixed audio.
func (p *RecallProvider) GetRecording(ctx context.Context, captureID string) (*Recording, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	var bot map[string]interface{}
	if err := p.http.doJSON(ctx, http.MethodGet, "/api/v1/bot/"+captureID+"/", nil, &bot); err != nil {
		return nil, err
	}
	recordingID, downloadURL, contentType := findRecallRecordingDownload(bot)
	if downloadURL == "" {
		return nil, fmt.Errorf("recall recording is not ready")
	}
	return &Recording{ProviderRecordingID: recordingID, DownloadURL: downloadURL, ContentType: contentType}, nil
}

// DeleteArtifacts permanently deletes Recall media after Helpin has copied it.
func (p *RecallProvider) DeleteArtifacts(ctx context.Context, captureID string) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	return p.http.doJSON(ctx, http.MethodPost, "/api/v1/bot/"+captureID+"/delete_media/", map[string]interface{}{}, nil)
}

// VerifyWebhook verifies Recall/Svix HMAC headers using the raw request body.
func (p *RecallProvider) VerifyWebhook(headers http.Header, payload []byte) error {
	if p == nil || p.webhookSecret == "" {
		return fmt.Errorf("recall webhook verification secret is not configured")
	}
	id := firstNonBlank(headers.Get("Webhook-Id"), headers.Get("Svix-Id"))
	timestamp := firstNonBlank(headers.Get("Webhook-Timestamp"), headers.Get("Svix-Timestamp"))
	signatures := firstNonBlank(headers.Get("Webhook-Signature"), headers.Get("Svix-Signature"))
	if id == "" || timestamp == "" || signatures == "" {
		return fmt.Errorf("missing Recall webhook verification headers")
	}
	unixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Recall webhook timestamp")
	}
	if delta := time.Since(time.Unix(unixTimestamp, 0)); delta > 5*time.Minute || delta < -5*time.Minute {
		return fmt.Errorf("stale Recall webhook timestamp")
	}
	secret := strings.TrimPrefix(p.webhookSecret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("invalid Recall webhook secret")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "."))
	mac.Write(payload)
	expected := mac.Sum(nil)
	for _, candidate := range strings.Fields(signatures) {
		parts := strings.SplitN(candidate, ",", 2)
		if len(parts) != 2 || parts[0] != "v1" {
			continue
		}
		decoded, decodeErr := base64.StdEncoding.DecodeString(parts[1])
		if decodeErr == nil && hmac.Equal(decoded, expected) {
			return nil
		}
	}
	return fmt.Errorf("invalid Recall webhook signature")
}

// NormalizeWebhook converts Recall bot and transcript events to Helpin lifecycle events.
func (p *RecallProvider) NormalizeWebhook(headers http.Header, payload []byte) (*ProviderEvent, error) {
	var envelope recallWebhook
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("decode Recall webhook: %w", err)
	}
	eventID := firstNonBlank(headers.Get("Webhook-Id"), headers.Get("Svix-Id"))
	if eventID == "" {
		hash := sha256.Sum256(payload)
		eventID = hex.EncodeToString(hash[:])
	}
	providerStatus := strings.TrimPrefix(envelope.Event, "bot.")
	if envelope.Data.Data.Code != "" {
		providerStatus = envelope.Data.Data.Code
	}
	status := normalizeRecallStatus(providerStatus)
	event := &ProviderEvent{
		EventID:              eventID,
		EventType:            envelope.Event,
		ProviderCaptureID:    envelope.Data.Bot.ID,
		ProviderStatus:       providerStatus,
		Status:               status,
		ProviderRecordingID:  envelope.Data.Recording.ID,
		ProviderTranscriptID: envelope.Data.Transcript.ID,
		FailureCode:          envelope.Data.Data.SubCode,
		FailureMessage:       envelope.Data.Data.Message,
		TranscriptReady:      envelope.Event == "transcript.done",
	}
	if parsed, err := time.Parse(time.RFC3339Nano, envelope.Data.Data.UpdatedAt); err == nil {
		event.OccurredAt = &parsed
	}
	if envelope.Event == "transcript.processing" {
		event.Status = "finalizing"
	}
	if envelope.Event == "transcript.done" {
		event.Status = "processing"
	}
	if envelope.Event == "transcript.failed" {
		event.Status = "failed"
	}
	return event, nil
}

type recallBot struct {
	ID            string               `json:"id"`
	StatusChanges []recallStatusChange `json:"status_changes"`
	Recordings    []recallRecording    `json:"recordings"`
}

type recallStatusChange struct {
	Code      string `json:"code"`
	SubCode   string `json:"sub_code"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type recallRecording struct {
	ID             string `json:"id"`
	MediaShortcuts struct {
		Transcript recallArtifact `json:"transcript"`
	} `json:"media_shortcuts"`
}

type recallArtifact struct {
	ID   string `json:"id"`
	Data struct {
		DownloadURL string `json:"download_url"`
	} `json:"data"`
}

type recallWebhook struct {
	Event string `json:"event"`
	Data  struct {
		Data struct {
			Code      string `json:"code"`
			SubCode   string `json:"sub_code"`
			Message   string `json:"message"`
			UpdatedAt string `json:"updated_at"`
		} `json:"data"`
		Bot struct {
			ID string `json:"id"`
		} `json:"bot"`
		Recording struct {
			ID string `json:"id"`
		} `json:"recording"`
		Transcript struct {
			ID string `json:"id"`
		} `json:"transcript"`
	} `json:"data"`
}

type recallTranscriptEntry struct {
	Participant struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"participant"`
	LanguageCode string `json:"language_code"`
	Words        []struct {
		Text           string `json:"text"`
		StartTimestamp struct {
			Relative float64 `json:"relative"`
		} `json:"start_timestamp"`
		EndTimestamp struct {
			Relative float64 `json:"relative"`
		} `json:"end_timestamp"`
	} `json:"words"`
}

func normalizeRecallBot(bot *recallBot) (string, string) {
	if bot == nil || len(bot.StatusChanges) == 0 {
		return "joining", "ready"
	}
	latest := bot.StatusChanges[len(bot.StatusChanges)-1]
	return normalizeRecallStatus(latest.Code), latest.Code
}

func normalizeRecallStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "joining_call":
		return "joining"
	case "in_waiting_room":
		return "waiting"
	case "in_call_not_recording", "recording_permission_allowed", "in_call_recording":
		return "recording"
	case "recording_done", "call_ended", "done", "transcript.processing":
		return "finalizing"
	case "analysis_done", "transcript.done":
		return "processing"
	case "fatal", "analysis_failed", "recording_permission_denied", "transcript.failed":
		return "failed"
	case "media_expired":
		return "ready"
	default:
		return "joining"
	}
}

func firstRecallTranscript(recordings []recallRecording) recallArtifact {
	for _, recording := range recordings {
		if recording.MediaShortcuts.Transcript.Data.DownloadURL != "" {
			return recording.MediaShortcuts.Transcript
		}
	}
	return recallArtifact{}
}

func findRecallRecordingDownload(bot map[string]interface{}) (string, string, string) {
	recordings, _ := bot["recordings"].([]interface{})
	for _, item := range recordings {
		recording, _ := item.(map[string]interface{})
		recordingID, _ := recording["id"].(string)
		shortcuts, _ := recording["media_shortcuts"].(map[string]interface{})
		media := []struct{ key, contentType string }{
			{key: "video_mixed", contentType: "video/mp4"},
			{key: "video_mixed_mp4", contentType: "video/mp4"},
			{key: "audio_mixed", contentType: "audio/mpeg"},
			{key: "audio_mixed_mp3", contentType: "audio/mpeg"},
		}
		for _, candidate := range media {
			artifact, _ := shortcuts[candidate.key].(map[string]interface{})
			data, _ := artifact["data"].(map[string]interface{})
			if downloadURL, _ := data["download_url"].(string); downloadURL != "" {
				return recordingID, downloadURL, candidate.contentType
			}
		}
	}
	return "", "", ""
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
