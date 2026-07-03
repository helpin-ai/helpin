package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// EmailVerificationTokenRepository handles email verification token persistence.
type EmailVerificationTokenRepository struct {
	db *gorm.DB
}

func NewEmailVerificationTokenRepository(db *gorm.DB) *EmailVerificationTokenRepository {
	return &EmailVerificationTokenRepository{db: db}
}

func (r *EmailVerificationTokenRepository) Create(ctx context.Context, token *model.EmailVerificationToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("create email verification token: %w", err)
	}
	return nil
}

func (r *EmailVerificationTokenRepository) GetActiveByTokenHash(ctx context.Context, tokenHash string, now time.Time) (*model.EmailVerificationToken, error) {
	var token model.EmailVerificationToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, now).
		First(&token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get email verification token: %w", err)
	}
	return &token, nil
}

func (r *EmailVerificationTokenRepository) MarkUsed(ctx context.Context, id string, usedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.EmailVerificationToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", usedAt)
	if result.Error != nil {
		return false, fmt.Errorf("mark email verification token used: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *EmailVerificationTokenRepository) InvalidateAllForUser(ctx context.Context, userID string, usedAt time.Time) error {
	if err := r.db.WithContext(ctx).
		Model(&model.EmailVerificationToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", usedAt).Error; err != nil {
		return fmt.Errorf("invalidate email verification tokens: %w", err)
	}
	return nil
}

func (r *EmailVerificationTokenRepository) WithTx(ctx context.Context, fn func(txRepo *EmailVerificationTokenRepository, tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewEmailVerificationTokenRepository(tx), tx)
	})
}
