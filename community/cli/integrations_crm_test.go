package main

import (
	"bufio"
	"path/filepath"
	"strings"
	"testing"
)

const recallSecret = "whsec_c2VjcmV0LXNpZ25pbmcta2V5LWZvci10ZXN0cw=="

func TestHeadlessMeetingAndGoogleSettingsAreWritten(t *testing.T) {
	a, out := testApp(t)
	dir := t.TempDir()
	original := strings.ReplaceAll(template(t), "=GENERATE\n", "=generated-secret\n")
	put(t, filepath.Join(dir, ".env"), original)
	googleSecret := filepath.Join(t.TempDir(), "google-secret")
	put(t, googleSecret, "GOCSPX-client-secret\n")
	t.Setenv(meetingKeyEnv, "recall-api-key-123")
	t.Setenv(meetingSecretEnv, recallSecret)
	o := options{yes: true, mode: "server", domain: "helpin.example.com", meetingProvider: "recall",
		googleClientID: "123-abc.apps.googleusercontent.com", googleClientSecretFile: googleSecret}
	if err := a.integrations(&o, envValues(original)); err != nil {
		t.Fatal(err)
	}
	if err := writeIntegrations(dir, o); err != nil {
		t.Fatal(err)
	}
	values, _ := readEnv(dir)
	want := map[string]string{
		"CRM_MEETING_CAPTURE_PROVIDER": "recall", "RECALL_API_KEY": "recall-api-key-123", "RECALL_WEBHOOK_SECRET": recallSecret,
		"GMAIL_CLIENT_ID": "123-abc.apps.googleusercontent.com", "GMAIL_CLIENT_SECRET": "GOCSPX-client-secret", "GMAIL_OAUTH_REDIRECT_URL": "",
		"VEXA_API_KEY": "", "JWT_SECRET": "generated-secret",
	}
	for key, value := range want {
		if values[key] != value {
			t.Errorf("%s=%q, want %q", key, values[key], value)
		}
	}
	report := out.String()
	for _, text := range []string{
		"https://helpin.example.com/api/webhooks/meeting-capture/recall",
		"https://helpin.example.com/api/crm/email/oauth/callback",
	} {
		if !strings.Contains(report, text) {
			t.Errorf("output does not name %s:\n%s", text, report)
		}
	}
	if strings.Contains(report, "recall-api-key-123") || strings.Contains(report, "GOCSPX") {
		t.Fatal("secret echoed")
	}
}

func TestHeadlessVexaUsesSelfHostedURLAndWarnsWithoutHTTPS(t *testing.T) {
	a, out := testApp(t)
	t.Setenv(meetingKeyEnv, "vexa-api-key-123")
	t.Setenv(meetingSecretEnv, "0123456789abcdef0123")
	o := options{yes: true, mode: "local", port: 8085, meetingProvider: "vexa", vexaURL: "http://vexa.internal:8056/"}
	if err := a.integrations(&o, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if o.meeting == nil || o.meeting.vexaURL != "http://vexa.internal:8056" || !o.meeting.keySet || !o.meeting.secretSet {
		t.Fatalf("%+v", o.meeting)
	}
	values := map[string]string{}
	for _, value := range crmIntegrationValues(o) {
		values[value[0]] = value[1]
	}
	if values["VEXA_BASE_URL"] != "http://vexa.internal:8056" || values["VEXA_API_KEY"] != "vexa-api-key-123" || values["CRM_MEETING_CAPTURE_PROVIDER"] != "vexa" {
		t.Fatal(values)
	}
	if !strings.Contains(out.String(), "public https address") {
		t.Fatal(out.String())
	}
}

func TestHeadlessMeetingKeepsStoredSecrets(t *testing.T) {
	a, _ := testApp(t)
	old := map[string]string{"RECALL_API_KEY": "stored-key-123", "RECALL_WEBHOOK_SECRET": recallSecret}
	o := options{yes: true, meetingProvider: "recall"}
	if err := a.integrations(&o, old); err != nil {
		t.Fatal(err)
	}
	values := crmIntegrationValues(o)
	if len(values) != 1 || values[0] != [2]string{"CRM_MEETING_CAPTURE_PROVIDER", "recall"} {
		t.Fatal(values)
	}
}

func TestHeadlessMeetingAndGoogleRejectMissingOrInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name        string
		o           options
		key, secret string
	}{
		{"unknown provider", options{meetingProvider: "zoom"}, "key-12345678", recallSecret},
		{"missing api key", options{meetingProvider: "recall"}, "", recallSecret},
		{"missing webhook secret", options{meetingProvider: "recall"}, "key-12345678", ""},
		{"recall secret is not a signing secret", options{meetingProvider: "recall"}, "key-12345678", "not base64!!"},
		{"short vexa secret", options{meetingProvider: "vexa"}, "key-12345678", "short-secret"},
		{"vexa url without scheme", options{meetingProvider: "vexa", vexaURL: "vexa.example.com"}, "key-12345678", "0123456789abcdef0123"},
		{"google client id", options{googleClientID: "not-a-client-id"}, "", ""},
		{"google without secret", options{googleClientID: "123-abc.apps.googleusercontent.com"}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testApp(t)
			t.Setenv(meetingKeyEnv, tc.key)
			t.Setenv(meetingSecretEnv, tc.secret)
			t.Setenv(googleSecretEnv, "")
			tc.o.yes = true
			if err := a.integrations(&tc.o, map[string]string{}); err == nil {
				t.Fatal("accepted invalid integration settings")
			}
		})
	}
}

func TestInteractiveMeetingAndGooglePromptHiddenSecrets(t *testing.T) {
	a, out := testApp(t)
	a.interactive = true
	// SMTP skip, AI skip, meeting provider, Vexa URL (default), Google client ID.
	a.in = bufio.NewReader(strings.NewReader("skip\nskip\nvexa\n\n123-abc.apps.googleusercontent.com\n"))
	var prompts []string
	a.secret = func(label string) (string, error) {
		prompts = append(prompts, label)
		switch {
		case strings.Contains(label, "API key"):
			return "vexa-api-key-123", nil
		case strings.Contains(label, "webhook secret"):
			return "0123456789abcdef0123", nil
		default:
			return "", nil // keep the stored Google secret
		}
	}
	old := map[string]string{"GMAIL_CLIENT_SECRET": "stored-google-secret", "APP_BASE_URL": "https://helpin.example.com"}
	o := options{}
	if err := a.integrations(&o, old); err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 3 || !strings.Contains(prompts[2], "blank keeps") {
		t.Fatal(prompts)
	}
	if o.meeting == nil || o.meeting.provider != "vexa" || o.meeting.vexaURL != defaultVexaURL {
		t.Fatalf("%+v", o.meeting)
	}
	if o.google == nil || o.google.secretSet {
		t.Fatalf("%+v", o.google)
	}
	if strings.Contains(out.String(), "vexa-api-key-123") {
		t.Fatal("secret echoed")
	}
}
