package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// PasskeyRepository stores and retrieves user passkeys.
type PasskeyRepository struct {
	db *gorm.DB
}

func NewPasskeyRepository(db *gorm.DB) *PasskeyRepository {
	return &PasskeyRepository{db: db}
}

func (r *PasskeyRepository) Create(ctx context.Context, passkey *model.UserPasskey) (*model.UserPasskey, error) {
	if err := r.db.WithContext(ctx).Create(passkey).Error; err != nil {
		return nil, fmt.Errorf("create passkey: %w", err)
	}
	return passkey, nil
}

func (r *PasskeyRepository) ListByUser(ctx context.Context, userID string) ([]model.UserPasskey, error) {
	var passkeys []model.UserPasskey
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&passkeys).Error; err != nil {
		return nil, fmt.Errorf("list passkeys: %w", err)
	}
	return passkeys, nil
}

func (r *PasskeyRepository) GetByID(ctx context.Context, userID, id string) (*model.UserPasskey, error) {
	var passkey model.UserPasskey
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&passkey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get passkey by id: %w", err)
	}
	return &passkey, nil
}

func (r *PasskeyRepository) GetByCredentialID(ctx context.Context, credentialID []byte) (*model.UserPasskey, error) {
	var passkey model.UserPasskey
	if err := r.db.WithContext(ctx).
		Where("credential_id = ?", credentialID).
		First(&passkey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get passkey by credential id: %w", err)
	}
	return &passkey, nil
}

func (r *PasskeyRepository) Delete(ctx context.Context, userID, id string) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.UserPasskey{})
	if result.Error != nil {
		return fmt.Errorf("delete passkey: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PasskeyRepository) UpdateAuthenticationState(ctx context.Context, id string, signCount int64, flags int, verified bool) (*model.UserPasskey, error) {
	updates := map[string]any{
		"sign_count": signCount,
		"flags":      flags,
		"verified":   verified,
	}
	if err := r.db.WithContext(ctx).Model(&model.UserPasskey{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update passkey authentication state: %w", err)
	}

	var passkey model.UserPasskey
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&passkey).Error; err != nil {
		return nil, fmt.Errorf("reload passkey authentication state: %w", err)
	}
	return &passkey, nil
}
