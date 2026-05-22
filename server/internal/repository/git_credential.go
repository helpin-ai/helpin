package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type GitCredentialRepository struct {
	db *gorm.DB
}

func NewGitCredentialRepository(db *gorm.DB) *GitCredentialRepository {
	return &GitCredentialRepository{db: db}
}

func (r *GitCredentialRepository) Create(ctx context.Context, credential *model.GitCredential) error {
	if credential == nil {
		return fmt.Errorf("git credential is required")
	}
	if err := r.db.WithContext(ctx).Create(credential).Error; err != nil {
		return fmt.Errorf("create git credential: %w", err)
	}
	return nil
}

func (r *GitCredentialRepository) Update(ctx context.Context, credential *model.GitCredential) error {
	if credential == nil {
		return fmt.Errorf("git credential is required")
	}
	if err := r.db.WithContext(ctx).Save(credential).Error; err != nil {
		return fmt.Errorf("update git credential: %w", err)
	}
	return nil
}

func (r *GitCredentialRepository) UpsertOAuthUser(ctx context.Context, credential *model.GitCredential) (*model.GitCredential, error) {
	if credential == nil {
		return nil, fmt.Errorf("git credential is required")
	}
	if strings.TrimSpace(credential.OrganizationID) == "" || strings.TrimSpace(credential.Provider) == "" || strings.TrimSpace(credential.BaseURL) == "" || credential.ExternalUserID == nil || strings.TrimSpace(*credential.ExternalUserID) == "" {
		return nil, fmt.Errorf("organization_id, provider, base_url, and external_user_id are required")
	}
	var existing model.GitCredential
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND provider = ? AND base_url = ? AND external_user_id = ? AND status = ?", credential.OrganizationID, credential.Provider, credential.BaseURL, *credential.ExternalUserID, "active").
		First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("load git credential: %w", err)
	}
	if err == gorm.ErrRecordNotFound {
		if err := r.Create(ctx, credential); err != nil {
			return nil, err
		}
		return credential, nil
	}
	existing.AuthType = credential.AuthType
	existing.AccountLogin = credential.AccountLogin
	existing.DisplayName = credential.DisplayName
	existing.Scopes = credential.Scopes
	existing.AccessTokenEncrypted = credential.AccessTokenEncrypted
	existing.RefreshTokenEncrypted = credential.RefreshTokenEncrypted
	existing.ExpiresAt = credential.ExpiresAt
	existing.Status = credential.Status
	existing.ConnectedBy = credential.ConnectedBy
	existing.LastError = nil
	if err := r.Update(ctx, &existing); err != nil {
		return nil, err
	}
	return &existing, nil
}

func (r *GitCredentialRepository) GetByID(ctx context.Context, id string) (*model.GitCredential, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil
	}
	var credential model.GitCredential
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&credential).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get git credential: %w", err)
	}
	return &credential, nil
}
