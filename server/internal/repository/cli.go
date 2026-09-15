package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CLIRepository persists consent, hashed credentials, and local execution leases.
type CLIRepository struct{ db *gorm.DB }

// NewCLIRepository constructs a CLI repository.
func NewCLIRepository(db *gorm.DB) *CLIRepository { return &CLIRepository{db: db} }

// Consent creates a connection and its single-use authorization code atomically.
func (r *CLIRepository) Consent(ctx context.Context, c *model.CLIConnection, t *model.CLIToken) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		return tx.Create(t).Error
	})
}

// Connection returns the consent record, including revocation state.
func (r *CLIRepository) Connection(ctx context.Context, id string) (*model.CLIConnection, error) {
	var c model.CLIConnection
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

// Token looks up a digest; raw credentials never enter database queries.
func (r *CLIRepository) Token(ctx context.Context, hash string) (*model.CLIToken, error) {
	var t model.CLIToken
	err := r.db.WithContext(ctx).Where("hash = ?", hash).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &t, err
}

// Rotate consumes one code/refresh credential and saves the replacement pair.
// A refresh replay revokes the whole consent family in the same transaction.
func (r *CLIRepository) Rotate(ctx context.Context, old *model.CLIToken, tokens []model.CLIToken, now time.Time) (bool, error) {
	accepted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.CLIToken{}).Where("hash = ? AND used_at IS NULL AND expires_at > ?", old.Hash, now).Update("used_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			if old.Kind == "refresh" {
				return tx.Model(&model.CLIConnection{}).Where("id = ?", old.ConnectionID).Update("revoked_at", now).Error
			}
			return nil
		}
		if err := tx.Create(&tokens).Error; err != nil {
			return err
		}
		accepted = true
		return nil
	})
	return accepted, err
}

// Revoke invalidates all credentials and grants derived from a connection.
func (r *CLIRepository) Revoke(ctx context.Context, id string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.CLIConnection{}).Where("id = ?", id).Update("revoked_at", now).Error
}

// Reserve records the idempotency key before run preparation.
func (r *CLIRepository) Reserve(ctx context.Context, e *model.CLIExecution) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(e)
	return result.RowsAffected == 1, result.Error
}

// Execution finds an execution using its immutable identity.
func (r *CLIRepository) Execution(ctx context.Context, id string) (*model.CLIExecution, error) {
	var e model.CLIExecution
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &e, err
}

// SaveAdmission writes only the immutable policy snapshot after run preparation.
func (r *CLIRepository) SaveAdmission(ctx context.Context, id string, snapshot []byte, hash string) error {
	return r.db.WithContext(ctx).Model(&model.CLIExecution{}).Where("id = ? AND policy_hash = ''", id).Updates(map[string]any{"snapshot": string(snapshot), "policy_hash": hash}).Error
}

// Bind attaches exactly one local runtime ID to an unexpired execution epoch.
func (r *CLIRepository) Bind(ctx context.Context, id string, epoch int64, localID string, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.CLIExecution{}).Where("id = ? AND epoch = ? AND revoked_at IS NULL AND lease_expires_at > ? AND (local_run_id = '' OR local_run_id = ?)", id, epoch, now, localID).Update("local_run_id", localID)
	return result.RowsAffected == 1, result.Error
}

// Renew keeps the current bound epoch alive, or fences an expired epoch for recovery.
func (r *CLIRepository) Renew(ctx context.Context, e *model.CLIExecution, now time.Time) (bool, error) {
	updates := map[string]any{"lease_expires_at": now.Add(15 * time.Minute)}
	if !e.LeaseExpiresAt.After(now) {
		updates["epoch"] = e.Epoch + 1
		updates["local_run_id"] = ""
	}
	result := r.db.WithContext(ctx).Model(&model.CLIExecution{}).Where("id = ? AND epoch = ? AND lease_expires_at = ? AND revoked_at IS NULL", e.ID, e.Epoch, e.LeaseExpiresAt).Updates(updates)
	return result.RowsAffected == 1, result.Error
}

// RevokeExecution permanently disables one local execution grant.
func (r *CLIRepository) RevokeExecution(ctx context.Context, id string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.CLIExecution{}).Where("id = ?", id).Update("revoked_at", now).Error
}
