package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

// PasswordResetTokenRepository handles password reset token persistence.
type PasswordResetTokenRepository struct {
	db *gorm.DB
}

// NewPasswordResetTokenRepository creates a new PasswordResetTokenRepository.
func NewPasswordResetTokenRepository(db *gorm.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{db: db}
}

// Create inserts a password reset token row.
func (r *PasswordResetTokenRepository) Create(ctx context.Context, token *model.PasswordResetToken) error {
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	return nil
}

// GetActiveByTokenHash returns an unconsumed, unexpired reset token.
func (r *PasswordResetTokenRepository) GetActiveByTokenHash(ctx context.Context, tokenHash string, now time.Time) (*model.PasswordResetToken, error) {
	var token model.PasswordResetToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, now).
		First(&token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get password reset token: %w", err)
	}
	return &token, nil
}

// MarkUsed marks a reset token as consumed if it is still active.
func (r *PasswordResetTokenRepository) MarkUsed(ctx context.Context, id string, usedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", usedAt)
	if result.Error != nil {
		return false, fmt.Errorf("mark password reset token used: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// InvalidateAllForUser consumes all active tokens for a user.
func (r *PasswordResetTokenRepository) InvalidateAllForUser(ctx context.Context, userID string, usedAt time.Time) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", usedAt).Error; err != nil {
		return fmt.Errorf("invalidate password reset tokens: %w", err)
	}
	return nil
}

// InvalidateOtherTokensForUser consumes all active tokens for a user except the current token.
func (r *PasswordResetTokenRepository) InvalidateOtherTokensForUser(ctx context.Context, userID, excludeID string, usedAt time.Time) error {
	if err := r.db.WithContext(ctx).
		Model(&model.PasswordResetToken{}).
		Where("user_id = ? AND id <> ? AND used_at IS NULL", userID, excludeID).
		Update("used_at", usedAt).Error; err != nil {
		return fmt.Errorf("invalidate other password reset tokens: %w", err)
	}
	return nil
}

// WithTx runs fn in a transaction and passes a repository bound to that transaction.
func (r *PasswordResetTokenRepository) WithTx(ctx context.Context, fn func(txRepo *PasswordResetTokenRepository, tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewPasswordResetTokenRepository(tx), tx)
	})
}
