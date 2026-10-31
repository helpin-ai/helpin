package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// UserOIDCIdentityRepository persists links between users and OIDC subjects.
type UserOIDCIdentityRepository struct {
	db *gorm.DB
}

func NewUserOIDCIdentityRepository(db *gorm.DB) *UserOIDCIdentityRepository {
	return &UserOIDCIdentityRepository{db: db}
}

// GetByIssuerSubject returns the identity for an issuer and subject, or nil
// when none exists.
func (r *UserOIDCIdentityRepository) GetByIssuerSubject(ctx context.Context, issuer, subject string) (*model.UserOIDCIdentity, error) {
	identity := &model.UserOIDCIdentity{}
	err := r.db.WithContext(ctx).Where("issuer = ? AND subject = ?", issuer, subject).First(identity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get oidc identity: %w", err)
	}
	return identity, nil
}

// HasIdentity reports whether the user is linked to any OIDC identity.
func (r *UserOIDCIdentityRepository) HasIdentity(ctx context.Context, userID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UserOIDCIdentity{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("count oidc identities: %w", err)
	}
	return count > 0, nil
}

// Transaction runs fn in a database transaction.
func (r *UserOIDCIdentityRepository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func (r *UserOIDCIdentityRepository) Create(ctx context.Context, identity *model.UserOIDCIdentity) error {
	if err := r.db.WithContext(ctx).Create(identity).Error; err != nil {
		return fmt.Errorf("create oidc identity: %w", err)
	}
	return nil
}
