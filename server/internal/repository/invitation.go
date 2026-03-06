package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// InvitationRepository handles database operations for workspace invitations.
type InvitationRepository struct {
	db *gorm.DB
}

// NewInvitationRepository creates a new InvitationRepository.
func NewInvitationRepository(db *gorm.DB) *InvitationRepository {
	return &InvitationRepository{db: db}
}

// Create inserts a new invitation.
func (r *InvitationRepository) Create(ctx context.Context, inv *model.WorkspaceInvitation) (*model.WorkspaceInvitation, error) {
	if err := r.db.WithContext(ctx).Create(inv).Error; err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}
	return inv, nil
}

// GetByToken retrieves an invitation by its token.
func (r *InvitationRepository) GetByToken(ctx context.Context, token string) (*model.WorkspaceInvitation, error) {
	inv := &model.WorkspaceInvitation{}
	err := r.db.WithContext(ctx).Where("token = ?", token).First(inv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get invitation by token: %w", err)
	}
	return inv, nil
}

// GetByID retrieves an invitation by its ID.
func (r *InvitationRepository) GetByID(ctx context.Context, id string) (*model.WorkspaceInvitation, error) {
	inv := &model.WorkspaceInvitation{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(inv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get invitation by id: %w", err)
	}
	return inv, nil
}

// ListByWorkspace returns all invitations for a workspace.
func (r *InvitationRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.WorkspaceInvitation, error) {
	var invitations []model.WorkspaceInvitation
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at DESC").Find(&invitations).Error
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	return invitations, nil
}

// GetPendingByEmail returns a pending invitation for a specific email in a workspace.
func (r *InvitationRepository) GetPendingByEmail(ctx context.Context, workspaceID, email string) (*model.WorkspaceInvitation, error) {
	inv := &model.WorkspaceInvitation{}
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND LOWER(email) = LOWER(?) AND status = 'pending'", workspaceID, email).
		First(inv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending invitation: %w", err)
	}
	return inv, nil
}

// UpdateStatus updates the status (and optionally accepted_at) of an invitation.
func (r *InvitationRepository) UpdateStatus(ctx context.Context, id, status string, acceptedAt *time.Time) error {
	updates := map[string]interface{}{"status": status}
	if acceptedAt != nil {
		updates["accepted_at"] = acceptedAt
	}
	if err := r.db.WithContext(ctx).Model(&model.WorkspaceInvitation{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update invitation status: %w", err)
	}
	return nil
}

// UpdateTokenAndExpiry updates the token and expiry of an invitation.
func (r *InvitationRepository) UpdateTokenAndExpiry(ctx context.Context, id, token string, expiresAt time.Time) error {
	updates := map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt,
	}
	if err := r.db.WithContext(ctx).Model(&model.WorkspaceInvitation{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update invitation token: %w", err)
	}
	return nil
}

// GetByTokenWithDetails returns an invitation with workspace and inviter details.
func (r *InvitationRepository) GetByTokenWithDetails(ctx context.Context, token string) (*model.InvitationWithDetails, error) {
	result := &model.InvitationWithDetails{}
	err := r.db.WithContext(ctx).
		Table("workspace_invitations").
		Select("workspace_invitations.*, workspaces.name as workspace_name, workspaces.slug as workspace_slug, users.full_name as inviter_name").
		Joins("JOIN workspaces ON workspaces.id = workspace_invitations.workspace_id").
		Joins("JOIN users ON users.id = workspace_invitations.invited_by").
		Where("workspace_invitations.token = ?", token).
		Scan(result).Error
	if err != nil {
		return nil, fmt.Errorf("get invitation with details: %w", err)
	}
	if result.ID == "" {
		return nil, nil
	}
	return result, nil
}
