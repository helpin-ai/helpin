package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

type CodexWorkspaceAuthRepository struct {
	db *gorm.DB
}

func NewCodexWorkspaceAuthRepository(db *gorm.DB) *CodexWorkspaceAuthRepository {
	return &CodexWorkspaceAuthRepository{db: db}
}

func (r *CodexWorkspaceAuthRepository) GetByScope(ctx context.Context, workspaceID, provider, authMode string) (*model.CodexWorkspaceAuth, error) {
	var record model.CodexWorkspaceAuth
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND provider = ? AND auth_mode = ?", strings.TrimSpace(workspaceID), strings.TrimSpace(provider), strings.TrimSpace(authMode)).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get codex workspace auth: %w", err)
	}
	return &record, nil
}

func (r *CodexWorkspaceAuthRepository) Upsert(ctx context.Context, record *model.CodexWorkspaceAuth) error {
	if record == nil {
		return fmt.Errorf("codex workspace auth record is required")
	}
	record.WorkspaceID = strings.TrimSpace(record.WorkspaceID)
	record.Provider = strings.TrimSpace(record.Provider)
	record.AuthMode = strings.TrimSpace(record.AuthMode)
	if record.WorkspaceID == "" || record.Provider == "" || record.AuthMode == "" {
		return fmt.Errorf("workspace_id, provider, and auth_mode are required")
	}
	if strings.TrimSpace(record.AuthJSONEncrypted) == "" {
		return fmt.Errorf("auth_json_encrypted is required")
	}

	existing, err := r.GetByScope(ctx, record.WorkspaceID, record.Provider, record.AuthMode)
	if err != nil {
		return err
	}
	if existing == nil {
		if strings.TrimSpace(record.ID) == "" {
			record.ID = uuid.NewString()
		}
		if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
			return fmt.Errorf("create codex workspace auth: %w", err)
		}
		return nil
	}

	existing.AuthJSONEncrypted = record.AuthJSONEncrypted
	if err := r.db.WithContext(ctx).Save(existing).Error; err != nil {
		return fmt.Errorf("update codex workspace auth: %w", err)
	}
	record.ID = existing.ID
	record.CreatedAt = existing.CreatedAt
	record.UpdatedAt = existing.UpdatedAt
	return nil
}
