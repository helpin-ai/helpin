package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestCLIJSONRejectsUntrustedFieldsAndTrailingData(t *testing.T) {
	for _, body := range []string{`{"request_id":"test","workspace_id":"forged"}`, `{"request_id":"test"} {}`, `{"request_id":"` + strings.Repeat("x", 100<<10) + `"}`} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		var input model.CLIAdmissionRequest
		if err := decodeCLIJSON(w, r, &input); err == nil {
			t.Fatal("accepted untrusted payload")
		}
	}
}
func TestCLIErrorsAreSanitized(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{errors.New("provider password secret"), 500}, {service.ErrCLIUnauthorized, 401}, {service.ErrCLIForbidden, 403}, {service.ErrCLIConflict, 409}, {model.ErrAIUsageExhausted, 402}} {
		w := httptest.NewRecorder()
		writeCLIError(w, httptest.NewRequest("GET", "/", nil), tc.err)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("unsafe error: %d %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("credential response is cacheable")
		}
	}
}
func TestCLIDisabledRoutesDoNotAuthenticate(t *testing.T) {
	s, err := service.NewCLIService(nil, nil, nil, nil, service.CLIConfig{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewCLIHandler(s)
	for _, endpoint := range []http.HandlerFunc{h.Discovery, h.Metadata, h.Me, h.Agents, h.Admit, h.Execution} {
		w := httptest.NewRecorder()
		endpoint(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 404 {
			t.Fatalf("disabled route status %d", w.Code)
		}
	}
}
