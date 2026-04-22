package worker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	gitCommandTimeout       = 60 * time.Second
	staleGitIndexLockMinAge = gitCommandTimeout + 5*time.Second
)

func toolCreateBranch(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Name == "" {
		return "", fmt.Errorf("branch name is required")
	}

	out, err := runGit(ctx, "checkout", "-b", params.Name)
	if err != nil {
		return "", fmt.Errorf("create branch: %s", out)
	}
	ctx.WorkingBranch = params.Name
	return fmt.Sprintf("Created and switched to branch %q", params.Name), nil
}

func toolCommitAndPush(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Message == "" {
		return "", fmt.Errorf("commit message is required")
	}

	if out, err := runGit(ctx, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add: %s", out)
	}

	if out, err := runGit(ctx, "commit", "-m", params.Message); err != nil {
		return "", fmt.Errorf("git commit: %s", out)
	}

	// Get current branch.
	branch, err := runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get branch: %s", branch)
	}
	branch = strings.TrimSpace(branch)
	ctx.WorkingBranch = branch

	if out, err := runGit(ctx, "push", "-u", "origin", branch); err != nil {
		return "", fmt.Errorf("git push: %s", out)
	}

	sha, _ := runGit(ctx, "rev-parse", "HEAD")
	sha = strings.TrimSpace(sha)
	if ctx.OnGitPush != nil {
		_ = ctx.OnGitPush(branch, sha)
	}
	return fmt.Sprintf("Committed and pushed to %s (SHA: %s)", branch, sha), nil
}

func toolOpenPR(ctx *ExecutionContext, input json.RawMessage) (string, error) {
	var params struct {
		Title      string `json:"title"`
		Body       string `json:"body"`
		BaseBranch string `json:"base_branch"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.BaseBranch == "" {
		params.BaseBranch = ctx.BaseBranch
	}
	if params.BaseBranch == "" {
		params.BaseBranch = "main"
	}

	if ctx.GitIntegration == nil {
		return "", fmt.Errorf("no git integration configured for this run")
	}

	// Get current branch.
	branch, err := runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get branch: %s", branch)
	}
	branch = strings.TrimSpace(branch)

	if ctx.GitIntegration.Provider == "github" {
		return createGitHubPR(ctx, params.Title, params.Body, branch, params.BaseBranch)
	}

	return "", fmt.Errorf("PR creation not supported for provider %q", ctx.GitIntegration.Provider)
}

func createGitHubPR(ctx *ExecutionContext, title, body, head, base string) (string, error) {
	apiBase := model.ResolveGitHubAPIBaseURL(ctx.GitIntegration.BaseURL)

	url := fmt.Sprintf("%s/repos/%s/pulls", apiBase, ctx.Repo)

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	})

	reqCtx, cancel := context.WithTimeout(ctx.Context, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	token := ctx.GitAccessToken
	if token == "" && ctx.GitIntegration != nil {
		token = ctx.GitIntegration.AccessToken
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusCreated {
		msg, _ := result["message"].(string)
		return "", fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, msg)
	}

	prURL, _ := result["html_url"].(string)
	prNumber, _ := result["number"].(float64)
	ctx.LatestPRMetadata = &PRMetadata{
		Provider: "github",
		URL:      prURL,
		Number:   int(prNumber),
		Head:     head,
		Base:     base,
	}
	if ctx.OnPROpen != nil {
		_ = ctx.OnPROpen(*ctx.LatestPRMetadata, title)
	}
	return fmt.Sprintf("PR #%d created: %s", int(prNumber), prURL), nil
}

func runGit(ctx *ExecutionContext, args ...string) (string, error) {
	out, err := runGitOnce(ctx, args...)
	if err == nil || !isGitIndexLockError(out, err) {
		return out, err
	}

	recovered, recoveryErr := recoverStaleGitIndexLock(ctx)
	if recoveryErr != nil || !recovered {
		return out, err
	}

	return runGitOnce(ctx, args...)
}

func runGitOnce(ctx *ExecutionContext, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx.Context, gitCommandTimeout)
	defer cancel()

	cmdArgs := append(gitAuthArgs(ctx), args...)
	cmd := exec.CommandContext(cmdCtx, "git", cmdArgs...)
	cmd.Dir = ctx.WorkDir

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func isGitIndexLockError(output string, err error) bool {
	if err == nil {
		return false
	}
	normalized := strings.ToLower(output)
	return strings.Contains(normalized, "index.lock") &&
		(strings.Contains(normalized, "another git process seems to be running") ||
			strings.Contains(normalized, "unable to create"))
}

func recoverStaleGitIndexLock(ctx *ExecutionContext) (bool, error) {
	gitDir, err := resolveGitDirPath(ctx)
	if err != nil {
		return false, err
	}

	lockPath := filepath.Join(gitDir, "index.lock")
	info, err := os.Stat(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if time.Since(info.ModTime()) < staleGitIndexLockMinAge {
		return false, nil
	}

	if err := os.Remove(lockPath); err != nil {
		return false, err
	}

	if ctx != nil {
		slog.WarnContext(ctx.Context, "removed stale git index lock before retrying git command",
			"work_dir", ctx.WorkDir,
			"lock_path", lockPath,
			"lock_age_seconds", time.Since(info.ModTime()).Seconds())
	}

	return true, nil
}

func resolveGitDirPath(ctx *ExecutionContext) (string, error) {
	if ctx == nil || strings.TrimSpace(ctx.WorkDir) == "" {
		return "", fmt.Errorf("workdir is required to resolve git directory")
	}

	cmdCtx, cancel := context.WithTimeout(ctx.Context, 10*time.Second)
	defer cancel()

	cmdArgs := append(gitAuthArgs(ctx), "rev-parse", "--git-dir")
	cmd := exec.CommandContext(cmdCtx, "git", cmdArgs...)
	cmd.Dir = ctx.WorkDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve git dir: %s", strings.TrimSpace(firstNonEmptyText(string(out), err.Error())))
	}

	gitDir := strings.TrimSpace(string(out))
	if gitDir == "" {
		return "", fmt.Errorf("resolve git dir: git returned an empty path")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(ctx.WorkDir, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

func gitAuthArgs(ctx *ExecutionContext) []string {
	token := ctx.GitAccessToken
	if token == "" && ctx.GitIntegration != nil {
		token = ctx.GitIntegration.AccessToken
	}
	if token == "" || ctx.GitIntegration == nil {
		return nil
	}

	switch ctx.GitIntegration.Provider {
	case "github":
		auth := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	case "gitlab":
		auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + token))
		return []string{"-c", "http.extraheader=Authorization: Basic " + auth}
	default:
		return nil
	}
}
