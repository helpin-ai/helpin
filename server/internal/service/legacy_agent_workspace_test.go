package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyAgentWorkspacePathMatchesRetiredWorkerLayout(t *testing.T) {
	tests := []struct {
		name  string
		runID string
		want  string
	}{
		{name: "normal id", runID: "Run-ABC_123", want: "run-abc-123"},
		{name: "blank id", runID: "  ", want: "run"},
		{name: "punctuation only", runID: "!!!", want: "run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := filepath.Join(os.TempDir(), "helpin-agent-workspaces", tt.want, "repo")
			if got := legacyAgentWorkspacePath(tt.runID); got != want {
				t.Fatalf("legacyAgentWorkspacePath(%q) = %q, want %q", tt.runID, got, want)
			}
		})
	}
}
