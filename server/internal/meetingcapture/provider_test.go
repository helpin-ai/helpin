package meetingcapture

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecallProviderStartCaptureUsesRecallContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/bot/" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "recall-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["meeting_url"] != "https://meet.google.com/abc-defg-hij" {
			t.Fatalf("meeting_url = %#v", body["meeting_url"])
		}
		recordingConfig, _ := body["recording_config"].(map[string]interface{})
		if _, ok := recordingConfig["video_mixed_mp4"].(map[string]interface{}); !ok {
			t.Fatalf("video_mixed_mp4 = %#v, want enabled object", recordingConfig["video_mixed_mp4"])
		}
		if _, exists := recordingConfig["audio_mixed_mp3"]; exists {
			t.Fatalf("audio_mixed_mp3 should not be requested for new video captures")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"bot-1","status_changes":[{"code":"joining_call"}]}`))
	}))
	defer server.Close()

	provider := NewRecallProvider(RecallConfig{BaseURL: server.URL, APIKey: "recall-key", WebhookSecret: "whsec_dGVzdA==", HTTPClient: server.Client()})
	capture, err := provider.StartCapture(context.Background(), StartCaptureInput{
		WorkspaceID:     "workspace-1",
		MeetingID:       "meeting-1",
		MeetingURL:      "https://meet.google.com/abc-defg-hij",
		Platform:        "google_meet",
		NativeMeetingID: "abc-defg-hij",
		BotName:         "Helpin Notetaker",
		RecordAudio:     true,
	})
	if err != nil {
		t.Fatalf("StartCapture: %v", err)
	}
	if capture.ProviderCaptureID != "bot-1" || capture.Status != "joining" {
		t.Fatalf("capture = %#v", capture)
	}
}

func TestRecallProviderGetRecordingPrefersVideoAndFallsBackToAudio(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		response    string
		wantURL     string
		contentType string
	}{
		{
			name:     "mixed video",
			response: `{"recordings":[{"id":"recording-1","media_shortcuts":{"video_mixed":{"data":{"download_url":"https://media.example/video.mp4"}},"audio_mixed":{"data":{"download_url":"https://media.example/audio.mp3"}}}}]}`,
			wantURL:  "https://media.example/video.mp4", contentType: "video/mp4",
		},
		{
			name:     "legacy mixed audio",
			response: `{"recordings":[{"id":"recording-2","media_shortcuts":{"audio_mixed":{"data":{"download_url":"https://media.example/audio.mp3"}}}}]}`,
			wantURL:  "https://media.example/audio.mp3", contentType: "audio/mpeg",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/bot/bot-1/" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.response))
			}))
			defer server.Close()

			provider := NewRecallProvider(RecallConfig{BaseURL: server.URL, APIKey: "recall-key", WebhookSecret: "whsec_dGVzdA==", HTTPClient: server.Client()})
			recording, err := provider.GetRecording(context.Background(), "bot-1")
			if err != nil {
				t.Fatalf("GetRecording: %v", err)
			}
			if recording.ProviderRecordingID == "" || recording.DownloadURL != test.wantURL || recording.ContentType != test.contentType {
				t.Fatalf("recording = %#v", recording)
			}
		})
	}
}
func TestVexaProviderStartCaptureUsesProviderNeutralID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/bots" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "vexa-key" {
			t.Fatalf("X-API-Key = %q", got)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["native_meeting_id"] != "abc-defg-hij" {
			t.Fatalf("unexpected body %#v, err=%v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"requested"}`))
	}))
	defer server.Close()

	provider := NewVexaProvider(VexaConfig{BaseURL: server.URL, APIKey: "vexa-key", WebhookSecret: "vexa-secret", HTTPClient: server.Client()})
	capture, err := provider.StartCapture(context.Background(), StartCaptureInput{
		MeetingURL:      "https://meet.google.com/abc-defg-hij",
		Platform:        "google_meet",
		NativeMeetingID: "abc-defg-hij",
		BotName:         "Helpin Notetaker",
	})
	if err != nil {
		t.Fatalf("StartCapture: %v", err)
	}
	if capture.ProviderCaptureID != "google_meet:abc-defg-hij" || capture.Status != "joining" {
		t.Fatalf("capture = %#v", capture)
	}
}

func TestRecallProviderVerifyWebhook(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"event":"transcript.done"}`)
	eventID := "event-1"
	timestamp := time.Now().Unix()
	secretBytes := []byte("recall-webhook-secret")
	secret := "whsec_" + base64.StdEncoding.EncodeToString(secretBytes)
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(eventID + "." + jsonNumber(timestamp) + "."))
	mac.Write(payload)
	headers := http.Header{
		"Webhook-Id":        []string{eventID},
		"Webhook-Timestamp": []string{jsonNumber(timestamp)},
		"Webhook-Signature": []string{"v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))},
	}
	provider := NewRecallProvider(RecallConfig{APIKey: "key", WebhookSecret: secret})
	if err := provider.VerifyWebhook(headers, payload); err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if err := provider.VerifyWebhook(headers, []byte(`{"tampered":true}`)); err == nil {
		t.Fatal("expected tampered payload to fail verification")
	}
}

func TestRecallProviderNormalizesTranscriptDoneWebhook(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"event":"transcript.done",
		"data":{
			"bot":{"id":"bot-1"},
			"recording":{"id":"recording-1"},
			"transcript":{"id":"transcript-1"}
		}
	}`)
	provider := NewRecallProvider(RecallConfig{APIKey: "key", WebhookSecret: "whsec_dGVzdA=="})
	event, err := provider.NormalizeWebhook(http.Header{"Webhook-Id": []string{"event-1"}}, payload)
	if err != nil {
		t.Fatalf("NormalizeWebhook: %v", err)
	}
	if event.EventID != "event-1" || event.ProviderCaptureID != "bot-1" ||
		event.ProviderRecordingID != "recording-1" || event.ProviderTranscriptID != "transcript-1" ||
		!event.TranscriptReady || event.Status != "processing" {
		t.Fatalf("event = %#v", event)
	}
}

func TestVexaProviderVerifyLegacyWebhook(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"event_type":"meeting.status_change"}`)
	headers := http.Header{"Authorization": []string{"Bearer vexa-secret"}}
	provider := NewVexaProvider(VexaConfig{APIKey: "key", WebhookSecret: "vexa-secret"})
	if err := provider.VerifyWebhook(headers, payload); err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	headers.Set("Authorization", "Bearer wrong-secret")
	if err := provider.VerifyWebhook(headers, payload); err == nil {
		t.Fatal("expected wrong bearer secret to fail verification")
	}
}

func TestVexaProviderNormalizesOfficialStatusWebhook(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"event_type":"meeting.status_change",
		"meeting":{"platform":"google_meet","native_meeting_id":"abc-defg-hij","status":"completed"},
		"status_change":{"from":"stopping","to":"completed","reason":"self_initiated_leave","timestamp":"2026-08-14T12:00:00Z"}
	}`)
	provider := NewVexaProvider(VexaConfig{APIKey: "key", WebhookSecret: "secret"})
	event, err := provider.NormalizeWebhook(http.Header{}, payload)
	if err != nil {
		t.Fatalf("NormalizeWebhook: %v", err)
	}
	if event.ProviderCaptureID != "google_meet:abc-defg-hij" {
		t.Fatalf("ProviderCaptureID = %q", event.ProviderCaptureID)
	}
	if !event.TranscriptReady || event.Status != "processing" {
		t.Fatalf("event = %#v", event)
	}
}

func TestVexaProviderVerifySignedWebhook(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"event_type":"meeting.completed"}`)
	timestamp := jsonNumber(time.Now().Unix())
	mac := hmac.New(sha256.New, []byte("vexa-secret"))
	mac.Write([]byte(timestamp + "."))
	mac.Write(payload)
	headers := http.Header{
		"X-Webhook-Timestamp": []string{timestamp},
		"X-Webhook-Signature": []string{"sha256=" + hex.EncodeToString(mac.Sum(nil))},
	}
	provider := NewVexaProvider(VexaConfig{APIKey: "key", WebhookSecret: "vexa-secret"})
	if err := provider.VerifyWebhook(headers, payload); err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if err := provider.VerifyWebhook(headers, []byte(`{"tampered":true}`)); err == nil {
		t.Fatal("expected tampered payload to fail verification")
	}
	headers.Set("X-Webhook-Timestamp", jsonNumber(time.Now().Add(-6*time.Minute).Unix()))
	if err := provider.VerifyWebhook(headers, payload); err == nil {
		t.Fatal("expected stale timestamp to fail verification")
	}
}

func TestVexaProviderNormalizesV1CompletedWebhook(t *testing.T) {
	t.Parallel()
	payload := []byte(`{
		"event_id":"evt-1",
		"event_type":"meeting.completed",
		"api_version":"webhook.v1",
		"created_at":"2026-08-14T12:00:00Z",
		"data":{"meeting":{"platform":"google_meet","native_meeting_id":"abc-defg-hij","status":"completed"}}
	}`)
	provider := NewVexaProvider(VexaConfig{APIKey: "key", WebhookSecret: "secret"})
	event, err := provider.NormalizeWebhook(http.Header{}, payload)
	if err != nil {
		t.Fatalf("NormalizeWebhook: %v", err)
	}
	if event.EventID != "evt-1" || event.ProviderCaptureID != "google_meet:abc-defg-hij" || !event.TranscriptReady || event.Status != "processing" {
		t.Fatalf("event = %#v", event)
	}
}

func TestVexaProviderGetsCurrentTranscriptSegments(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/transcripts/google_meet/abc-defg-hij" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"segments":[{"segment_id":"segment-1","speaker":"Azhar","text":"Ship it","start":1.5,"end":3.25,"language":"en","confidence":0.98,"completed":true}]}`))
	}))
	defer server.Close()

	provider := NewVexaProvider(VexaConfig{BaseURL: server.URL, APIKey: "key", WebhookSecret: "secret", HTTPClient: server.Client()})
	transcript, err := provider.GetTranscript(context.Background(), "google_meet:abc-defg-hij")
	if err != nil {
		t.Fatalf("GetTranscript: %v", err)
	}
	if len(transcript.Segments) != 1 || transcript.Segments[0].StartSeconds != 1.5 || transcript.Segments[0].EndSeconds != 3.25 {
		t.Fatalf("transcript = %#v", transcript)
	}
}

func TestVexaProviderAcceptsLegacyTeamsURLWithoutPasscode(t *testing.T) {
	t.Parallel()
	meetingURL := "https://teams.microsoft.com/l/meetup-join/19%3ameeting_example%40thread.v2/0?context=example"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["meeting_url"] != meetingURL {
			t.Fatalf("meeting_url = %#v", body["meeting_url"])
		}
		_, _ = w.Write([]byte(`{"status":"requested"}`))
	}))
	defer server.Close()

	provider := NewVexaProvider(VexaConfig{BaseURL: server.URL, APIKey: "key", WebhookSecret: "secret", HTTPClient: server.Client()})
	_, err := provider.StartCapture(context.Background(), StartCaptureInput{
		MeetingURL: meetingURL, Platform: "teams", NativeMeetingID: "19:meeting_example@thread.v2",
	})
	if err != nil {
		t.Fatalf("StartCapture: %v", err)
	}
}

func TestFindVexaRecordingSupportsNumericIDs(t *testing.T) {
	t.Parallel()
	recordingID, mediaID := findVexaRecording(map[string]interface{}{
		"recordings": []interface{}{map[string]interface{}{
			"id":          float64(906238426347),
			"media_files": []interface{}{map[string]interface{}{"id": float64(906238426348)}},
		}},
	})
	if recordingID != "906238426347" || mediaID != "906238426348" {
		t.Fatalf("recording IDs = %q, %q", recordingID, mediaID)
	}
}

func jsonNumber(value int64) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
