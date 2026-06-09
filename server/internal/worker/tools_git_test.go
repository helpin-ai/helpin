package worker

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunGitRecoversFromStaleIndexLock(t *testing.T) {
	repoDir := initTestGitRepo(t)
	writeTestRepoFile(t, repoDir, "tracked.txt", "hello\n")

	lockPath := filepath.Join(repoDir, ".git", "index.lock")
	if err := os.WriteFile(lockPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale index lock: %v", err)
	}
	staleTime := time.Now().Add(-staleGitIndexLockMinAge - time.Second)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("age stale index lock: %v", err)
	}

	execCtx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: repoDir,
	}

	if out, err := runGit(execCtx, "add", "-A"); err != nil {
		t.Fatalf("git add after stale lock recovery failed: %v\noutput: %s", err, out)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale index lock to be removed, got err=%v", err)
	}

	stagedFiles, err := runGit(execCtx, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatalf("inspect staged files: %v\noutput: %s", err, stagedFiles)
	}
	if !strings.Contains(stagedFiles, "tracked.txt") {
		t.Fatalf("expected tracked.txt to be staged, got %q", stagedFiles)
	}
}

func TestRunGitLeavesFreshIndexLockInPlace(t *testing.T) {
	repoDir := initTestGitRepo(t)
	writeTestRepoFile(t, repoDir, "tracked.txt", "hello\n")

	lockPath := filepath.Join(repoDir, ".git", "index.lock")
	if err := os.WriteFile(lockPath, []byte("fresh"), 0o644); err != nil {
		t.Fatalf("write fresh index lock: %v", err)
	}

	execCtx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: repoDir,
	}

	out, err := runGit(execCtx, "add", "-A")
	if err == nil {
		t.Fatalf("expected git add to fail while fresh index lock exists; output=%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "index.lock") {
		t.Fatalf("expected index.lock error output, got %q", out)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected fresh index lock to remain, got err=%v", err)
	}
}

func initTestGitRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\noutput: %s", err, string(out))
	}
	return repoDir
}

func writeTestRepoFile(t *testing.T, repoDir, name, content string) {
	t.Helper()

	path := filepath.Join(repoDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write repo file %s: %v", name, err)
	}
}

func TestToolListCommitsRequiresRepository(t *testing.T) {
	ctx := &ExecutionContext{Context: context.Background(), WorkspaceID: "ws-1"}
	if _, err := toolListCommits(ctx, json.RawMessage(`{"branch":"main"}`)); err == nil ||
		!strings.Contains(err.Error(), "requires a repository target") {
		t.Fatalf("expected repository-required error, got %v", err)
	}
}

func TestToolListCommitsReadsLog(t *testing.T) {
	repoDir := initTestGitRepo(t)
	for _, cfg := range [][]string{{"config", "user.email", "agent@helpin.test"}, {"config", "user.name", "Agent"}} {
		cmd := exec.Command("git", cfg...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", cfg, err, out)
		}
	}
	commit := func(file, msg string) {
		writeTestRepoFile(t, repoDir, file, msg+"\n")
		for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", msg}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
	}
	commit("a.txt", "first commit")
	commit("b.txt", "second commit")

	ctx := &ExecutionContext{Context: context.Background(), WorkspaceID: "ws-1", WorkDir: repoDir}
	// No branch → logs HEAD; the best-effort fetch fails (no origin) and is ignored.
	out, err := toolListCommits(ctx, json.RawMessage(`{"limit":10}`))
	if err != nil {
		t.Fatalf("toolListCommits returned error: %v", err)
	}
	if !strings.Contains(out, "first commit") || !strings.Contains(out, "second commit") {
		t.Fatalf("expected both commit subjects in output, got %q", out)
	}
}
