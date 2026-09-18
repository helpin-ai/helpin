package widgetorigin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectionDiagnosticsExcludeCredentials(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	r := httptest.NewRequest("POST", "/widget/session?session_token=SECRET_SESSION&widget_key=SECRET_KEY", strings.NewReader(`{"session_token":"SECRET_BODY"}`))
	r.Header.Set("Origin", "tauri://localhost")
	r.Header.Set("X-Session-Token", "SECRET_HEADER")
	LogRejection(context.Background(), r, &DeniedError{InstallationID: "installation-one"})
	if strings.Contains(output.String(), "SECRET") {
		t.Fatal("credentials in diagnostic log")
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"origin": "tauri://localhost", "installation_id": "installation-one", "path": "/widget/session", "method": "POST"} {
		if record[key] != want {
			t.Errorf("%s = %v, want %s", key, record[key], want)
		}
	}
	output.Reset()
	r.Header.Set("Origin", "https://user:SECRET_PASSWORD@localhost/?token=SECRET_QUERY")
	LogRejection(context.Background(), r, errors.New("SECRET_INTERNAL_ERROR"))
	if strings.Contains(output.String(), "SECRET") {
		t.Fatal("malformed origin or internal error leaked credentials")
	}
	if !strings.Contains(output.String(), "[invalid origin]") {
		t.Fatal("missing malformed-origin diagnostic")
	}
}

func TestDiagnosticOrigin(t *testing.T) {
	for _, origin := range []string{"", "null", "tauri://localhost", "https://tauri.localhost", "app://localhost"} {
		if got := diagnosticOrigin(origin); got != origin {
			t.Errorf("origin %q became %q", origin, got)
		}
	}
	for _, origin := range []string{"https://localhost/path", "https://localhost?token=secret", "https://localhost#secret", "https://user:secret@localhost", "https://localhost\nsecret", "https://" + strings.Repeat("a", 513)} {
		if got := diagnosticOrigin(origin); got != "[invalid origin]" {
			t.Errorf("unsafe origin logged: %q", got)
		}
	}
}
