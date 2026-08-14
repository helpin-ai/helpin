package meetingcapture

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	})
	if err != nil {
		t.Fatalf("StartCapture: %v", err)
	}
	if capture.ProviderCaptureID != "bot-1" || capture.Status != "joining" {
		t.Fatalf("capture = %#v", capture)
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

func TestVexaProviderVerifyWebhook(t *testing.T) {
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
