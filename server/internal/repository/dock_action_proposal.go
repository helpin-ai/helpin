package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// DockActionProposalRepository persists scoped Dock execution proposals.
type DockActionProposalRepository struct {
	db *gorm.DB
}

// NewDockActionProposalRepository creates a Dock action proposal repository.
func NewDockActionProposalRepository(db *gorm.DB) *DockActionProposalRepository {
	return &DockActionProposalRepository{db: db}
}

// Create inserts a prepared proposal.
func (r *DockActionProposalRepository) Create(ctx context.Context, proposal *model.DockActionProposal) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("dock action proposal repository is not configured")
	}
	if err := r.db.WithContext(ctx).Create(proposal).Error; err != nil {
		return fmt.Errorf("create dock action proposal: %w", err)
	}
	return nil
}

// GetByID returns one workspace-scoped proposal.
func (r *DockActionProposalRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.DockActionProposal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("dock action proposal repository is not configured")
	}
	var proposal model.DockActionProposal
	if err := r.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&proposal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get dock action proposal: %w", err)
	}
	return &proposal, nil
}

// ListActiveForRun returns unexpired active grants for one Dock backing run.
func (r *DockActionProposalRepository) ListActiveForRun(ctx context.Context, workspaceID, runID string, now time.Time) ([]model.DockActionProposal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("dock action proposal repository is not configured")
	}
	var proposals []model.DockActionProposal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dock_chat_run_id = ? AND status = ? AND expires_at > ?", workspaceID, runID, model.DockActionProposalStatusActive, now).
		Order("activated_at DESC, created_at DESC").
		Find(&proposals).Error; err != nil {
		return nil, fmt.Errorf("list active dock action proposals: %w", err)
	}
	return proposals, nil
}

// ListPreparedForRun returns unexpired proposals awaiting approval for one
// Dock backing run. Callers use this to reuse an identical recovery proposal
// instead of creating duplicates when a model retries a guarded mutation.
func (r *DockActionProposalRepository) ListPreparedForRun(ctx context.Context, workspaceID, runID string, now time.Time) ([]model.DockActionProposal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("dock action proposal repository is not configured")
	}
	var proposals []model.DockActionProposal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND dock_chat_run_id = ? AND status = ? AND expires_at > ?", workspaceID, runID, model.DockActionProposalStatusPrepared, now).
		Order("created_at DESC").
		Find(&proposals).Error; err != nil {
		return nil, fmt.Errorf("list prepared dock action proposals: %w", err)
	}
	return proposals, nil
}

// Activate atomically binds a prepared proposal to one approval interaction.
func (r *DockActionProposalRepository) Activate(ctx context.Context, workspaceID, id, approvalInteractionID string, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.DockActionProposal{}).
		Where("workspace_id = ? AND id = ? AND status = ? AND expires_at > ?", workspaceID, id, model.DockActionProposalStatusPrepared, now).
		Updates(map[string]interface{}{
			"status":                  model.DockActionProposalStatusActive,
			"approval_interaction_id": approvalInteractionID,
			"activated_at":            now,
		})
	if result.Error != nil {
		return false, fmt.Errorf("activate dock action proposal: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// ConsumeUsage atomically replaces the usage counters when they still match
// the caller's snapshot. This prevents concurrent Dock calls from both
// consuming the same final allowance.
func (r *DockActionProposalRepository) ConsumeUsage(ctx context.Context, proposal *model.DockActionProposal, previous json.RawMessage) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.DockActionProposal{}).
		Where("workspace_id = ? AND id = ? AND status = ? AND usage = ?", proposal.WorkspaceID, proposal.ID, model.DockActionProposalStatusActive, previous).
		Update("usage", proposal.Usage)
	if result.Error != nil {
		return false, fmt.Errorf("consume dock action proposal usage: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// Complete closes an active proposal after its expected operations ran.
func (r *DockActionProposalRepository) Complete(ctx context.Context, workspaceID, id string, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.DockActionProposal{}).
		Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, id, model.DockActionProposalStatusActive).
		Updates(map[string]interface{}{"status": model.DockActionProposalStatusCompleted, "completed_at": now})
	if result.Error != nil {
		return false, fmt.Errorf("complete dock action proposal: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// ResetActivation returns a claimed proposal to prepared when its downstream
// mutation failed before producing a result, allowing the same unconsumed
// approval to be retried safely.
func (r *DockActionProposalRepository) ResetActivation(ctx context.Context, workspaceID, id, approvalInteractionID string) error {
	if err := r.db.WithContext(ctx).Model(&model.DockActionProposal{}).
		Where("workspace_id = ? AND id = ? AND status = ? AND approval_interaction_id = ?", workspaceID, id, model.DockActionProposalStatusActive, approvalInteractionID).
		Updates(map[string]interface{}{"status": model.DockActionProposalStatusPrepared, "approval_interaction_id": nil, "activated_at": nil}).Error; err != nil {
		return fmt.Errorf("reset dock action proposal activation: %w", err)
	}
	return nil
}
