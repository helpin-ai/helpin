package widgetorigin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// DeniedError carries only the resolved installation ID, never credentials.
type DeniedError struct {
	InstallationID string
}

func (e *DeniedError) Error() string { return "widget origin or credentials are not allowed" }

// LogRejection records admission diagnostics without URLs containing session
// tokens, request bodies, widget keys, or internal authorization errors.
func LogRejection(ctx context.Context, r *http.Request, err error) {
	attrs := []any{
		"origin", diagnosticOrigin(r.Header.Get("Origin")),
		"origin_header_count", len(r.Header.Values("Origin")),
		"method", r.Method,
		"path", r.URL.Path,
	}
	var denied *DeniedError
	if errors.As(err, &denied) && denied.InstallationID != "" {
		attrs = append(attrs, "installation_id", denied.InstallationID)
	}
	slog.WarnContext(ctx, "widget origin rejected", attrs...)
}

func diagnosticOrigin(raw string) string {
	if raw == "" || raw == "null" {
		return raw
	}
	// Malformed origins can contain credentials or arbitrary payloads. Only log
	// a bounded origin-shaped value; preserve the literal header for valid cases.
	u, err := url.Parse(raw)
	if len(raw) > 512 || err != nil || u.Scheme == "" || u.Hostname() == "" ||
		u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || strings.ContainsAny(raw, " \t\r\n\\#") {
		return "[invalid origin]"
	}
	return raw
}
