package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/prcontent"
)

func TestEnsurePullRequestRefreshesOnlyManagedBody(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	existingBody := "Human introduction.\n\n" + prcontent.Wrap("old Helpin section") + "\n\n- [ ] Human checklist"
	generatedBody := prcontent.Wrap("new Helpin section")
	var patchedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/installation-1/access_tokens":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/api/pulls":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"number":   12,
				"title":    "Existing PR",
				"body":     existingBody,
				"html_url": "https://github.test/acme/api/pull/12",
				"state":    "open",
				"head":     map[string]string{"ref": "feature", "sha": "abc"},
				"base":     map[string]string{"ref": "main"},
			}})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/acme/api/pulls/12":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			patchedBody = payload["body"]
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   12,
				"title":    "Existing PR",
				"body":     patchedBody,
				"html_url": "https://github.test/acme/api/pull/12",
				"state":    "open",
				"head":     map[string]string{"ref": "feature", "sha": "abc"},
				"base":     map[string]string{"ref": "main"},
			})
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &Client{appID: "1", privateKey: key, apiBaseURL: server.URL, httpClient: server.Client()}
	pr, err := client.EnsurePullRequest(context.Background(), "installation-1", "acme", "api", EnsurePullRequestInput{
		Head: "feature", Base: "main", Title: "Ignored for existing PR", Body: generatedBody,
	})
	if err != nil {
		t.Fatalf("EnsurePullRequest: %v", err)
	}
	for _, expected := range []string{"Human introduction.", "new Helpin section", "- [ ] Human checklist"} {
		if !strings.Contains(patchedBody, expected) {
			t.Fatalf("patched body missing %q: %s", expected, patchedBody)
		}
	}
	if strings.Contains(patchedBody, "old Helpin section") {
		t.Fatalf("patched body retained old managed content: %s", patchedBody)
	}
	if pr == nil || pr.Body != patchedBody {
		t.Fatalf("pull request body = %#v, want patched body", pr)
	}
}
