package worker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestToolGetPullRequestDiffUsesRunRepository(t *testing.T) {
	restore := stubGitHubToolHTTP(t, map[string]string{
		"/repos/acme/api/pulls/42/files?per_page=100": `[{"filename":"main.go","status":"modified","additions":3,"deletions":1,"changes":4,"patch":"@@ patch"}]`,
	})
	defer restore()

	ctx := &ExecutionContext{
		Context:        context.Background(),
		Repo:           "acme/api",
		GitAccessToken: "token",
	}
	output, err := toolGetPullRequestDiff(ctx, json.RawMessage(`{"pull_number":42}`))
	if err != nil {
		t.Fatalf("toolGetPullRequestDiff returned error: %v", err)
	}
	if !strings.Contains(output, `"filename":"main.go"`) || !strings.Contains(output, `"pull_number":42`) {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestToolGetCheckRunLogsReturnsOutputAndAnnotations(t *testing.T) {
	restore := stubGitHubToolHTTP(t, map[string]string{
		"/repos/acme/api/check-runs/99":                         `{"id":99,"name":"ci","status":"completed","conclusion":"failure","output":{"title":"failed","summary":"tests failed","text":"panic"}}`,
		"/repos/acme/api/check-runs/99/annotations?per_page=50": `[{"path":"main.go","start_line":10,"end_line":10,"annotation_level":"failure","message":"bad","title":"lint"}]`,
	})
	defer restore()

	ctx := &ExecutionContext{
		Context: context.Background(),
		RunInput: &model.AgentRunInputPayload{Event: &model.AgentRunEventContext{GitHub: &model.AgentRunGitHubEventContext{
			RepoFullName: "acme/api",
		}}},
		GitAccessToken: "token",
	}
	output, err := toolGetCheckRunLogs(ctx, json.RawMessage(`{"check_run_id":99}`))
	if err != nil {
		t.Fatalf("toolGetCheckRunLogs returned error: %v", err)
	}
	if !strings.Contains(output, `"conclusion":"failure"`) || !strings.Contains(output, `"message":"bad"`) {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestToolGetPullRequestDiffRequiresPullNumber(t *testing.T) {
	ctx := &ExecutionContext{Context: context.Background(), Repo: "acme/api", GitAccessToken: "token"}
	if _, err := toolGetPullRequestDiff(ctx, json.RawMessage(`{}`)); err == nil || err.Error() != "pull_number is required" {
		t.Fatalf("expected pull_number validation error, got %v", err)
	}
}

func stubGitHubToolHTTP(t *testing.T, responses map[string]string) func() {
	t.Helper()
	previous := githubToolHTTPClient
	githubToolHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, ok := responses[req.URL.RequestURI()]
		if !ok {
			t.Fatalf("unexpected GitHub request %s", req.URL.RequestURI())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization header = %q, want Bearer token", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	return func() { githubToolHTTPClient = previous }
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
