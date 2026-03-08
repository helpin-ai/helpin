package authorization

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// GORMMemberRepository implements MemberRepository using GORM.
type GORMMemberRepository struct {
	db *gorm.DB
}

// NewGORMMemberRepository creates a new GORM-backed MemberRepository.
func NewGORMMemberRepository(db *gorm.DB) *GORMMemberRepository {
	return &GORMMemberRepository{db: db}
}

// GetMembership returns the membership info for a user in a workspace.
func (r *GORMMemberRepository) GetMembership(ctx context.Context, workspaceID, userID string) (*MemberInfo, error) {
	var m model.WorkspaceMember
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return &MemberInfo{
		ID:     m.ID,
		Role:   m.Role,
		Status: m.Status,
	}, nil
}

// GetTeamMemberships returns all team memberships for a workspace member.
func (r *GORMMemberRepository) GetTeamMemberships(ctx context.Context, workspaceMemberID string) ([]TeamRole, error) {
	var memberships []model.TeamWorkspaceMembership
	err := r.db.WithContext(ctx).
		Where("workspace_member_id = ?", workspaceMemberID).
		Find(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("get team memberships: %w", err)
	}

	roles := make([]TeamRole, len(memberships))
	for i, m := range memberships {
		roles[i] = TeamRole{
			TeamID: m.TeamID,
			Role:   m.Role,
		}
	}
	return roles, nil
}
