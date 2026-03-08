package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// FollowerRepository handles entity follower operations.
type FollowerRepository struct {
	db *gorm.DB
}

// NewFollowerRepository creates a new follower repository.
func NewFollowerRepository(db *gorm.DB) *FollowerRepository {
	return &FollowerRepository{db: db}
}

// Follow adds a user as a follower of an entity. Uses upsert to avoid duplicates.
func (r *FollowerRepository) Follow(ctx context.Context, follower *model.EntityFollower) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "entity_type"}, {Name: "entity_id"}},
			DoNothing: true,
		}).
		Create(follower).Error
}

// Unfollow removes a user from following an entity.
func (r *FollowerRepository) Unfollow(ctx context.Context, userID, entityType, entityID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, entityType, entityID).
		Delete(&model.EntityFollower{}).Error
}

// GetFollowers returns all user IDs following an entity.
func (r *FollowerRepository) GetFollowers(ctx context.Context, entityType, entityID string) ([]string, error) {
	var userIDs []string
	err := r.db.WithContext(ctx).Model(&model.EntityFollower{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Pluck("user_id", &userIDs).Error
	if err != nil {
		return nil, fmt.Errorf("get followers: %w", err)
	}
	return userIDs, nil
}

// IsFollowing checks if a user follows an entity.
func (r *FollowerRepository) IsFollowing(ctx context.Context, userID, entityType, entityID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.EntityFollower{}).
		Where("user_id = ? AND entity_type = ? AND entity_id = ?", userID, entityType, entityID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("is following: %w", err)
	}
	return count > 0, nil
}

// ListFollowers returns full follower records for an entity.
func (r *FollowerRepository) ListFollowers(ctx context.Context, entityType, entityID string) ([]model.EntityFollower, error) {
	var followers []model.EntityFollower
	err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Find(&followers).Error
	if err != nil {
		return nil, fmt.Errorf("list followers: %w", err)
	}
	return followers, nil
}

// ListUserFollowing returns all entities a user follows in a workspace.
func (r *FollowerRepository) ListUserFollowing(ctx context.Context, userID, workspaceID string) ([]model.EntityFollower, error) {
	var followers []model.EntityFollower
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND workspace_id = ?", userID, workspaceID).
		Order("created_at DESC").
		Find(&followers).Error
	if err != nil {
		return nil, fmt.Errorf("list user following: %w", err)
	}
	return followers, nil
}
