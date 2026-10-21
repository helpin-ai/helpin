package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

// Upload rule violations are the customer's to fix: they return 400 with
// the reason and are not logged as server errors.
func TestWritePortalErrorReportsAttachmentRejections(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelError})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	rejected := &service.SupportAttachmentRejectedError{Message: "clip.avi exceeds the maximum size of 100MB"}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"rejected upload", fmt.Errorf("create attachment: %w", rejected), rejected.Message},
		{"too many files", service.ErrPortalTooManyAttachments, service.ErrPortalTooManyAttachments.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writePortalError(w, httptest.NewRequest(http.MethodPost, "/", nil), "ws", tc.err, "attachment unavailable")
			var body struct{ Error string }
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusBadRequest || body.Error != tc.want {
				t.Fatalf("status %d body %q, want 400 %q", w.Code, body.Error, tc.want)
			}
		})
	}
	if strings.Contains(logs.String(), "portal request failed") {
		t.Fatalf("rejections were logged as server errors: %s", logs.String())
	}
}
