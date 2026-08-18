package repository

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// PublicShareRepository persists revocable public resource links.
type PublicShareRepository struct {
	db *gorm.DB
}

// NewPublicShareRepository creates a PublicShareRepository.
func NewPublicShareRepository(db *gorm.DB) *PublicShareRepository {
	return &PublicShareRepository{db: db}
}

// CreateActive returns the existing active share or creates a new one.
func (r *PublicShareRepository) CreateActive(ctx context.Context, workspaceID, resourceType, resourceID, actorID string) (*model.PublicShare, error) {
	var result *model.PublicShare
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.PublicShare
		err := tx.Where("workspace_id = ? AND resource_type = ? AND resource_id = ? AND revoked_at IS NULL", workspaceID, resourceType, resourceID).First(&existing).Error
		if err == nil {
			result = &existing
			return nil
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		token, err := newPublicShareToken()
		if err != nil {
			return err
		}
		created := &model.PublicShare{WorkspaceID: workspaceID, ResourceType: resourceType, ResourceID: resourceID, Token: token, CreatedBy: actorID}
		if err := tx.Create(created).Error; err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		// A concurrent creator may have won the partial unique index race.
		if existing, lookupErr := r.GetActiveByResource(ctx, workspaceID, resourceType, resourceID); lookupErr == nil && existing != nil {
			return existing, nil
		}
	}
	return result, err
}

// GetActiveByToken finds an unrevoked share without requiring authentication.
func (r *PublicShareRepository) GetActiveByToken(ctx context.Context, token string) (*model.PublicShare, error) {
	var share model.PublicShare
	err := r.db.WithContext(ctx).Where("token = ? AND revoked_at IS NULL", token).First(&share).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &share, err
}

// GetActiveByResource finds the current public share for a resource.
func (r *PublicShareRepository) GetActiveByResource(ctx context.Context, workspaceID, resourceType, resourceID string) (*model.PublicShare, error) {
	var share model.PublicShare
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND resource_type = ? AND resource_id = ? AND revoked_at IS NULL", workspaceID, resourceType, resourceID).
		First(&share).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &share, err
}

// RevokeActive revokes the active share and records the actor.
func (r *PublicShareRepository) RevokeActive(ctx context.Context, workspaceID, resourceType, resourceID, actorID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&model.PublicShare{}).
		Where("workspace_id = ? AND resource_type = ? AND resource_id = ? AND revoked_at IS NULL", workspaceID, resourceType, resourceID).
		Updates(map[string]any{"revoked_at": now, "revoked_by": actorID}).Error
}

func newPublicShareToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate public share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
