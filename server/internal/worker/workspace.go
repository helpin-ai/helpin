package worker

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

const persistentWorkspaceRootDir = "helpin-agent-workspaces"

const (
	runtimeSkillRootDir    = "helpin-runtime-skills"
	codexRuntimeRootDir    = "helpin-codex"
	openCodeRuntimeRootDir = "helpin-opencode"
)

const workspaceGitUserName = "Helpin Agent"
const workspaceGitUserEmail = "agent@helpin.ai"

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
	var firstErr error
	for _, cleanupPath := range []string{
		persistentWorkspacePath(runID),
		runtimeSkillRootPath(runID),
		codexRuntimeRootPath(runID),
		openCodeRuntimeRootPath(runID),
	} {
		if err := os.RemoveAll(cleanupPath); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// PersistentWorkspacePathForRun returns the stable checkout path used for an interactive run.
func PersistentWorkspacePathForRun(runID string) string {
	return persistentWorkspacePath(runID)
}

// RuntimeSkillRootPathForRun returns the per-run staged skill root for a runtime.
func RuntimeSkillRootPathForRun(runID, runtimeKind string) string {
	return filepath.Join(runtimeSkillRootPath(runID), sanitizeWorkspacePathComponent(runtimeKind), "helpin")
}

type RepoSkillMask struct {
	moves []repoSkillMove
}

type repoSkillMove struct {
	originalPath string
	hiddenPath   string
}

func MaskRepoSkillRoots(workDir, runID string) (*RepoSkillMask, error) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return nil, nil
	}
	suffix := sanitizeWorkspacePathComponent(runID)
	if suffix == "" {
		suffix = "run"
	}
	mask := &RepoSkillMask{}
	for _, relativePath := range []string{
		filepath.Join(".agents", "skills"),
		filepath.Join(".codex", "skills"),
	} {
		originalPath := filepath.Join(workDir, relativePath)
		info, err := os.Lstat(originalPath)
		if err != nil {
			if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) || isPathComponentNotDirectory(err) {
				continue
			}
			_ = mask.Restore()
			return nil, fmt.Errorf("stat repo skill root %q: %w", relativePath, err)
		}
		if !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		hiddenPath := filepath.Join(filepath.Dir(originalPath), ".helpin-hidden-skills-"+suffix)
		if err := os.RemoveAll(hiddenPath); err != nil {
			_ = mask.Restore()
			return nil, fmt.Errorf("clear hidden repo skill root %q: %w", hiddenPath, err)
		}
		if err := os.Rename(originalPath, hiddenPath); err != nil {
			_ = mask.Restore()
			return nil, fmt.Errorf("hide repo skill root %q: %w", relativePath, err)
		}
		mask.moves = append(mask.moves, repoSkillMove{originalPath: originalPath, hiddenPath: hiddenPath})
	}
	if len(mask.moves) == 0 {
		return nil, nil
	}
	return mask, nil
}

func isPathComponentNotDirectory(err error) bool {
	if err == nil {
		return false
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	return errors.Is(pathErr.Err, os.ErrNotExist) || errors.Is(pathErr.Err, syscall.ENOTDIR)
}

func (m *RepoSkillMask) Restore() error {
	if m == nil {
		return nil
	}
	var firstErr error
	for idx := len(m.moves) - 1; idx >= 0; idx-- {
		move := m.moves[idx]
		if _, err := os.Lstat(move.hiddenPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.RemoveAll(move.originalPath); err != nil && !os.IsNotExist(err) {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.Rename(move.hiddenPath, move.originalPath); err != nil && !os.IsNotExist(err) {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func prepareWorkspace(ctx context.Context, gitIntegration *model.GitIntegration, repo, authToken, runID string) (string, bool, error) {
	if runID != "" {
		workRoot := filepath.Dir(persistentWorkspacePath(runID))
		workDir := workRoot
		if gitIntegration != nil && repo != "" {
			workDir = filepath.Join(workRoot, "repo")
		}
		if info, err := os.Stat(workDir); err == nil && info.IsDir() {
			if err := ensureWorkspaceGitIdentity(ctx, workDir); err != nil {
				return "", false, err
			}
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
		outputText := strings.TrimSpace(string(output))
		if outputText == "" {
			return fmt.Errorf("git clone failed: %w", err)
		}
		return fmt.Errorf("git clone failed: %w: %s", err, outputText)
	}

	// Remove the persisted http.extraheader from the cloned repo config.
	// git clone -c persists config values into .git/config, which causes
	// "Duplicate header" errors when subsequent commands also pass -c http.extraheader.
	unsetCmd := exec.CommandContext(cloneCtx, "git", "config", "--unset-all", "http.extraheader")
	unsetCmd.Dir = cloneDir
	_ = unsetCmd.Run()

	if err := ensureWorkspaceGitIdentity(ctx, cloneDir); err != nil {
		_ = os.RemoveAll(workRoot)
		return err
	}

	return nil
}

func ensureWorkspaceGitIdentity(ctx context.Context, cloneDir string) error {
	if strings.TrimSpace(cloneDir) == "" {
		return nil
	}
	configCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, pair := range [][2]string{
		{"user.name", workspaceGitUserName},
		{"user.email", workspaceGitUserEmail},
	} {
		cmd := exec.CommandContext(configCtx, "git", "config", pair[0], pair[1])
		cmd.Dir = cloneDir
		if output, err := cmd.CombinedOutput(); err != nil {
			outputText := strings.TrimSpace(string(output))
			if outputText == "" {
				return fmt.Errorf("configure git %s: %w", pair[0], err)
			}
			return fmt.Errorf("configure git %s: %w: %s", pair[0], err, outputText)
		}
	}
	return nil
}

func persistentWorkspacePath(runID string) string {
	return filepath.Join(os.TempDir(), persistentWorkspaceRootDir, sanitizeWorkspacePathComponent(runID), "repo")
}

func runtimeSkillRootPath(runID string) string {
	return filepath.Join(os.TempDir(), runtimeSkillRootDir, sanitizeWorkspacePathComponent(runID))
}

func codexRuntimeRootPath(runID string) string {
	return filepath.Join(os.TempDir(), codexRuntimeRootDir, sanitizeWorkspacePathComponent(runID))
}

func openCodeRuntimeRootPath(runID string) string {
	return filepath.Join(os.TempDir(), openCodeRuntimeRootDir, sanitizeWorkspacePathComponent(runID))
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
		switch gitIntegration.Provider {
		case "github":
			baseURL = model.ResolveGitHubWebBaseURL(gitIntegration.BaseURL)
		case "gitlab":
			baseURL = model.ResolveGitLabWebBaseURL(gitIntegration.BaseURL)
		default:
			baseURL = *gitIntegration.BaseURL
		}
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
