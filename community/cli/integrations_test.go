package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeadlessIntegrationsWriteSecretsFromFilesAndEnvironment(t *testing.T) {
	a, _ := testApp(t)
	dir := t.TempDir()
	original := strings.ReplaceAll(template(t), "=GENERATE\n", "=generated-secret\n")
	put(t, filepath.Join(dir, ".env"), original)
	keyFile := filepath.Join(t.TempDir(), "ai-key")
	put(t, keyFile, "sk-or-v1-abcdef123456\n")
	t.Setenv(smtpPasswordEnv, "pa$$ word#1")
	o := options{yes: true, mode: "local", smtpHost: "smtp.example.com", smtpUser: "mailer", smtpFrom: "Helpin <help@example.com>", aiProvider: "openrouter", aiKeyFile: keyFile}
	if err := a.configuration(&o, nil); err != nil {
		t.Fatal(err)
	}
	if err := a.integrations(&o, envValues(original)); err != nil {
		t.Fatal(err)
	}
	if err := writeConfiguration(dir, o); err != nil {
		t.Fatal(err)
	}
	if err := writeIntegrations(dir, o); err != nil {
		t.Fatal(err)
	}
	values, _ := readEnv(dir)
	want := map[string]string{
		"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "587", "SMTP_USERNAME": "mailer", "SMTP_TLS_MODE": "starttls",
		"SMTP_FROM": "'Helpin <help@example.com>'", "SMTP_PASSWORD": "'pa$$ word#1'",
		"OPENROUTER_API_KEY": "sk-or-v1-abcdef123456", "JWT_SECRET": "generated-secret", "OPENAI_API_KEY": "",
	}
	for key, value := range want {
		if values[key] != value {
			t.Errorf("%s=%q, want %q", key, values[key], value)
		}
	}
	info, _ := os.Stat(filepath.Join(dir, ".env"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("secrets are not private")
	}
}

func TestHeadlessIntegrationsWithoutFlagsChangeNothing(t *testing.T) {
	a, _ := testApp(t)
	dir := t.TempDir()
	original := template(t) + "SMTP_PASSWORD=kept\n"
	put(t, filepath.Join(dir, ".env"), original)
	o := options{yes: true}
	if err := a.integrations(&o, envValues(original)); err != nil {
		t.Fatal(err)
	}
	if o.smtp != nil || o.ai != nil || o.meeting != nil || o.google != nil {
		t.Fatal("headless run collected integrations without flags")
	}
	if err := writeIntegrations(dir, o); err != nil {
		t.Fatal(err)
	}
	if get(t, filepath.Join(dir, ".env")) != original {
		t.Fatal("configuration changed")
	}
}

func TestHeadlessIntegrationRejectsMissingOrUnsafeValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		o    options
		env  string
	}{
		{"ai provider without key", options{aiProvider: "openai"}, ""},
		{"unknown provider", options{aiProvider: "gemini"}, "key-12345678"},
		{"username without password", options{smtpHost: "smtp.example.com", smtpUser: "u", smtpFrom: "a@example.com"}, ""},
		{"missing sender", options{smtpHost: "smtp.example.com"}, ""},
		{"host with scheme", options{smtpHost: "smtp://example.com", smtpFrom: "a@example.com"}, ""},
		{"credentials without TLS", options{smtpHost: "smtp.example.com", smtpUser: "u", smtpFrom: "a@example.com", smtpTLS: "none"}, "secret"},
		{"bad port", options{smtpHost: "smtp.example.com", smtpFrom: "a@example.com", smtpPort: "70000"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testApp(t)
			t.Setenv(aiKeyEnv, tc.env)
			t.Setenv(smtpPasswordEnv, tc.env)
			tc.o.yes = true
			if err := a.integrations(&tc.o, map[string]string{}); err == nil {
				t.Fatal("accepted invalid integration settings")
			}
		})
	}
}

func TestInteractiveIntegrationsReadSecretsHiddenAndKeepStoredValues(t *testing.T) {
	a, out := testApp(t)
	a.interactive = true
	// SMTP host, port, TLS, username, sender; then the AI provider.
	a.in = bufio.NewReader(strings.NewReader("mail.internal\n2525\ntls\nrelay\nops@example.com\nanthropic\nskip\nskip\n"))
	var prompts []string
	a.secret = func(label string) (string, error) {
		prompts = append(prompts, label)
		if strings.Contains(label, "SMTP") {
			return "", nil // keep the stored password
		}
		return "sk-ant-secret-value", nil
	}
	old := map[string]string{"SMTP_PASSWORD": "stored"}
	o := options{}
	if err := a.integrations(&o, old); err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 2 || !strings.Contains(prompts[0], "blank keeps") {
		t.Fatal(prompts)
	}
	if o.smtp == nil || o.smtp.host != "mail.internal" || o.smtp.port != "2525" || o.smtp.passwordSet {
		t.Fatalf("%+v", o.smtp)
	}
	if o.ai == nil || o.ai.provider != "anthropic" || o.ai.key != "sk-ant-secret-value" {
		t.Fatalf("%+v", o.ai)
	}
	if strings.Contains(out.String(), "sk-ant-secret-value") {
		t.Fatal("secret echoed")
	}
	dir := t.TempDir()
	put(t, filepath.Join(dir, ".env"), "SMTP_HOST=\nSMTP_PASSWORD=stored\n")
	if err := writeIntegrations(dir, o); err != nil {
		t.Fatal(err)
	}
	values, _ := readEnv(dir)
	if values["SMTP_PASSWORD"] != "stored" || values["ANTHROPIC_API_KEY"] != "sk-ant-secret-value" || values["SMTP_HOST"] != "mail.internal" {
		t.Fatal(values)
	}
}

func TestInteractiveIntegrationsCanBeSkipped(t *testing.T) {
	a, _ := testApp(t)
	a.interactive = true
	a.in = bufio.NewReader(strings.NewReader("\n\n\n\n"))
	a.secret = func(string) (string, error) { t.Fatal("prompted for a secret after skipping"); return "", nil }
	o := options{}
	if err := a.integrations(&o, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if o.smtp != nil || o.ai != nil || o.meeting != nil || o.google != nil {
		t.Fatal("skipped integrations were collected")
	}
}

func TestSecretsAreNeverReadWithoutATerminal(t *testing.T) {
	a, _ := testApp(t)
	a.secret = func(string) (string, error) { return "leaked", nil }
	if _, err := a.readSecret("API key"); err == nil {
		t.Fatal("read a hidden value without an interactive terminal")
	}
}

func TestEnvValueQuotingRejectsLineBreaks(t *testing.T) {
	for _, value := range []string{"a\nb", "it's", "a\rb"} {
		if _, err := envValue(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	if got, _ := envValue("$HOME"); got != "'$HOME'" {
		t.Fatal(got)
	}
	if plainEnv("'x y'") != "x y" {
		t.Fatal("plainEnv did not unquote")
	}
}

func TestDoctorReportsCapabilitiesWithInternalSecret(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/config":
			fmt.Fprint(w, `{"public_widget_url":"http://expected.test"}`)
		case "/api/instance/capabilities":
			auth = r.Header.Get("Authorization")
			fmt.Fprint(w, `{"edition":"community","capabilities":[
				{"key":"email_outbound","status":"unable_to_verify","detail":"Application email is configured but no test email has been sent.","required":false,"action":{"kind":"server_config","label":"Send a test email"}},
				{"key":"ai_chat","status":"ready","detail":"1 shared AI connection(s) passed their last test.","required":false},
				{"key":"workers","status":"needs_setup","detail":"No background worker is polling for jobs.","required":true,"action":{"kind":"server_config","label":"Start the worker service"}}]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	a, out := testApp(t)
	failures := a.reportCapabilities(server.Client(), map[string]string{"DASHBOARD_PORT": port, "INTERNAL_API_SECRET": "internal-secret"})
	if auth != "Bearer internal-secret" {
		t.Fatal("capabilities request was not authenticated:", auth)
	}
	report := out.String()
	for _, want := range []string{"! Application email: unable to verify", "Next: Send a test email", "✓ AI chat: ready", "✗ Background workers: needs setup"} {
		if !strings.Contains(report, want) {
			t.Fatalf("missing %q in:\n%s", want, report)
		}
	}
	if len(failures) != 1 || failures[0] != "Background workers" {
		t.Fatal(failures)
	}
}

func TestDoctorSkipsCapabilitiesOnOlderServers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer server.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	a, out := testApp(t)
	if failures := a.reportCapabilities(server.Client(), map[string]string{"DASHBOARD_PORT": port, "INTERNAL_API_SECRET": "x"}); len(failures) != 0 {
		t.Fatal(failures)
	}
	if !strings.Contains(out.String(), "Skipped") {
		t.Fatal(out.String())
	}
	if _, err := instanceCapabilities(server.Client(), map[string]string{"DASHBOARD_PORT": port}); err == nil || errors.Is(err, errCapabilitiesUnsupported) {
		t.Fatal("missing secret must be reported", err)
	}
}
