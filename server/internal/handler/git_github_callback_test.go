package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestGitHubCallbackRedirectsInsteadOfFailing(t *testing.T) {
	gitService := service.NewGitService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		"https://helpin.example.com", "", "state-secret")
	h := NewGitHandler(gitService, nil)

	tests := []struct {
		name      string
		query     string
		wantQuery map[string]string
	}{
		{name: "installed from GitHub without state", query: "installation_id=42&setup_action=install",
			wantQuery: map[string]string{"installation_id": "42", "setup_action": "install"}},
		{name: "invalid state", query: "installation_id=42&setup_action=update&state=forged",
			wantQuery: map[string]string{"installation_id": "42", "setup_action": "update"}},
		{name: "nothing to link", query: "",
			wantQuery: map[string]string{"github": service.GitHubResultError}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.GitHubCallback(rec, httptest.NewRequest(http.MethodGet, "/api/git/github/callback?"+tt.query, nil))
			if rec.Code != http.StatusFound {
				t.Fatalf("expected a redirect, got %d: %s", rec.Code, rec.Body.String())
			}
			location, err := url.Parse(rec.Header().Get("Location"))
			if err != nil {
				t.Fatalf("parse location: %v", err)
			}
			if location.Host != "helpin.example.com" || location.Path != "/github/installed" {
				t.Fatalf("unexpected location %q", location.String())
			}
			for key, want := range tt.wantQuery {
				if got := location.Query().Get(key); got != want {
					t.Fatalf("%s = %q, want %q", key, got, want)
				}
			}
		})
	}
}
