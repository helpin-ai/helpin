package agentcontract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncRuntimeSkillRootCopiesTree(t *testing.T) {
	srcRoot := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(filepath.Join(srcRoot, "demo", "agents"), 0o755); err != nil {
		t.Fatalf("mkdir src tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "demo", "SKILL.md"), []byte("---\nname: demo\ndescription: demo\n---\n\nbody\n"), 0o644); err != nil {
		t.Fatalf("write src skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "demo", "agents", "openai.yaml"), []byte("policy:\n  allow_implicit_invocation: false\n"), 0o644); err != nil {
		t.Fatalf("write src metadata: %v", err)
	}

	destRoot := filepath.Join(t.TempDir(), "dest")
	if err := SyncRuntimeSkillRoot(srcRoot, destRoot); err != nil {
		t.Fatalf("sync runtime skill root: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destRoot, "demo", "SKILL.md")); err != nil {
		t.Fatalf("expected staged SKILL.md, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "demo", "agents", "openai.yaml")); err != nil {
		t.Fatalf("expected staged openai.yaml, got err=%v", err)
	}
}
