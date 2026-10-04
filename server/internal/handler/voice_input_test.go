package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type voiceHandlerStub struct {
	err             error
	calls           int
	workspace, user string
}

func (s *voiceHandlerStub) Available() bool { return true }
func (s *voiceHandlerStub) Transcribe(_ context.Context, ws, user string, _ []byte) (llm.TranscriptionResult, error) {
	s.calls++
	s.workspace = ws
	s.user = user
	return llm.TranscriptionResult{Text: "editable draft"}, s.err
}
func TestVoiceHandlerSanitizesErrorsAndPreservesBillingStatus(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
		body   string
	}{
		{nil, 200, "editable draft"},
		{model.ErrAIAllowanceExhausted, 402, "AI usage exhausted"},
		{service.ErrVoiceBusy, 429, "already being transcribed"},
		{service.ErrVoiceInvalidAudio, 400, "5 minutes"},
		{errors.New("private upstream payload"), 502, "could not transcribe"},
	} {
		stub := &voiceHandlerStub{err: tt.err}
		handler := NewVoiceInputHandler(stub)
		req := httptest.NewRequest(http.MethodPost, "/dock/transcriptions", strings.NewReader("audio"))
		req = req.WithContext(middleware.WithWorkspaceID(middleware.WithUserID(req.Context(), "user"), "workspace"))
		resp := httptest.NewRecorder()
		handler.Transcribe(resp, req)
		if resp.Code != tt.status || !strings.Contains(resp.Body.String(), tt.body) || strings.Contains(resp.Body.String(), "private upstream") {
			t.Fatalf("response: %d %s", resp.Code, resp.Body.String())
		}
		if stub.workspace != "workspace" || stub.user != "user" {
			t.Fatal("lost authenticated identity")
		}
	}
}
func TestVoiceHandlerBoundsBodyBeforeService(t *testing.T) {
	stub := &voiceHandlerStub{}
	h := NewVoiceInputHandler(stub)
	req := httptest.NewRequest(http.MethodPost, "/dock/transcriptions", bytes.NewReader(make([]byte, service.MaxVoiceUploadBytes+1)))
	resp := httptest.NewRecorder()
	h.Transcribe(resp, req)
	if resp.Code != http.StatusRequestEntityTooLarge || stub.calls != 0 {
		t.Fatalf("status %d, calls %d", resp.Code, stub.calls)
	}
}

func TestVoiceCapabilitiesAdvertiseFiveMinutes(t *testing.T) {
	h := NewVoiceInputHandler(&voiceHandlerStub{})
	resp := httptest.NewRecorder()
	h.Capabilities(resp, httptest.NewRequest(http.MethodGet, "/dock/transcriptions", nil))
	if !strings.Contains(resp.Body.String(), `"max_seconds":300`) {
		t.Fatalf("capabilities: %s", resp.Body.String())
	}
}
