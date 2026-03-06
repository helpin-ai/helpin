package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// WorkspaceRepository handles database operations for workspaces and workspace members.
type WorkspaceRepository struct {
	db *gorm.DB
}

// NewWorkspaceRepository creates a new WorkspaceRepository.
func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

// Create inserts a new workspace.
func (r *WorkspaceRepository) Create(ctx context.Context, name, slug, ownerID string, description *string, timezone string) (*model.Workspace, error) {
	ws := &model.Workspace{
		Name:        name,
		Slug:        slug,
		OwnerID:     ownerID,
		Description: description,
		Timezone:    timezone,
	}
	if err := r.db.WithContext(ctx).Create(ws).Error; err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return ws, nil
}

// List returns all workspaces a user is a member of, along with their role.
func (r *WorkspaceRepository) List(ctx context.Context, userID string) ([]model.WorkspaceWithRole, error) {
	var results []model.WorkspaceWithRole
	err := r.db.WithContext(ctx).
		Table("workspaces w").
		Select("w.id, w.name, w.slug, w.owner_id, w.description, w.timezone, w.created_at, w.updated_at, wm.role").
		Joins("JOIN workspace_members wm ON w.id = wm.workspace_id").
		Where("wm.user_id = ?", userID).
		Order("w.created_at DESC").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	return results, nil
}

// GetByID returns a workspace by its ID.
func (r *WorkspaceRepository) GetByID(ctx context.Context, id string) (*model.Workspace, error) {
	ws := &model.Workspace{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(ws).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace by id: %w", err)
	}
	return ws, nil
}

// GetBySlug returns a workspace by its slug.
func (r *WorkspaceRepository) GetBySlug(ctx context.Context, slug string) (*model.Workspace, error) {
	ws := &model.Workspace{}
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(ws).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get workspace by slug: %w", err)
	}
	return ws, nil
}

// Update modifies workspace fields.
func (r *WorkspaceRepository) Update(ctx context.Context, id string, name, description, timezone *string) (*model.Workspace, error) {
	updates := map[string]interface{}{}
	if name != nil {
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if timezone != nil {
		updates["timezone"] = *timezone
	}

	if err := r.db.WithContext(ctx).Model(&model.Workspace{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}

	ws := &model.Workspace{}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(ws).Error; err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}
	return ws, nil
}

// Delete removes a workspace by ID.
func (r *WorkspaceRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Workspace{}).Error; err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

// AddMember adds a user as a member of a workspace.
func (r *WorkspaceRepository) AddMember(ctx context.Context, workspaceID, userID, role string) (*model.WorkspaceMember, error) {
	m := &model.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "workspace_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"role"}),
		}).
		Create(m).Error
	if err != nil {
		return nil, fmt.Errorf("add workspace member: %w", err)
	}
	// Re-fetch to get the correct ID and timestamps after upsert.
	result := &model.WorkspaceMember{}
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(result).Error; err != nil {
		return nil, fmt.Errorf("add workspace member: %w", err)
	}
	return result, nil
}

// GetMemberRole returns the role a user has in a workspace, or empty string if not a member.
func (r *WorkspaceRepository) GetMemberRole(ctx context.Context, workspaceID, userID string) (string, error) {
	var m model.WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("get member role: %w", err)
	}
	return m.Role, nil
}

// ListMembers returns all members of a workspace with user details.
func (r *WorkspaceRepository) ListMembers(ctx context.Context, workspaceID string) ([]model.MemberWithUser, error) {
	var results []model.MemberWithUser
	err := r.db.WithContext(ctx).
		Table("workspace_members wm").
		Select("wm.id, wm.user_id, wm.role, u.email, u.full_name, u.avatar_url").
		Joins("JOIN users u ON u.id = wm.user_id").
		Where("wm.workspace_id = ?", workspaceID).
		Order("u.full_name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	return results, nil
}

// GetMembership returns the full membership record for a user in a workspace.
func (r *WorkspaceRepository) GetMembership(ctx context.Context, workspaceID, userID string) (*model.WorkspaceMember, error) {
	m := &model.WorkspaceMember{}
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return m, nil
}
