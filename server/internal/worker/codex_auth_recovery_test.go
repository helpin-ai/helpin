package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCodexShouldReauthForError(t *testing.T) {
	t.Parallel()

	refreshErr := errors.New("Your access token could not be refreshed because your refresh token was already used.")
	deviceProfile := codexResolvedRuntimeProfile{
		Provider: appmodel.AgentModelProviderOpenAI,
		AuthMode: codexOpenAIAuthModeDevice,
	}

	if !codexShouldReauthForError(deviceProfile, refreshErr) {
		t.Fatalf("expected device-code refresh-token reuse error to require reauth")
	}
	if codexShouldReauthForError(deviceProfile, errors.New("temporary network failure")) {
		t.Fatalf("expected unrelated error to skip reauth")
	}

	apiKeyProfile := codexResolvedRuntimeProfile{
		Provider: appmodel.AgentModelProviderOpenAI,
		AuthMode: codexOpenAIAuthModeAPIKey,
	}
	if codexShouldReauthForError(apiKeyProfile, refreshErr) {
		t.Fatalf("expected api key auth mode to skip device-code reauth handling")
	}

	otherProviderProfile := codexResolvedRuntimeProfile{
		Provider: "openrouter",
		AuthMode: codexOpenAIAuthModeDevice,
	}
	if codexShouldReauthForError(otherProviderProfile, refreshErr) {
		t.Fatalf("expected non-openai provider to skip device-code reauth handling")
	}
}

func TestCodexClearRecoveredAuthStateRemovesSessionFileAndWorkspaceRecord(t *testing.T) {
	t.Parallel()

	db := setupCodexWorkspaceAuthStoreTestDB(t)
	store := NewCodexWorkspaceAuthStore(repository.NewCodexWorkspaceAuthRepository(db), make([]byte, 32))
	executor := &CodexExecutor{workspaceAuth: store}
	ctx := context.Background()

	profile := codexResolvedRuntimeProfile{
		Provider: appmodel.AgentModelProviderOpenAI,
		AuthMode: codexOpenAIAuthModeDevice,
	}
	state := &codexSessionState{
		CodexHome: filepath.Join(t.TempDir(), ".codex"),
	}
	if err := os.MkdirAll(state.CodexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	authPath := filepath.Join(state.CodexHome, codexAuthFileName)
	if err := os.WriteFile(authPath, []byte(`{"refresh_token":"stale"}`), 0o600); err != nil {
		t.Fatalf("write session auth: %v", err)
	}
	if err := store.Promote(ctx, "ws-1", profile.Provider, profile.AuthMode, state.CodexHome); err != nil {
		t.Fatalf("promote auth: %v", err)
	}

	repo := repository.NewCodexWorkspaceAuthRepository(db)
	record, err := repo.GetByScope(ctx, "ws-1", profile.Provider, profile.AuthMode)
	if err != nil {
		t.Fatalf("get persisted auth before clear: %v", err)
	}
	if record == nil {
		t.Fatalf("expected persisted auth record before clear")
	}

	codexClearRecoveredAuthState(ctx, executor, "ws-1", state, profile)

	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("expected local auth file to be removed, got err=%v", err)
	}
	record, err = repo.GetByScope(ctx, "ws-1", profile.Provider, profile.AuthMode)
	if err != nil {
		t.Fatalf("get persisted auth after clear: %v", err)
	}
	if record != nil {
		t.Fatalf("expected persisted auth record to be removed")
	}
}
