package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/observability"
)

// WidgetTelemetry accepts a small allowlisted event behind origin and rate-limit middleware.
// Deliberately excludes arbitrary metadata, errors, URLs and customer identifiers.
func WidgetTelemetry(metrics *observability.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var event struct {
			CloseCode  *int    `json:"close_code"`
			WidgetKey  string  `json:"widget_key"`
			SDKRelease string  `json:"sdk_release"`
			Stage      string  `json:"stage"`
			Outcome    string  `json:"outcome"`
			DurationMS float64 `json:"duration_ms"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			writeError(w, 400, "invalid telemetry")
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			writeError(w, 400, "invalid telemetry")
			return
		}
		switch event.Stage {
		case "initialization", "storage", "confirmation", "upload", "connection", "message":
		default:
			writeError(w, 400, "invalid stage")
			return
		}
		switch event.Outcome {
		case "success", "error", "timeout", "cancelled", "not_connected", "closed":
		default:
			writeError(w, 400, "invalid outcome")
			return
		}
		if event.Outcome == "closed" && event.Stage != "connection" {
			writeError(w, 400, "invalid outcome for stage")
			return
		}
		if event.CloseCode != nil && (event.Stage != "connection" || *event.CloseCode < 0 || *event.CloseCode > 4999) {
			writeError(w, 400, "invalid close code")
			return
		}
		if math.IsNaN(event.DurationMS) || math.IsInf(event.DurationMS, 0) || event.DurationMS < 0 || event.DurationMS > 600000 {
			writeError(w, 400, "invalid duration")
			return
		}
		if event.SDKRelease != "" && event.SDKRelease != "local" {
			if ok, _ := regexp.MatchString("^[a-f0-9]{7,40}$", event.SDKRelease); !ok {
				writeError(w, 400, "invalid SDK release")
				return
			}
		}
		browser := "other"
		ua := r.UserAgent()
		switch {
		case strings.Contains(ua, "Firefox/"):
			browser = "firefox"
		case strings.Contains(ua, "Edg/"):
			browser = "edge"
		case strings.Contains(ua, "Chrome/"):
			browser = "chrome"
		case strings.Contains(ua, "Safari/"):
			browser = "safari"
		}
		metrics.Widget(event.Stage, event.Outcome, browser, event.DurationMS/1000)
		if event.Outcome == "error" || event.Outcome == "timeout" || event.Outcome == "not_connected" {
			slog.WarnContext(r.Context(), "widget operation failed", "stage", event.Stage, "outcome", event.Outcome, "browser", browser, "sdk_release", event.SDKRelease, "close_code", event.CloseCode)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
