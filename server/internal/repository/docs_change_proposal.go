package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

type DocsChangeProposalRepository struct {
	db *gorm.DB
}

func NewDocsChangeProposalRepository(db *gorm.DB) *DocsChangeProposalRepository {
	return &DocsChangeProposalRepository{db: db}
}

func (r *DocsChangeProposalRepository) Create(ctx context.Context, proposal *model.DocsChangeProposal) (*model.DocsChangeProposal, error) {
	if err := r.db.WithContext(ctx).Create(proposal).Error; err != nil {
		return nil, fmt.Errorf("create docs change proposal: %w", err)
	}
	return proposal, nil
}

func (r *DocsChangeProposalRepository) GetByID(ctx context.Context, workspaceID, id string) (*model.DocsChangeProposal, error) {
	var proposal model.DocsChangeProposal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		First(&proposal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs change proposal: %w", err)
	}
	return &proposal, nil
}

func (r *DocsChangeProposalRepository) GetByDocumentIDAndID(ctx context.Context, workspaceID, documentID, id string) (*model.DocsChangeProposal, error) {
	var proposal model.DocsChangeProposal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND document_id = ? AND id = ?", workspaceID, documentID, id).
		First(&proposal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get docs change proposal by document: %w", err)
	}
	return &proposal, nil
}

func (r *DocsChangeProposalRepository) ListPendingByDocument(ctx context.Context, workspaceID, documentID string) ([]model.DocsChangeProposal, error) {
	var proposals []model.DocsChangeProposal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND document_id = ? AND status = ?", workspaceID, documentID, model.DocsChangeProposalStatusPending).
		Order("created_at DESC").
		Find(&proposals).Error; err != nil {
		return nil, fmt.Errorf("list docs change proposals: %w", err)
	}
	return proposals, nil
}

func (r *DocsChangeProposalRepository) Resolve(ctx context.Context, workspaceID, id, status, actorID string) error {
	updates := map[string]any{
		"status":      status,
		"resolved_by": actorID,
		"resolved_at": time.Now().UTC(),
	}
	result := r.db.WithContext(ctx).
		Model(&model.DocsChangeProposal{}).
		Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, id, model.DocsChangeProposalStatusPending).
		Updates(updates)
	if err := result.Error; err != nil {
		return fmt.Errorf("resolve docs change proposal: %w", err)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("proposal not found")
	}
	return nil
}
