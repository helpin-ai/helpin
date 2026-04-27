package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const codexRefreshTokenReuseMessage = "ChatGPT authentication needs to be refreshed. Sign in with ChatGPT again."

func codexUsesPersistedWorkspaceAuth(profile codexResolvedRuntimeProfile) bool {
	return strings.TrimSpace(profile.Provider) == appmodel.AgentModelProviderOpenAI &&
		strings.TrimSpace(profile.AuthMode) == codexOpenAIAuthModeDevice
}

func codexShouldReauthForError(profile codexResolvedRuntimeProfile, err error) bool {
	if err == nil {
		return false
	}
	return codexShouldReauthForMessage(profile, err.Error())
}

func codexShouldReauthForMessage(profile codexResolvedRuntimeProfile, message string) bool {
	if !codexUsesPersistedWorkspaceAuth(profile) {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "refresh token") &&
		strings.Contains(normalized, "already used")
}

func codexBuildReauthRequiredState(profile codexResolvedRuntimeProfile, message string) *appmodel.CodexAuthState {
	return &appmodel.CodexAuthState{
		Provider:  strings.TrimSpace(profile.Provider),
		AuthMode:  strings.TrimSpace(profile.AuthMode),
		State:     appmodel.CodexAuthStateRequired,
		Error:     optionalStringPtr(message),
		UpdatedAt: time.Now().UTC(),
	}
}

func codexClearRecoveredAuthState(ctx context.Context, executor *CodexExecutor, workspaceID string, state *codexSessionState, profile codexResolvedRuntimeProfile) {
	if state != nil && strings.TrimSpace(state.CodexHome) != "" {
		_ = os.Remove(filepath.Join(strings.TrimSpace(state.CodexHome), codexAuthFileName))
	}
	if executor != nil && ctx != nil && strings.TrimSpace(workspaceID) != "" && codexUsesPersistedWorkspaceAuth(profile) {
		_ = executor.clearWorkspaceAuth(ctx, workspaceID, profile.Provider, profile.AuthMode)
	}
}
