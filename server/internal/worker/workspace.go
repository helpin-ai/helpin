package worker

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const persistentWorkspaceRootDir = "helpin-agent-workspaces"

// PrepareWorkspace creates a temp workspace and clones the repository when configured.
func PrepareWorkspace(ctx context.Context, gitIntegration *model.GitIntegration, repo, authToken string) (string, error) {
	workDir, _, err := prepareWorkspace(ctx, gitIntegration, repo, authToken, "")
	return workDir, err
}

// PrepareWorkspaceForRun returns a stable per-run workspace path so an interactive
// runtime can pause and later resume against the same filesystem state.
func PrepareWorkspaceForRun(ctx context.Context, gitIntegration *model.GitIntegration, repo, authToken, runID string) (string, bool, error) {
	return prepareWorkspace(ctx, gitIntegration, repo, authToken, strings.TrimSpace(runID))
}

// CleanupWorkspaceForRun removes a stable per-run workspace created by PrepareWorkspaceForRun.
func CleanupWorkspaceForRun(runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	return os.RemoveAll(persistentWorkspacePath(runID))
}

// PersistentWorkspacePathForRun returns the stable checkout path used for an interactive run.
func PersistentWorkspacePathForRun(runID string) string {
	return persistentWorkspacePath(runID)
}

func prepareWorkspace(ctx context.Context, gitIntegration *model.GitIntegration, repo, authToken, runID string) (string, bool, error) {
	if runID != "" {
		workRoot := filepath.Dir(persistentWorkspacePath(runID))
		workDir := workRoot
		if gitIntegration != nil && repo != "" {
			workDir = filepath.Join(workRoot, "repo")
		}
		if info, err := os.Stat(workDir); err == nil && info.IsDir() {
			return workDir, true, nil
		}
		if err := os.MkdirAll(filepath.Dir(workRoot), 0o755); err != nil {
			return "", false, fmt.Errorf("create persistent workspace root: %w", err)
		}
		_ = os.RemoveAll(workRoot)
		if err := os.MkdirAll(workRoot, 0o755); err != nil {
			return "", false, fmt.Errorf("reset persistent workspace root: %w", err)
		}
		if err := cloneWorkspace(ctx, workRoot, workDir, gitIntegration, repo, authToken); err != nil {
			return "", false, err
		}
		return workDir, false, nil
	}

	workDir, err := os.MkdirTemp("", "agent-workspace-*")
	if err != nil {
		return "", false, fmt.Errorf("create temp dir: %w", err)
	}
	cloneDir := workDir
	if gitIntegration != nil && repo != "" {
		cloneDir = filepath.Join(workDir, "repo")
	}
	if err := cloneWorkspace(ctx, workDir, cloneDir, gitIntegration, repo, authToken); err != nil {
		return "", false, err
	}
	return cloneDir, false, nil
}

func cloneWorkspace(ctx context.Context, workRoot, cloneDir string, gitIntegration *model.GitIntegration, repo, authToken string) error {
	if gitIntegration == nil || repo == "" {
		return nil
	}

	cloneURL, err := cloneURLFor(gitIntegration, repo)
	if err != nil {
		_ = os.RemoveAll(workRoot)
		return err
	}

	cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{"clone", "--depth", "1"}
	args = append(args, gitCloneAuthArgs(gitIntegration.Provider, authToken)...)
	args = append(args, cloneURL, cloneDir)

	cmd := exec.CommandContext(cloneCtx, "git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(workRoot)
		return fmt.Errorf("git clone failed: %s", string(output))
	}

	// Remove the persisted http.extraheader from the cloned repo config.
	// git clone -c persists config values into .git/config, which causes
	// "Duplicate header" errors when subsequent commands also pass -c http.extraheader.
	unsetCmd := exec.CommandContext(cloneCtx, "git", "config", "--unset-all", "http.extraheader")
	unsetCmd.Dir = cloneDir
	_ = unsetCmd.Run()

	return nil
}

func persistentWorkspacePath(runID string) string {
	return filepath.Join(os.TempDir(), persistentWorkspaceRootDir, sanitizeWorkspacePathComponent(runID), "repo")
}

func sanitizeWorkspacePathComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "run"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return "run"
	}
	return sanitized
}

func cloneURLFor(gitIntegration *model.GitIntegration, repo string) (string, error) {
	var baseURL string
	switch gitIntegration.Provider {
	case "github":
		baseURL = "https://github.com"
	case "gitlab":
		baseURL = "https://gitlab.com"
	default:
		return "", fmt.Errorf("unsupported git provider %q", gitIntegration.Provider)
	}
	if gitIntegration.BaseURL != nil && *gitIntegration.BaseURL != "" {
		baseURL = *gitIntegration.BaseURL
	}
	return fmt.Sprintf("%s/%s.git", baseURL, repo), nil
}

func gitCloneAuthArgs(provider, token string) []string {
	if token == "" {
		return nil
	}
	switch provider {
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
