package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMDealService contains CRM deal and pipeline business logic.
type CRMDealService struct {
	dealRepo  *repository.CRMDealRepository
	assocRepo *repository.CRMAssociationRepository
}

// NewCRMDealService creates a new CRMDealService.
func NewCRMDealService(dealRepo *repository.CRMDealRepository, assocRepo *repository.CRMAssociationRepository) *CRMDealService {
	return &CRMDealService{dealRepo: dealRepo, assocRepo: assocRepo}
}

// SeedWorkspaceDefaults creates a default sales pipeline for a new workspace.
func (s *CRMDealService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	return s.dealRepo.SeedDefaultPipeline(ctx, workspaceID)
}

// ── Pipeline operations ──

// ListPipelines returns all pipelines in a workspace.
func (s *CRMDealService) ListPipelines(ctx context.Context, workspaceID string) ([]model.CRMPipeline, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.dealRepo.ListPipelines(ctx, workspaceID)
}

// GetPipeline returns a pipeline by ID.
func (s *CRMDealService) GetPipeline(ctx context.Context, id string) (*model.CRMPipeline, error) {
	pipeline, err := s.dealRepo.GetPipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline not found")
	}
	return pipeline, nil
}

// CreatePipeline creates a pipeline with stages.
func (s *CRMDealService) CreatePipeline(ctx context.Context, req model.CreateCRMPipelineRequest) (*model.CRMPipeline, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}

	isDefault := false
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}

	pipeline := &model.CRMPipeline{
		WorkspaceID: req.WorkspaceID,
		Name:        strings.TrimSpace(req.Name),
		IsDefault:   isDefault,
	}

	// Build stages
	for _, s := range req.Stages {
		pipeline.Stages = append(pipeline.Stages, model.CRMPipelineStage{
			Name:        s.Name,
			StageType:   s.StageType,
			Position:    s.Position,
			Probability: s.Probability,
		})
	}

	if err := s.dealRepo.CreatePipeline(ctx, pipeline); err != nil {
		return nil, err
	}
	return s.dealRepo.GetPipeline(ctx, pipeline.ID)
}

// UpdatePipeline updates a pipeline and optionally replaces its stages.
func (s *CRMDealService) UpdatePipeline(ctx context.Context, id string, req model.UpdateCRMPipelineRequest) (*model.CRMPipeline, error) {
	pipeline, err := s.dealRepo.GetPipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		pipeline.Name = name
	}
	if req.IsDefault != nil {
		pipeline.IsDefault = *req.IsDefault
	}

	if err := s.dealRepo.UpdatePipeline(ctx, pipeline); err != nil {
		return nil, err
	}

	if req.Stages != nil {
		var stages []model.CRMPipelineStage
		for _, st := range req.Stages {
			stage := model.CRMPipelineStage{
				Name:        st.Name,
				StageType:   st.StageType,
				Position:    st.Position,
				Probability: st.Probability,
			}
			if st.ID != nil {
				stage.ID = *st.ID
			}
			stages = append(stages, stage)
		}
		if err := s.dealRepo.ReplaceStages(ctx, id, stages); err != nil {
			return nil, err
		}
	}

	return s.dealRepo.GetPipeline(ctx, id)
}

// DeletePipeline removes a pipeline.
func (s *CRMDealService) DeletePipeline(ctx context.Context, id string) error {
	pipeline, err := s.dealRepo.GetPipeline(ctx, id)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return fmt.Errorf("pipeline not found")
	}
	return s.dealRepo.DeletePipeline(ctx, id)
}

// ── Deal operations ──

// List returns deals with filters and pagination.
func (s *CRMDealService) List(ctx context.Context, workspaceID string, filters model.CRMDealListFilters, pagination model.PMPagination) ([]model.CRMDeal, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.dealRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a deal by ID.
func (s *CRMDealService) GetByID(ctx context.Context, id string) (*model.CRMDeal, error) {
	deal, err := s.dealRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if deal == nil {
		return nil, fmt.Errorf("deal not found")
	}
	return deal, nil
}

// Create creates a deal.
func (s *CRMDealService) Create(ctx context.Context, req model.CreateCRMDealRequest) (*model.CRMDeal, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("workspace_id and name are required")
	}
	if req.ContactID == "" {
		return nil, fmt.Errorf("contact_id is required")
	}
	if req.PipelineID == "" || req.StageID == "" {
		return nil, fmt.Errorf("pipeline_id and stage_id are required")
	}

	// Validate pipeline exists and belongs to workspace
	pipeline, err := s.dealRepo.GetPipeline(ctx, req.PipelineID)
	if err != nil {
		return nil, err
	}
	if pipeline == nil || pipeline.WorkspaceID != req.WorkspaceID {
		return nil, fmt.Errorf("pipeline not found in workspace")
	}

	// Validate stage belongs to pipeline
	stage, err := s.dealRepo.GetStage(ctx, req.StageID)
	if err != nil {
		return nil, err
	}
	if stage == nil || stage.PipelineID != req.PipelineID {
		return nil, fmt.Errorf("stage not found in pipeline")
	}

	displayID, err := s.dealRepo.GetNextDisplayID(ctx, req.WorkspaceID)
	if err != nil {
		return nil, err
	}

	currency := "USD"
	if req.Currency != nil && *req.Currency != "" {
		currency = *req.Currency
	}

	deal := &model.CRMDeal{
		WorkspaceID:      req.WorkspaceID,
		DisplayID:        displayID,
		Name:             strings.TrimSpace(req.Name),
		PipelineID:       req.PipelineID,
		StageID:          req.StageID,
		Amount:           req.Amount,
		Currency:         currency,
		CloseDate:        req.CloseDate,
		OwnerMemberID:    req.OwnerMemberID,
		Probability:      req.Probability,
		CustomProperties: model.JSONB(req.CustomProperties),
	}

	if err := s.dealRepo.Create(ctx, deal); err != nil {
		return nil, err
	}

	assoc := &model.CRMAssociation{
		WorkspaceID:    req.WorkspaceID,
		FromObjectType: model.CRMObjectDeal,
		FromObjectID:   deal.ID,
		ToObjectType:   model.CRMObjectContact,
		ToObjectID:     req.ContactID,
	}
	if err := s.assocRepo.Create(ctx, assoc); err != nil {
		return nil, err
	}

	return s.dealRepo.GetByID(ctx, deal.ID)
}

// Update updates a deal.
func (s *CRMDealService) Update(ctx context.Context, id string, req model.UpdateCRMDealRequest) (*model.CRMDeal, error) {
	deal, err := s.dealRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if deal == nil {
		return nil, fmt.Errorf("deal not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		deal.Name = name
	}
	if req.PipelineID != nil {
		deal.PipelineID = *req.PipelineID
	}
	if req.StageID != nil {
		deal.StageID = *req.StageID
	}
	if req.Amount != nil {
		deal.Amount = req.Amount
	}
	if req.Currency != nil {
		deal.Currency = *req.Currency
	}
	if req.CloseDate != nil {
		deal.CloseDate = req.CloseDate
	}
	if req.OwnerMemberID != nil {
		deal.OwnerMemberID = req.OwnerMemberID
	}
	if req.Probability != nil {
		deal.Probability = req.Probability
	}
	if req.CustomProperties != nil {
		deal.CustomProperties = model.JSONB(req.CustomProperties)
	}

	// Clear preloaded associations before save
	deal.Pipeline = nil
	deal.Stage = nil

	if err := s.dealRepo.Update(ctx, deal); err != nil {
		return nil, err
	}
	return s.dealRepo.GetByID(ctx, id)
}

// Delete removes a deal.
func (s *CRMDealService) Delete(ctx context.Context, id string) error {
	deal, err := s.dealRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if deal == nil {
		return fmt.Errorf("deal not found")
	}
	return s.dealRepo.Delete(ctx, id)
}
