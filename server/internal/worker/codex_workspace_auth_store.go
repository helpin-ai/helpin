package worker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	appmodel "github.com/helpin-ai/helpin/server/internal/model"
)

const (
	codexWorkspaceAuthRootDir = "helpin-codex-auth"
	codexAuthFileName         = "auth.json"
)

func codexShouldPersistWorkspaceAuth(provider, authMode string) bool {
	return strings.TrimSpace(provider) == appmodel.AgentModelProviderOpenAI &&
		strings.TrimSpace(authMode) == codexOpenAIAuthModeDevice
}

func codexWorkspaceAuthHome(workspaceID, provider, authMode string) string {
	return codexWorkspaceAuthHomeUnder(os.TempDir(), workspaceID, provider, authMode)
}

func codexWorkspaceAuthHomeUnder(baseDir, workspaceID, provider, authMode string) string {
	return filepath.Join(
		baseDir,
		codexWorkspaceAuthRootDir,
		sanitizeCodexPathComponent(workspaceID),
		sanitizeCodexPathComponent(provider),
		sanitizeCodexPathComponent(authMode),
		".codex",
	)
}

func codexRestoreWorkspaceAuth(workspaceID, provider, authMode, codexHome string) error {
	return codexRestoreWorkspaceAuthUnder(os.TempDir(), workspaceID, provider, authMode, codexHome)
}

func codexRestoreWorkspaceAuthUnder(baseDir, workspaceID, provider, authMode, codexHome string) error {
	if strings.TrimSpace(workspaceID) == "" || !codexShouldPersistWorkspaceAuth(provider, authMode) || strings.TrimSpace(codexHome) == "" {
		return nil
	}
	src := filepath.Join(codexWorkspaceAuthHomeUnder(baseDir, workspaceID, provider, authMode), codexAuthFileName)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat workspace codex auth: %w", err)
	}
	return copyCodexAuthFile(src, filepath.Join(strings.TrimSpace(codexHome), codexAuthFileName))
}

func codexPromoteWorkspaceAuth(workspaceID, provider, authMode, codexHome string) error {
	return codexPromoteWorkspaceAuthUnder(os.TempDir(), workspaceID, provider, authMode, codexHome)
}

func codexPromoteWorkspaceAuthUnder(baseDir, workspaceID, provider, authMode, codexHome string) error {
	if strings.TrimSpace(workspaceID) == "" || !codexShouldPersistWorkspaceAuth(provider, authMode) || strings.TrimSpace(codexHome) == "" {
		return nil
	}
	src := filepath.Join(strings.TrimSpace(codexHome), codexAuthFileName)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat session codex auth: %w", err)
	}
	dstHome := codexWorkspaceAuthHomeUnder(baseDir, workspaceID, provider, authMode)
	if err := os.MkdirAll(dstHome, 0o755); err != nil {
		return fmt.Errorf("create workspace codex auth home: %w", err)
	}
	return copyCodexAuthFile(src, filepath.Join(dstHome, codexAuthFileName))
}

func copyCodexAuthFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}

	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}
