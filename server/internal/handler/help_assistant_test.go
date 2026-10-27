package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestWriteHelpAssistant(t *testing.T) {
	for _, tt := range []struct {
		name   string
		body   any
		err    error
		status int
	}{
		// 200, not 202: the portal client drops the body of a 202.
		{"answer started", &service.HelpAssistantTurn{SessionID: "s1"}, nil, http.StatusOK},
		{"invalid", nil, service.ErrHelpAssistantInvalid, http.StatusBadRequest},
		{"not found", nil, service.ErrHelpAssistantNotFound, http.StatusNotFound},
		{"pending", nil, service.ErrHelpAssistantPending, http.StatusConflict},
		{"full", nil, service.ErrHelpAssistantSessionFull, http.StatusConflict},
		{"limit", nil, service.ErrHelpAssistantLimit, http.StatusTooManyRequests},
		{"busy", nil, service.ErrHelpAssistantBusy, http.StatusServiceUnavailable},
		{"unavailable with detail", nil, errors.Join(service.ErrHelpAssistantUnavailable, errors.New("runtime said no")), http.StatusServiceUnavailable},
		{"other", nil, errors.New("database down"), http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeHelpAssistant(w, httptest.NewRequest(http.MethodPost, "/", nil), http.StatusOK, tt.body, tt.err)
			if w.Code != tt.status || w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("status %d cache %q, want %d", w.Code, w.Header().Get("Cache-Control"), tt.status)
			}
			if strings.Contains(w.Body.String(), "runtime said no") || strings.Contains(w.Body.String(), "database down") {
				t.Fatalf("internal detail leaked: %s", w.Body.String())
			}
		})
	}
}
