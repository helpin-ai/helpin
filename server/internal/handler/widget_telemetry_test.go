package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/observability"
)

func TestWidgetTelemetryValidationAndPrivacy(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"valid", `{"widget_key":"installation","sdk_release":"local","stage":"storage","outcome":"timeout","duration_ms":30000}`, 204},
		{"customer content rejected", `{"stage":"storage","outcome":"error","message":"customer secret"}`, 400},
		{"unbounded stage rejected", `{"stage":"customer-id","outcome":"error"}`, 400},
		{"unbounded outcome rejected", `{"stage":"upload","outcome":"secret"}`, 400},
		{"negative duration", `{"stage":"upload","outcome":"error","duration_ms":-1}`, 400},
		{"excessive duration", `{"stage":"upload","outcome":"error","duration_ms":600001}`, 400},
		{"trailing body", `{"stage":"upload","outcome":"error"} {}`, 400},
		{"invalid release", `{"stage":"upload","outcome":"error","sdk_release":"customer-secret"}`, 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := observability.NewMetrics()
			r := httptest.NewRequest("POST", "/widget/telemetry", strings.NewReader(tt.body))
			r.Header.Set("User-Agent", "Firefox/155.0")
			w := httptest.NewRecorder()
			WidgetTelemetry(m)(w, r)
			if w.Code != tt.status {
				t.Fatalf("status=%d want=%d", w.Code, tt.status)
			}
			scrape := httptest.NewRecorder()
			m.Handler().ServeHTTP(scrape, httptest.NewRequest("GET", "/metrics", nil))
			text := scrape.Body.String()
			if strings.Contains(text, "installation") || strings.Contains(text, "secret") {
				t.Fatal("sensitive values reached metrics")
			}
			if tt.status == 204 && !strings.Contains(text, `helpin_widget_events_total{browser="firefox",outcome="timeout",stage="storage"} 1`) {
				t.Fatal("missing expected event")
			}
			if tt.status == 400 && strings.Contains(text, "helpin_widget_events_total{") {
				t.Fatal("invalid event recorded")
			}
		})
	}
}

func TestWidgetConnectionTelemetry(t *testing.T) {
	for _, tt := range []struct {
		body   string
		status int
	}{
		{`{"stage":"connection","outcome":"closed","close_code":1000}`, 204},
		{`{"stage":"connection","outcome":"error","close_code":1006}`, 204},
		{`{"stage":"connection","outcome":"error"}`, 204},
		{`{"stage":"connection","outcome":"error","close_code":5000}`, 400},
		{`{"stage":"connection","outcome":"error","close_code":-1}`, 400},
		{`{"stage":"upload","outcome":"closed"}`, 400},
		{`{"stage":"upload","outcome":"error","close_code":1000}`, 400},
	} {
		t.Run(tt.body, func(t *testing.T) {
			w := httptest.NewRecorder()
			WidgetTelemetry(observability.NewMetrics())(w, httptest.NewRequest("POST", "/widget/telemetry", strings.NewReader(tt.body)))
			if w.Code != tt.status {
				t.Fatalf("status=%d want=%d", w.Code, tt.status)
			}
		})
	}
}
