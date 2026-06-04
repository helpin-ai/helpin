package worker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

var githubToolHTTPClient = http.DefaultClient

func toolGetPullRequestDiff(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Owner      string `json:"owner"`
		Repo       string `json:"repo"`
		PullNumber int    `json:"pull_number"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	owner, repo := resolveGitHubOwnerRepo(ctx, params.Owner, params.Repo)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("owner and repo are required")
	}
	if params.PullNumber <= 0 {
		return "", fmt.Errorf("pull_number is required")
	}

	var files []struct {
		Filename  string `json:"filename"`
		Status    string `json:"status"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Changes   int    `json:"changes"`
		Patch     string `json:"patch,omitempty"`
	}
	if err := githubToolRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=100", owner, repo, params.PullNumber), &files); err != nil {
		return "", err
	}
	return marshalToolJSON(map[string]any{
		"owner":       owner,
		"repo":        repo,
		"pull_number": params.PullNumber,
		"files":       files,
	})
}

func toolGetCheckRunLogs(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Owner      string `json:"owner"`
		Repo       string `json:"repo"`
		CheckRunID int64  `json:"check_run_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}
	owner, repo := resolveGitHubOwnerRepo(ctx, params.Owner, params.Repo)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("owner and repo are required")
	}
	if params.CheckRunID <= 0 {
		return "", fmt.Errorf("check_run_id is required")
	}

	var checkRun struct {
		ID          int64      `json:"id"`
		Name        string     `json:"name"`
		HTMLURL     string     `json:"html_url"`
		Status      string     `json:"status"`
		Conclusion  string     `json:"conclusion"`
		StartedAt   *time.Time `json:"started_at"`
		CompletedAt *time.Time `json:"completed_at"`
		Output      struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Text    string `json:"text"`
		} `json:"output"`
	}
	if err := githubToolRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/check-runs/%d", owner, repo, params.CheckRunID), &checkRun); err != nil {
		return "", err
	}
	var annotations []struct {
		Path            string `json:"path"`
		StartLine       int    `json:"start_line"`
		EndLine         int    `json:"end_line"`
		AnnotationLevel string `json:"annotation_level"`
		Message         string `json:"message"`
		Title           string `json:"title"`
	}
	if err := githubToolRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/check-runs/%d/annotations?per_page=50", owner, repo, params.CheckRunID), &annotations); err != nil {
		return "", err
	}
	return marshalToolJSON(map[string]any{
		"owner":       owner,
		"repo":        repo,
		"check_run":   checkRun,
		"annotations": annotations,
	})
}

func resolveGitHubOwnerRepo(ctx *ExecutionContext, owner, repo string) (string, string) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner != "" && repo != "" {
		return owner, repo
	}
	repoFullName := strings.TrimSpace(ctx.Repo)
	if repoFullName == "" && ctx.RunInput != nil && ctx.RunInput.Event != nil && ctx.RunInput.Event.GitHub != nil {
		repoFullName = strings.TrimSpace(ctx.RunInput.Event.GitHub.RepoFullName)
	}
	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return owner, repo
	}
	if owner == "" {
		owner = strings.TrimSpace(parts[0])
	}
	if repo == "" {
		repo = strings.TrimSpace(parts[1])
	}
	return owner, repo
}

func githubToolRequest(ctx *ExecutionContext, method, path string, out any) error {
	if ctx == nil {
		return fmt.Errorf("execution context is required")
	}
	token := strings.TrimSpace(ctx.GitAccessToken)
	if token == "" && ctx.GitIntegration != nil {
		token = strings.TrimSpace(ctx.GitIntegration.AccessToken)
	}
	if token == "" {
		return fmt.Errorf("github access token is not available for this run")
	}
	apiBase := "https://api.github.com"
	if ctx.GitIntegration != nil {
		apiBase = model.ResolveGitHubAPIBaseURL(ctx.GitIntegration.BaseURL)
	}
	req, err := http.NewRequestWithContext(ctx.Context, method, strings.TrimRight(apiBase, "/")+path, nil)
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := githubToolHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request github: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("github request failed (%d)", resp.StatusCode)
	}
	return nil
}

func marshalToolJSON(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
