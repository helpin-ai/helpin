package worker

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PrepareWorkspace creates a temp workspace and clones the repository when configured.
func PrepareWorkspace(ctx context.Context, gitIntegration *model.GitIntegration, repo, authToken string) (string, error) {
	workDir, err := os.MkdirTemp("", "agent-workspace-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	if gitIntegration == nil || repo == "" {
		return workDir, nil
	}

	cloneURL, err := cloneURLFor(gitIntegration, repo)
	if err != nil {
		return "", err
	}

	cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cloneDir := filepath.Join(workDir, "repo")
	args := []string{"clone", "--depth", "1"}
	args = append(args, gitCloneAuthArgs(gitIntegration.Provider, authToken)...)
	args = append(args, cloneURL, cloneDir)

	cmd := exec.CommandContext(cloneCtx, "git", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(workDir)
		return "", fmt.Errorf("git clone failed: %s", string(output))
	}

	// Remove the persisted http.extraheader from the cloned repo config.
	// git clone -c persists config values into .git/config, which causes
	// "Duplicate header" errors when subsequent commands also pass -c http.extraheader.
	unsetCmd := exec.CommandContext(cloneCtx, "git", "config", "--unset-all", "http.extraheader")
	unsetCmd.Dir = cloneDir
	_ = unsetCmd.Run()

	return cloneDir, nil
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
