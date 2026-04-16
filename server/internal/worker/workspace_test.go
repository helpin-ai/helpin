package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPrepareWorkspaceForRunReusesCheckoutAndPreservesLocalChanges(t *testing.T) {
	tempDir := t.TempDir()
	remoteRoot := filepath.Join(tempDir, "remote-root")
	remoteDir := filepath.Join(remoteRoot, "owner", "repo.git")
	if err := os.MkdirAll(filepath.Dir(remoteDir), 0o755); err != nil {
		t.Fatalf("create remote root: %v", err)
	}
	runGitWorkspaceCmd(t, tempDir, "git", "init", "--bare", remoteDir)

	seedDir := filepath.Join(tempDir, "seed")
	runGitWorkspaceCmd(t, tempDir, "git", "clone", remoteDir, seedDir)
	configureWorkspaceGitIdentity(t, seedDir)
	writeWorkspaceFile(t, filepath.Join(seedDir, "README.md"), "hello\n")
	runGitWorkspaceCmd(t, seedDir, "git", "add", "README.md")
	runGitWorkspaceCmd(t, seedDir, "git", "commit", "-m", "initial commit")
	runGitWorkspaceCmd(t, seedDir, "git", "branch", "-M", "main")
	runGitWorkspaceCmd(t, seedDir, "git", "push", "-u", "origin", "main")

	baseURL := "file://" + filepath.ToSlash(remoteRoot)
	integration := &model.GitIntegration{Provider: "github", BaseURL: &baseURL}

	workDir, reused, err := PrepareWorkspaceForRun(context.Background(), integration, "owner/repo", "", "run-123")
	if err != nil {
		t.Fatalf("prepare persistent workspace: %v", err)
	}
	if reused {
		t.Fatal("expected first workspace prepare to create a fresh checkout")
	}
	if _, err := os.Stat(filepath.Join(workDir, ".git")); err != nil {
		t.Fatalf("expected git checkout to exist: %v", err)
	}

	modifiedPath := filepath.Join(workDir, "README.md")
	writeWorkspaceFile(t, modifiedPath, "hello\nupdated locally\n")

	reusedDir, reused, err := PrepareWorkspaceForRun(context.Background(), integration, "owner/repo", "", "run-123")
	if err != nil {
		t.Fatalf("reuse persistent workspace: %v", err)
	}
	if !reused {
		t.Fatal("expected second workspace prepare to reuse the existing checkout")
	}
	if reusedDir != workDir {
		t.Fatalf("expected reused workspace path %q, got %q", workDir, reusedDir)
	}
	content, err := os.ReadFile(modifiedPath)
	if err != nil {
		t.Fatalf("read modified file: %v", err)
	}
	if string(content) != "hello\nupdated locally\n" {
		t.Fatalf("expected local changes to persist, got %q", string(content))
	}

	if err := os.MkdirAll(filepath.Join(runtimeSkillRootPath("run-123"), "codex", "helpin"), 0o755); err != nil {
		t.Fatalf("create runtime skill root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(codexRuntimeRootPath("run-123"), "home"), 0o755); err != nil {
		t.Fatalf("create codex runtime root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(openCodeRuntimeRootPath("run-123"), "home"), 0o755); err != nil {
		t.Fatalf("create opencode runtime root: %v", err)
	}

	if err := CleanupWorkspaceForRun("run-123"); err != nil {
		t.Fatalf("cleanup persistent workspace: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatalf("expected workspace to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(runtimeSkillRootPath("run-123")); !os.IsNotExist(err) {
		t.Fatalf("expected runtime skill root to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(codexRuntimeRootPath("run-123")); !os.IsNotExist(err) {
		t.Fatalf("expected codex runtime root to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(openCodeRuntimeRootPath("run-123")); !os.IsNotExist(err) {
		t.Fatalf("expected opencode runtime root to be removed, stat err=%v", err)
	}
}

func TestMaskRepoSkillRootsHidesAndRestoresRepoSkillDirectories(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".agents", "skills", "demo"), 0o755); err != nil {
		t.Fatalf("mkdir .agents skills: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, ".codex", "skills", "legacy"), 0o755); err != nil {
		t.Fatalf("mkdir .codex skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, ".agents", "config.toml"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write sibling config: %v", err)
	}

	mask, err := MaskRepoSkillRoots(workDir, "run-123")
	if err != nil {
		t.Fatalf("mask repo skill roots: %v", err)
	}
	if mask == nil {
		t.Fatal("expected repo skill mask")
	}
	if _, err := os.Stat(filepath.Join(workDir, ".agents", "skills")); !os.IsNotExist(err) {
		t.Fatalf("expected .agents/skills to be hidden, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".codex", "skills")); !os.IsNotExist(err) {
		t.Fatalf("expected .codex/skills to be hidden, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".agents", "config.toml")); err != nil {
		t.Fatalf("expected sibling .agents config to remain, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".agents", ".helpin-hidden-skills-run-123")); err != nil {
		t.Fatalf("expected hidden .agents skills dir, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".codex", ".helpin-hidden-skills-run-123")); err != nil {
		t.Fatalf("expected hidden .codex skills dir, stat err=%v", err)
	}

	if err := mask.Restore(); err != nil {
		t.Fatalf("restore repo skill roots: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".agents", "skills", "demo")); err != nil {
		t.Fatalf("expected .agents skills to be restored, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".codex", "skills", "legacy")); err != nil {
		t.Fatalf("expected .codex skills to be restored, stat err=%v", err)
	}
}

func configureWorkspaceGitIdentity(t *testing.T, dir string) {
	t.Helper()
	runGitWorkspaceCmd(t, dir, "git", "config", "user.email", "test@example.com")
	runGitWorkspaceCmd(t, dir, "git", "config", "user.name", "Test User")
}

func writeWorkspaceFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGitWorkspaceCmd(t *testing.T, dir string, command string, args ...string) string {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", command, strings.Join(args, " "), err, string(output))
	}
	return string(output)
}
