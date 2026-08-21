package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OAuthMobileHandoffRepository persists single-use native OAuth handoffs.
type OAuthMobileHandoffRepository struct {
	db *gorm.DB
}

func NewOAuthMobileHandoffRepository(db *gorm.DB) *OAuthMobileHandoffRepository {
	return &OAuthMobileHandoffRepository{db: db}
}

func (r *OAuthMobileHandoffRepository) Create(ctx context.Context, handoff *model.OAuthMobileHandoff) error {
	if err := r.db.WithContext(ctx).Create(handoff).Error; err != nil {
		return fmt.Errorf("create oauth mobile handoff: %w", err)
	}
	return nil
}

// Consume atomically marks an active handoff as used and returns it. The row
// lock prevents two concurrent exchanges from both receiving a session.
func (r *OAuthMobileHandoffRepository) Consume(ctx context.Context, codeHash string, now time.Time) (*model.OAuthMobileHandoff, error) {
	var consumed *model.OAuthMobileHandoff
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var handoff model.OAuthMobileHandoff
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code_hash = ? AND used_at IS NULL AND expires_at > ?", codeHash, now).
			First(&handoff).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("get oauth mobile handoff: %w", err)
		}

		result := tx.Model(&model.OAuthMobileHandoff{}).
			Where("id = ? AND used_at IS NULL", handoff.ID).
			Update("used_at", now)
		if result.Error != nil {
			return fmt.Errorf("consume oauth mobile handoff: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return nil
		}

		handoff.UsedAt = &now
		consumed = &handoff
		return nil
	})
	if err != nil {
		return nil, err
	}
	return consumed, nil
}

func (r *OAuthMobileHandoffRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	if err := r.db.WithContext(ctx).
		Where("expires_at < ?", before).
		Delete(&model.OAuthMobileHandoff{}).Error; err != nil {
		return fmt.Errorf("delete expired oauth mobile handoffs: %w", err)
	}
	return nil
}
