package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appcrypto "github.com/helpin-ai/helpin/server/internal/crypto"
	appmodel "github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

const codexAuthFileName = "auth.json"

type CodexWorkspaceAuthStore struct {
	repo          *repository.CodexWorkspaceAuthRepository
	encryptionKey []byte
}

func NewCodexWorkspaceAuthStore(repo *repository.CodexWorkspaceAuthRepository, encryptionKey []byte) *CodexWorkspaceAuthStore {
	if repo == nil || len(encryptionKey) != 32 {
		return nil
	}
	return &CodexWorkspaceAuthStore{
		repo:          repo,
		encryptionKey: append([]byte(nil), encryptionKey...),
	}
}

func codexShouldPersistWorkspaceAuth(provider, authMode string) bool {
	return strings.TrimSpace(provider) == appmodel.AgentModelProviderOpenAI &&
		strings.TrimSpace(authMode) == codexOpenAIAuthModeDevice
}

func (s *CodexWorkspaceAuthStore) Restore(ctx context.Context, workspaceID, provider, authMode, codexHome string) error {
	if s == nil || strings.TrimSpace(workspaceID) == "" || !codexShouldPersistWorkspaceAuth(provider, authMode) || strings.TrimSpace(codexHome) == "" {
		return nil
	}

	record, err := s.repo.GetByScope(ctx, workspaceID, provider, authMode)
	if err != nil || record == nil {
		return err
	}

	authJSON, err := appcrypto.DecryptString(record.AuthJSONEncrypted, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("decrypt workspace codex auth: %w", err)
	}
	return writeCodexAuthFile(filepath.Join(strings.TrimSpace(codexHome), codexAuthFileName), authJSON)
}

func (s *CodexWorkspaceAuthStore) Promote(ctx context.Context, workspaceID, provider, authMode, codexHome string) error {
	if s == nil || strings.TrimSpace(workspaceID) == "" || !codexShouldPersistWorkspaceAuth(provider, authMode) || strings.TrimSpace(codexHome) == "" {
		return nil
	}

	authJSON, err := readCodexAuthFile(filepath.Join(strings.TrimSpace(codexHome), codexAuthFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read session codex auth: %w", err)
	}

	encrypted, err := appcrypto.EncryptString(authJSON, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypt workspace codex auth: %w", err)
	}
	return s.repo.Upsert(ctx, &appmodel.CodexWorkspaceAuth{
		WorkspaceID:       strings.TrimSpace(workspaceID),
		Provider:          strings.TrimSpace(provider),
		AuthMode:          strings.TrimSpace(authMode),
		AuthJSONEncrypted: encrypted,
	})
}

func readCodexAuthFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func writeCodexAuthFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}
