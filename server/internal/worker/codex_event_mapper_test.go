package worker

import (
	"strings"
	"testing"
)

func TestCodexDiffFromFileChangeNormalizesAbsoluteWorkdirPath(t *testing.T) {
	execCtx := &ExecutionContext{
		WorkDir: "/tmp/helpin-agent-workspaces/run-123/repo",
	}
	item := codexThreadItem{
		Type: "fileChange",
		Changes: []codexFileChange{
			{
				Path: "/tmp/helpin-agent-workspaces/run-123/repo/frontend/src/lib/utils/imageGeneration.ts",
				Diff: "@@ -1 +1 @@\n-old\n+new",
			},
		},
	}

	diff := codexDiffFromFileChange(execCtx, item)

	if !strings.Contains(diff, "--- a/frontend/src/lib/utils/imageGeneration.ts") {
		t.Fatalf("expected repo-relative diff header, got %q", diff)
	}
	if strings.Contains(diff, "/tmp/helpin-agent-workspaces/") {
		t.Fatalf("expected absolute temp path to be stripped, got %q", diff)
	}
}

func TestCodexDiffFromFileChangePreservesRelativePath(t *testing.T) {
	item := codexThreadItem{
		Type: "fileChange",
		Changes: []codexFileChange{
			{
				Path: "frontend/src/lib/utils/imageGeneration.ts",
				Diff: "@@ -1 +1 @@\n-old\n+new",
			},
		},
	}

	diff := codexDiffFromFileChange(nil, item)

	if !strings.Contains(diff, "--- a/frontend/src/lib/utils/imageGeneration.ts") {
		t.Fatalf("expected relative diff header, got %q", diff)
	}
}

func TestCodexDiffFromFileChangeNormalizesAgainstItemCWD(t *testing.T) {
	item := codexThreadItem{
		Type: "fileChange",
		Cwd:  "/tmp/helpin-agent-workspaces/run-123/repo/frontend",
		Changes: []codexFileChange{
			{
				Path: "/tmp/helpin-agent-workspaces/run-123/repo/frontend/src/lib/utils/imageGeneration.ts",
				Diff: "@@ -1 +1 @@\n-old\n+new",
			},
		},
	}

	diff := codexDiffFromFileChange(nil, item)

	if !strings.Contains(diff, "--- a/src/lib/utils/imageGeneration.ts") {
		t.Fatalf("expected item cwd-relative diff header, got %q", diff)
	}
}

func TestNormalizeCodexDiffPathLeavesExternalAbsolutePathCleaned(t *testing.T) {
	execCtx := &ExecutionContext{
		WorkDir: "/tmp/helpin-agent-workspaces/run-123/repo",
	}

	got := normalizeCodexDiffPath(execCtx, "/var/tmp/other-repo/file.txt")

	if got != "/var/tmp/other-repo/file.txt" {
		t.Fatalf("expected external path to remain absolute, got %q", got)
	}
}

func TestNormalizeCodexUnifiedDiffStripsAbsoluteHeaders(t *testing.T) {
	execCtx := &ExecutionContext{
		WorkDir: "/tmp/helpin-agent-workspaces/run-123/repo",
	}
	raw := strings.Join([]string{
		"diff --git a//tmp/helpin-agent-workspaces/run-123/repo/frontend/src/lib/utils/imageGeneration.ts b//tmp/helpin-agent-workspaces/run-123/repo/frontend/src/lib/utils/imageGeneration.ts",
		"--- a//tmp/helpin-agent-workspaces/run-123/repo/frontend/src/lib/utils/imageGeneration.ts",
		"+++ b//tmp/helpin-agent-workspaces/run-123/repo/frontend/src/lib/utils/imageGeneration.ts",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	got := normalizeCodexUnifiedDiff(execCtx, raw)

	if strings.Contains(got, "/tmp/helpin-agent-workspaces/") {
		t.Fatalf("expected absolute temp paths to be removed from unified diff, got %q", got)
	}
	if !strings.Contains(got, "diff --git a/frontend/src/lib/utils/imageGeneration.ts b/frontend/src/lib/utils/imageGeneration.ts") {
		t.Fatalf("expected normalized git header, got %q", got)
	}
}
