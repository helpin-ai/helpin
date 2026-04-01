package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// AgentRunInteractionRepository handles DB operations for run interactions.
type AgentRunInteractionRepository struct {
	db *gorm.DB
}

// NewAgentRunInteractionRepository creates a new AgentRunInteractionRepository.
func NewAgentRunInteractionRepository(db *gorm.DB) *AgentRunInteractionRepository {
	return &AgentRunInteractionRepository{db: db}
}

// ListByRun returns interactions for a run in chronological order.
func (r *AgentRunInteractionRepository) ListByRun(ctx context.Context, workspaceID, runID string) ([]model.AgentRunInteraction, error) {
	var interactions []model.AgentRunInteraction
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ?", workspaceID, runID).
		Order("created_at ASC, id ASC").
		Find(&interactions).Error; err != nil {
		return nil, fmt.Errorf("list run interactions: %w", err)
	}
	return interactions, nil
}

// GetByID returns a run interaction by id.
func (r *AgentRunInteractionRepository) GetByID(ctx context.Context, workspaceID, runID, interactionID string) (*model.AgentRunInteraction, error) {
	var interaction model.AgentRunInteraction
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ? AND id = ?", workspaceID, runID, interactionID).
		First(&interaction).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get run interaction: %w", err)
	}
	return &interaction, nil
}

// GetLatestPendingByRun returns the newest pending interaction for a run.
func (r *AgentRunInteractionRepository) GetLatestPendingByRun(ctx context.Context, workspaceID, runID string) (*model.AgentRunInteraction, error) {
	var interaction model.AgentRunInteraction
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ? AND status = ?", workspaceID, runID, model.AgentRunInteractionStatusPending).
		Order("CASE WHEN assistant_message_sequence_no IS NULL THEN 1 ELSE 0 END ASC").
		Order("assistant_message_sequence_no DESC").
		Order("created_at DESC").
		Order("id DESC").
		First(&interaction).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest pending run interaction: %w", err)
	}
	return &interaction, nil
}

// GetLatestResolvedByRun returns the newest resolved interaction for a run.
func (r *AgentRunInteractionRepository) GetLatestResolvedByRun(ctx context.Context, workspaceID, runID string) (*model.AgentRunInteraction, error) {
	var interaction model.AgentRunInteraction
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND run_id = ? AND status = ?", workspaceID, runID, model.AgentRunInteractionStatusResolved).
		Order("resolved_at DESC").
		Order("updated_at DESC").
		Order("id DESC").
		First(&interaction).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest resolved run interaction: %w", err)
	}
	return &interaction, nil
}

// Create inserts a new interaction.
func (r *AgentRunInteractionRepository) Create(ctx context.Context, interaction *model.AgentRunInteraction) error {
	if err := r.db.WithContext(ctx).Create(interaction).Error; err != nil {
		return fmt.Errorf("create run interaction: %w", err)
	}
	return nil
}

// Update saves changes to an interaction.
func (r *AgentRunInteractionRepository) Update(ctx context.Context, interaction *model.AgentRunInteraction) error {
	if err := r.db.WithContext(ctx).Save(interaction).Error; err != nil {
		return fmt.Errorf("update run interaction: %w", err)
	}
	return nil
}
