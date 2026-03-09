package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMWritingProfileRepository handles DB operations for CRM writing profiles.
type CRMWritingProfileRepository struct {
	db *gorm.DB
}

// NewCRMWritingProfileRepository creates a new CRMWritingProfileRepository.
func NewCRMWritingProfileRepository(db *gorm.DB) *CRMWritingProfileRepository {
	return &CRMWritingProfileRepository{db: db}
}

// Create inserts a writing profile.
func (r *CRMWritingProfileRepository) Create(ctx context.Context, profile *model.CRMWritingProfile) error {
	if err := r.db.WithContext(ctx).Create(profile).Error; err != nil {
		return fmt.Errorf("create writing profile: %w", err)
	}
	return nil
}

// GetByMemberID returns the writing profile for a member in a workspace.
func (r *CRMWritingProfileRepository) GetByMemberID(ctx context.Context, workspaceID, memberID string) (*model.CRMWritingProfile, error) {
	var profile model.CRMWritingProfile
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND member_id = ?", workspaceID, memberID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get writing profile: %w", err)
	}
	return &profile, nil
}

// GetByID returns a writing profile by ID.
func (r *CRMWritingProfileRepository) GetByID(ctx context.Context, id string) (*model.CRMWritingProfile, error) {
	var profile model.CRMWritingProfile
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get writing profile: %w", err)
	}
	return &profile, nil
}

// List returns writing profiles for a workspace.
func (r *CRMWritingProfileRepository) List(ctx context.Context, workspaceID string) ([]model.CRMWritingProfile, error) {
	var profiles []model.CRMWritingProfile
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at DESC").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("list writing profiles: %w", err)
	}
	return profiles, nil
}

// Update updates a writing profile.
func (r *CRMWritingProfileRepository) Update(ctx context.Context, profile *model.CRMWritingProfile) error {
	if err := r.db.WithContext(ctx).Save(profile).Error; err != nil {
		return fmt.Errorf("update writing profile: %w", err)
	}
	return nil
}

// Delete removes a writing profile.
func (r *CRMWritingProfileRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMWritingProfile{}).Error; err != nil {
		return fmt.Errorf("delete writing profile: %w", err)
	}
	return nil
}
