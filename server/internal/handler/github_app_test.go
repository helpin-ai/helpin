package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type fakeGitHubAppConfigurer struct {
	manifestErr error
	gotRequest  model.GitHubAppManifestRequest
	gotCode     string
	gotState    string
}

func (f *fakeGitHubAppConfigurer) Status(context.Context) (*model.GitHubAppStatusResponse, error) {
	return &model.GitHubAppStatusResponse{Configured: true, Source: model.GitHubAppSourceDatabase, Slug: "helpin"}, nil
}

func (f *fakeGitHubAppConfigurer) CreateManifest(_ context.Context, _, _ string, req model.GitHubAppManifestRequest) (*model.GitHubAppManifestResponse, error) {
	f.gotRequest = req
	if f.manifestErr != nil {
		return nil, f.manifestErr
	}
	return &model.GitHubAppManifestResponse{Manifest: json.RawMessage(`{"name":"Helpin"}`), PostURL: "https://github.com/settings/apps/new?state=s", State: "s"}, nil
}

func (f *fakeGitHubAppConfigurer) CompleteManifest(_ context.Context, code, state string) string {
	f.gotCode, f.gotState = code, state
	return "https://helpin.example.com/w/acme/settings/git-connections?github_app_manifest=created"
}

func serveGitHubApp(h *GitHubAppHandler, method, path, body string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/workspaces/{id}/github/app-status", h.Status)
	r.Post("/workspaces/{id}/github/app-manifest", h.CreateManifest)
	r.Get("/github/app-manifest/callback", h.ManifestCallback)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestGitHubAppHandlerCreateManifestStatuses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "ok", want: http.StatusOK},
		{name: "unavailable in edition", err: service.ErrGitHubAppManifestUnavailable, want: http.StatusNotFound},
		{name: "already configured", err: service.ErrGitHubAppAlreadyConfigured, want: http.StatusConflict},
		{name: "invalid", err: service.ErrGitHubAppManifestInvalid, want: http.StatusBadRequest},
		{name: "internal", err: errors.New("db down"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeGitHubAppConfigurer{manifestErr: tt.err}
			rec := serveGitHubApp(NewGitHubAppHandler(fake), http.MethodPost, "/workspaces/ws-1/github/app-manifest", `{"organization":"acme"}`)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.want, rec.Body.String())
			}
			if fake.gotRequest.Organization != "acme" {
				t.Fatalf("expected organization to be decoded, got %q", fake.gotRequest.Organization)
			}
			if tt.err != nil && strings.Contains(rec.Body.String(), "db down") {
				t.Fatal("internal error details leaked")
			}
		})
	}
}

func TestGitHubAppHandlerStatusAndCallback(t *testing.T) {
	fake := &fakeGitHubAppConfigurer{}
	h := NewGitHubAppHandler(fake)

	rec := serveGitHubApp(h, http.MethodGet, "/workspaces/ws-1/github/app-status", "")
	var status map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status response %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	for _, key := range []string{"configured", "source", "slug", "install_url", "manifest_available"} {
		if _, ok := status[key]; !ok {
			t.Fatalf("status response missing %q: %v", key, status)
		}
	}

	rec = serveGitHubApp(h, http.MethodGet, "/github/app-manifest/callback?code=abc&state=xyz", "")
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "github_app_manifest=created") {
		t.Fatalf("unexpected callback response %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if fake.gotCode != "abc" || fake.gotState != "xyz" {
		t.Fatalf("callback did not pass code/state: %q %q", fake.gotCode, fake.gotState)
	}
}
