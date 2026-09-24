package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestMeetingCaptureNotConfiguredIsAClientError(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := fmt.Errorf("start recall meeting capture: %w", fmt.Errorf("recall: %w", meetingcapture.ErrNotConfigured))
	if !writeMeetingCaptureNotConfigured(recorder, err) {
		t.Fatal("not-configured error was not handled")
	}
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), service.MeetingCaptureNotConfiguredMessage) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
	if writeMeetingCaptureNotConfigured(httptest.NewRecorder(), errors.New("meeting not found")) {
		t.Fatal("unrelated error was handled as not configured")
	}
}
