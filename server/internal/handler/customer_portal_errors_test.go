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
	unfinished := &service.SupportAttachmentRejectedError{Message: "clip.avi did not finish uploading. Please try again.", Retryable: true}
	for _, tc := range []struct {
		name, want, wantCode string
		err                  error
	}{
		{name: "rejected upload", err: fmt.Errorf("create attachment: %w", rejected), want: rejected.Message, wantCode: service.SupportAttachmentRefusedCode},
		{name: "unfinished upload", err: unfinished, want: unfinished.Message},
		{name: "too many files", err: service.ErrPortalTooManyAttachments, want: service.ErrPortalTooManyAttachments.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writePortalError(w, httptest.NewRequest(http.MethodPost, "/", nil), "ws", tc.err, "attachment unavailable")
			var body struct{ Error, Code string }
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != http.StatusBadRequest || body.Error != tc.want || body.Code != tc.wantCode {
				t.Fatalf("status %d body %q code %q, want 400 %q code %q", w.Code, body.Error, body.Code, tc.want, tc.wantCode)
			}
		})
	}
	if strings.Contains(logs.String(), "portal request failed") {
		t.Fatalf("rejections were logged as server errors: %s", logs.String())
	}
}
