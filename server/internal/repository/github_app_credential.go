package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// ErrGitHubAppCredentialExists reports that the instance already has a stored
// GitHub App and the caller did not ask to replace it.
var ErrGitHubAppCredentialExists = errors.New("github app credentials already exist")

// GitHubAppCredentialRepository stores the instance-wide GitHub App.
type GitHubAppCredentialRepository struct {
	db *gorm.DB
}

// NewGitHubAppCredentialRepository returns a repository backed by db.
func NewGitHubAppCredentialRepository(db *gorm.DB) *GitHubAppCredentialRepository {
	return &GitHubAppCredentialRepository{db: db}
}

// Get returns the stored App, or nil when none exists.
func (r *GitHubAppCredentialRepository) Get(ctx context.Context) (*model.GitHubAppCredential, error) {
	var credential model.GitHubAppCredential
	err := r.db.WithContext(ctx).Where("singleton = ?", true).First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get github app credentials: %w", err)
	}
	return &credential, nil
}

// Create stores the App. With replace=false it returns
// ErrGitHubAppCredentialExists when a row already exists; with replace=true it
// overwrites the existing row.
func (r *GitHubAppCredentialRepository) Create(ctx context.Context, credential *model.GitHubAppCredential, replace bool) error {
	if credential == nil {
		return fmt.Errorf("github app credential is required")
	}
	credential.Singleton = true
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.GitHubAppCredential
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("singleton = ?", true).First(&existing).Error
		switch {
		case err == nil && !replace:
			return ErrGitHubAppCredentialExists
		case err == nil:
			credential.ID = existing.ID
			credential.CreatedAt = existing.CreatedAt
			if err := tx.Save(credential).Error; err != nil {
				return fmt.Errorf("replace github app credentials: %w", err)
			}
			return nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return fmt.Errorf("load github app credentials: %w", err)
		}
		if err := tx.Create(credential).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrGitHubAppCredentialExists
			}
			return fmt.Errorf("create github app credentials: %w", err)
		}
		return nil
	})
}
