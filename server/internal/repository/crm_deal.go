package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// CRMDealRepository handles DB operations for CRM deals and pipelines.
type CRMDealRepository struct {
	db *gorm.DB
}

// NewCRMDealRepository creates a new CRMDealRepository.
func NewCRMDealRepository(db *gorm.DB) *CRMDealRepository {
	return &CRMDealRepository{db: db}
}

// ── Pipeline operations ──

// ListPipelines returns all pipelines in a workspace.
func (r *CRMDealRepository) ListPipelines(ctx context.Context, workspaceID string) ([]model.CRMPipeline, error) {
	var pipelines []model.CRMPipeline
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Preload("Stages", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Order("position ASC").
		Find(&pipelines).Error; err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	return pipelines, nil
}

// GetPipeline returns a pipeline by ID with its stages.
func (r *CRMDealRepository) GetPipeline(ctx context.Context, id string) (*model.CRMPipeline, error) {
	var pipeline model.CRMPipeline
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Preload("Stages", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&pipeline).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pipeline: %w", err)
	}
	return &pipeline, nil
}

// CreatePipeline inserts a pipeline with its stages.
func (r *CRMDealRepository) CreatePipeline(ctx context.Context, pipeline *model.CRMPipeline) error {
	if err := r.db.WithContext(ctx).Create(pipeline).Error; err != nil {
		return fmt.Errorf("create pipeline: %w", err)
	}
	return nil
}

// UpdatePipeline updates a pipeline.
func (r *CRMDealRepository) UpdatePipeline(ctx context.Context, pipeline *model.CRMPipeline) error {
	if err := r.db.WithContext(ctx).Save(pipeline).Error; err != nil {
		return fmt.Errorf("update pipeline: %w", err)
	}
	return nil
}

// DeletePipeline removes a pipeline and its stages.
func (r *CRMDealRepository) DeletePipeline(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("pipeline_id = ?", id).Delete(&model.CRMPipelineStage{}).Error; err != nil {
			return fmt.Errorf("delete pipeline stages: %w", err)
		}
		if err := tx.Where("id = ?", id).Delete(&model.CRMPipeline{}).Error; err != nil {
			return fmt.Errorf("delete pipeline: %w", err)
		}
		return nil
	})
}

// ReplaceStages replaces all stages in a pipeline.
func (r *CRMDealRepository) ReplaceStages(ctx context.Context, pipelineID string, stages []model.CRMPipelineStage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("pipeline_id = ?", pipelineID).Delete(&model.CRMPipelineStage{}).Error; err != nil {
			return fmt.Errorf("clear pipeline stages: %w", err)
		}
		for i := range stages {
			stages[i].PipelineID = pipelineID
			if err := tx.Create(&stages[i]).Error; err != nil {
				return fmt.Errorf("create pipeline stage: %w", err)
			}
		}
		return nil
	})
}

// GetStage returns a pipeline stage by ID.
func (r *CRMDealRepository) GetStage(ctx context.Context, id string) (*model.CRMPipelineStage, error) {
	var stage model.CRMPipelineStage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&stage).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get stage: %w", err)
	}
	return &stage, nil
}

// CountDealsByPipeline returns the number of deals in a pipeline.
func (r *CRMDealRepository) CountDealsByPipeline(ctx context.Context, pipelineID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMDeal{}).Where("pipeline_id = ?", pipelineID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count deals by pipeline: %w", err)
	}
	return count, nil
}

// ── Deal operations ──

// GetNextDisplayID generates the next sequential display ID for deals in a workspace.
func (r *CRMDealRepository) GetNextDisplayID(ctx context.Context, workspaceID string) (string, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMDeal{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		return "", fmt.Errorf("count deals: %w", err)
	}
	return fmt.Sprintf("DEAL-%d", count+1), nil
}

// List returns deals in a workspace with optional filters.
func (r *CRMDealRepository) List(ctx context.Context, workspaceID string, filters model.CRMDealListFilters, pagination model.PMPagination) ([]model.CRMDeal, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CRMDeal{}).Where("crm_deals.workspace_id = ?", workspaceID)

	if filters.PipelineID != nil && *filters.PipelineID != "" {
		query = query.Where("pipeline_id = ?", *filters.PipelineID)
	}
	if filters.StageID != nil && *filters.StageID != "" {
		query = query.Where("stage_id = ?", *filters.StageID)
	}
	if filters.OwnerMemberID != nil && *filters.OwnerMemberID != "" {
		query = query.Where("owner_member_id = ?", *filters.OwnerMemberID)
	}
	if filters.Search != nil && *filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(*filters.Search)) + "%"
		query = query.Where("LOWER(name) LIKE ?", search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count deals: %w", err)
	}

	var deals []model.CRMDeal
	offset := (pagination.Page - 1) * pagination.PerPage
	if pagination.Offset != nil {
		offset = *pagination.Offset
	}
	if err := query.
		Preload("Pipeline").
		Preload("Stage").
		Order("created_at DESC").
		Offset(offset).Limit(pagination.PerPage).
		Find(&deals).Error; err != nil {
		return nil, 0, fmt.Errorf("list deals: %w", err)
	}
	return deals, total, nil
}

// ListByIDs returns deals by ID for a workspace.
func (r *CRMDealRepository) ListByIDs(ctx context.Context, workspaceID string, ids []string) ([]model.CRMDeal, error) {
	if len(ids) == 0 {
		return []model.CRMDeal{}, nil
	}
	var deals []model.CRMDeal
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND id IN ?", workspaceID, ids).
		Find(&deals).Error; err != nil {
		return nil, fmt.Errorf("list deals by ids: %w", err)
	}
	return deals, nil
}

// GetByID returns a deal by ID with pipeline and stage.
func (r *CRMDealRepository) GetByID(ctx context.Context, id string) (*model.CRMDeal, error) {
	var deal model.CRMDeal
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Preload("Pipeline").
		Preload("Stage").
		First(&deal).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get deal: %w", err)
	}
	return &deal, nil
}

// Create inserts a deal.
func (r *CRMDealRepository) Create(ctx context.Context, deal *model.CRMDeal) error {
	if err := r.db.WithContext(ctx).Create(deal).Error; err != nil {
		return fmt.Errorf("create deal: %w", err)
	}
	return nil
}

// Update updates a deal.
func (r *CRMDealRepository) Update(ctx context.Context, deal *model.CRMDeal) error {
	if err := r.db.WithContext(ctx).Save(deal).Error; err != nil {
		return fmt.Errorf("update deal: %w", err)
	}
	return nil
}

// Delete removes a deal.
func (r *CRMDealRepository) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.CRMDeal{}).Error; err != nil {
		return fmt.Errorf("delete deal: %w", err)
	}
	return nil
}

// SeedDefaultPipeline creates a default "Sales Pipeline" with HubSpot-standard stages
// if the workspace has no pipelines yet.
func (r *CRMDealRepository) SeedDefaultPipeline(ctx context.Context, workspaceID string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.CRMPipeline{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
		return fmt.Errorf("count pipelines: %w", err)
	}
	if count > 0 {
		return nil
	}

	pipeline := &model.CRMPipeline{
		WorkspaceID: workspaceID,
		Name:        "Sales Pipeline",
		IsDefault:   true,
		Stages: []model.CRMPipelineStage{
			{Name: "Appointment Scheduled", StageType: "open", Position: 0, Probability: 20},
			{Name: "Qualified to Buy", StageType: "open", Position: 1, Probability: 40},
			{Name: "Presentation Scheduled", StageType: "open", Position: 2, Probability: 60},
			{Name: "Decision Maker Bought-In", StageType: "open", Position: 3, Probability: 80},
			{Name: "Contract Sent", StageType: "open", Position: 4, Probability: 90},
			{Name: "Closed Won", StageType: "won", Position: 5, Probability: 100},
			{Name: "Closed Lost", StageType: "lost", Position: 6, Probability: 0},
		},
	}

	if err := r.db.WithContext(ctx).Create(pipeline).Error; err != nil {
		return fmt.Errorf("seed default pipeline: %w", err)
	}
	return nil
}
