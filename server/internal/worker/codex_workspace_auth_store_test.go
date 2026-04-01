package worker

import (
	"os"
	"path/filepath"
	"testing"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

func TestCodexWorkspaceAuthPromoteAndRestore(t *testing.T) {
	baseDir := t.TempDir()
	sessionHome := filepath.Join(baseDir, "session", ".codex")
	if err := os.MkdirAll(sessionHome, 0o755); err != nil {
		t.Fatalf("mkdir session home: %v", err)
	}

	source := filepath.Join(sessionHome, codexAuthFileName)
	if err := os.WriteFile(source, []byte(`{"tokens":{"access_token":"abc"}}`), 0o600); err != nil {
		t.Fatalf("write session auth: %v", err)
	}

	if err := codexPromoteWorkspaceAuthUnder(baseDir, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeDevice, sessionHome); err != nil {
		t.Fatalf("promote auth: %v", err)
	}

	restoreHome := filepath.Join(baseDir, "restore", ".codex")
	if err := os.MkdirAll(restoreHome, 0o755); err != nil {
		t.Fatalf("mkdir restore home: %v", err)
	}
	if err := codexRestoreWorkspaceAuthUnder(baseDir, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeDevice, restoreHome); err != nil {
		t.Fatalf("restore auth: %v", err)
	}

	restored, err := os.ReadFile(filepath.Join(restoreHome, codexAuthFileName))
	if err != nil {
		t.Fatalf("read restored auth: %v", err)
	}
	if string(restored) != `{"tokens":{"access_token":"abc"}}` {
		t.Fatalf("unexpected restored auth: %s", string(restored))
	}
}

func TestCodexWorkspaceAuthStoreSkipsUnsupportedModes(t *testing.T) {
	baseDir := t.TempDir()
	sessionHome := filepath.Join(baseDir, "session", ".codex")
	if err := os.MkdirAll(sessionHome, 0o755); err != nil {
		t.Fatalf("mkdir session home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionHome, codexAuthFileName), []byte(`{"api_key":"x"}`), 0o600); err != nil {
		t.Fatalf("write session auth: %v", err)
	}

	if err := codexPromoteWorkspaceAuthUnder(baseDir, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeAPIKey, sessionHome); err != nil {
		t.Fatalf("promote unsupported mode: %v", err)
	}

	shared := filepath.Join(codexWorkspaceAuthHomeUnder(baseDir, "ws-1", appmodel.AgentModelProviderOpenAI, codexOpenAIAuthModeAPIKey), codexAuthFileName)
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatalf("expected no shared auth file for unsupported mode, got err=%v", err)
	}
}
