package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// FollowerService manages entity subscriptions.
type FollowerService struct {
	followerRepo *repository.FollowerRepository
}

// NewFollowerService creates a new follower service.
func NewFollowerService(followerRepo *repository.FollowerRepository) *FollowerService {
	return &FollowerService{followerRepo: followerRepo}
}

// Follow adds a user as a follower of an entity.
func (s *FollowerService) Follow(ctx context.Context, userID, entityType, entityID, workspaceID, reason string) error {
	if reason == "" {
		reason = "manual"
	}
	return s.followerRepo.Follow(ctx, &model.EntityFollower{
		UserID:      userID,
		EntityType:  entityType,
		EntityID:    entityID,
		WorkspaceID: workspaceID,
		Reason:      reason,
	})
}

// Unfollow removes a user from following an entity.
func (s *FollowerService) Unfollow(ctx context.Context, userID, entityType, entityID string) error {
	return s.followerRepo.Unfollow(ctx, userID, entityType, entityID)
}

// GetFollowers returns all user IDs following an entity.
func (s *FollowerService) GetFollowers(ctx context.Context, entityType, entityID string) ([]string, error) {
	return s.followerRepo.GetFollowers(ctx, entityType, entityID)
}

// IsFollowing checks if a user follows an entity.
func (s *FollowerService) IsFollowing(ctx context.Context, userID, entityType, entityID string) (bool, error) {
	return s.followerRepo.IsFollowing(ctx, userID, entityType, entityID)
}

// ListFollowers returns full follower records for an entity.
func (s *FollowerService) ListFollowers(ctx context.Context, entityType, entityID string) ([]model.EntityFollower, error) {
	return s.followerRepo.ListFollowers(ctx, entityType, entityID)
}

// ListUserFollowing returns all entities a user follows in a workspace.
func (s *FollowerService) ListUserFollowing(ctx context.Context, userID, workspaceID string) ([]model.EntityFollower, error) {
	return s.followerRepo.ListUserFollowing(ctx, userID, workspaceID)
}
