package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestEpicPlannerProfileOmitsRepositoryEditTools(t *testing.T) {
	profile := GetRuntimeProfile(model.AgentPresetEpicPlanner)
	disallowed := []string{"write_file", "edit_file", "apply_patch"}
	for _, tool := range disallowed {
		if strings.Contains(strings.Join(profile.AllowedTools, ","), tool) {
			t.Fatalf("expected epic planner profile to exclude %s, got %v", tool, profile.AllowedTools)
		}
	}
}

func TestToolApplyPatchRequiresPriorReadForExistingFile(t *testing.T) {
	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("alpha\nbeta\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	patch := "*** Begin Patch\n*** Update File: sample.txt\n@@\n alpha\n-beta\n+beta-updated\n*** End Patch\n"
	_, err := toolApplyPatch(ctx, mustMarshalPatchInput(t, patch))
	if err == nil || !strings.Contains(err.Error(), "must read sample.txt before modifying it") {
		t.Fatalf("expected prior-read error, got %v", err)
	}
}

func TestToolApplyPatchUpdatesAndAddsFiles(t *testing.T) {
	workDir := t.TempDir()
	existingPath := filepath.Join(workDir, "sample.txt")
	if err := os.WriteFile(existingPath, []byte("alpha\nbeta\ngamma\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	if _, err := toolReadFile(ctx, json.RawMessage(`{"path":"sample.txt"}`)); err != nil {
		t.Fatalf("toolReadFile returned error: %v", err)
	}

	patch := "*** Begin Patch\n*** Update File: sample.txt\n@@\n alpha\n-beta\n+beta-updated\n gamma\n*** Add File: created.txt\n+new file\n+body\n*** End Patch\n"
	output, err := toolApplyPatch(ctx, mustMarshalPatchInput(t, patch))
	if err != nil {
		t.Fatalf("toolApplyPatch returned error: %v", err)
	}
	if !strings.Contains(output, "Applied patch touching 2 file(s)") {
		t.Fatalf("unexpected output %q", output)
	}

	updatedData, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(updatedData) != "alpha\nbeta-updated\ngamma\n" {
		t.Fatalf("unexpected updated content %q", string(updatedData))
	}

	createdData, err := os.ReadFile(filepath.Join(workDir, "created.txt"))
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if string(createdData) != "new file\nbody" {
		t.Fatalf("unexpected created content %q", string(createdData))
	}
}

func TestToolApplyPatchMovesFile(t *testing.T) {
	workDir := t.TempDir()
	sourcePath := filepath.Join(workDir, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("alpha\nbeta\n"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	ctx := &ExecutionContext{
		Context: context.Background(),
		WorkDir: workDir,
	}

	if _, err := toolReadFile(ctx, json.RawMessage(`{"path":"source.txt"}`)); err != nil {
		t.Fatalf("toolReadFile returned error: %v", err)
	}

	patch := "*** Begin Patch\n*** Update File: source.txt\n*** Move to: moved.txt\n@@\n alpha\n-beta\n+beta-moved\n*** End Patch\n"
	if _, err := toolApplyPatch(ctx, mustMarshalPatchInput(t, patch)); err != nil {
		t.Fatalf("toolApplyPatch returned error: %v", err)
	}

	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected source file to be removed, got err=%v", err)
	}
	movedData, err := os.ReadFile(filepath.Join(workDir, "moved.txt"))
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if string(movedData) != "alpha\nbeta-moved\n" {
		t.Fatalf("unexpected moved content %q", string(movedData))
	}
}

func mustMarshalPatchInput(t *testing.T, patch string) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"patch": patch})
	if err != nil {
		t.Fatalf("marshal patch input: %v", err)
	}
	return payload
}
