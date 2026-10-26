package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/middleware"
)

type iconSuggestionStub struct {
	calls                 int
	workspace, kind, name string
	err                   error
}

func (s *iconSuggestionStub) Suggest(_ context.Context, workspace, kind, name string) (string, error) {
	s.calls++
	s.workspace, s.kind, s.name = workspace, kind, name
	return "rocket", s.err
}
func TestIconSuggestionHandler(t *testing.T) {
	for _, tc := range []struct {
		name, body     string
		inbox, failure bool
		status         int
	}{
		{name: "collection", body: `{"name":"Getting Started"}`, status: 200},
		{name: "inbox", body: `{"name":"Billing"}`, inbox: true, status: 200},
		{name: "unavailable", body: `{"name":"Billing"}`, failure: true, status: 200},
		{name: "invalid", body: `{`, status: 400},
		{name: "long", body: `{"name":"` + strings.Repeat("a", 161) + `"}`, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &iconSuggestionStub{}
			if tc.failure {
				stub.err = errors.New("unavailable")
			}
			h := NewIconSuggestionHandler(stub)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req = req.WithContext(middleware.WithWorkspaceID(req.Context(), "workspace"))
			response := httptest.NewRecorder()
			if tc.inbox {
				h.Inbox(response, req)
			} else {
				h.Collection(response, req)
			}
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if tc.status == 400 {
				if stub.calls != 0 {
					t.Fatal("invalid input reached provider")
				}
				return
			}
			kind := "collection"
			if tc.inbox {
				kind = "inbox"
			}
			if stub.workspace != "workspace" || stub.kind != kind {
				t.Fatalf("scope=%+v", stub)
			}
			if tc.failure && strings.TrimSpace(response.Body.String()) != `{"icon":""}` {
				t.Fatal(response.Body.String())
			}
		})
	}
}
