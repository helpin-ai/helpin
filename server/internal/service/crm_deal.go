package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMDealService contains CRM deal and pipeline business logic.
type CRMDealService struct {
	productAnalyticsEmitter
	dealRepo  *repository.CRMDealRepository
	assocRepo *repository.CRMAssociationRepository
	activity  *PMActivityService
}

// NewCRMDealService creates a new CRMDealService.
func NewCRMDealService(dealRepo *repository.CRMDealRepository, assocRepo *repository.CRMAssociationRepository) *CRMDealService {
	return &CRMDealService{dealRepo: dealRepo, assocRepo: assocRepo}
}

// SetActivityService enables durable user-facing deal milestone logging.
func (s *CRMDealService) SetActivityService(activity *PMActivityService) *CRMDealService {
	s.activity = activity
	return s
}

// SeedWorkspaceDefaults creates a default sales pipeline for a new workspace.
func (s *CRMDealService) SeedWorkspaceDefaults(ctx context.Context, workspaceID, actorID string) error {
	return s.dealRepo.SeedDefaultPipeline(ctx, workspaceID)
}

// ── Pipeline operations ──

// ListPipelines returns all pipelines in a workspace with deal counts.
func (s *CRMDealService) ListPipelines(ctx context.Context, workspaceID string) ([]model.CRMPipeline, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	pipelines, err := s.dealRepo.ListPipelines(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range pipelines {
		count, err := s.dealRepo.CountDealsByPipeline(ctx, pipelines[i].ID)
		if err != nil {
			return nil, err
		}
		pipelines[i].DealCount = count
	}
	return pipelines, nil
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

// DeletePipeline removes a pipeline if it has no active deals.
func (s *CRMDealService) DeletePipeline(ctx context.Context, id string) error {
	pipeline, err := s.dealRepo.GetPipeline(ctx, id)
	if err != nil {
		return err
	}
	if pipeline == nil {
		return fmt.Errorf("pipeline not found")
	}
	count, err := s.dealRepo.CountDealsByPipeline(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete pipeline with active deals")
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
	return s.create(ctx, req, "")
}

// CreateWithActor creates a deal and attributes its timeline milestone to the actor.
func (s *CRMDealService) CreateWithActor(ctx context.Context, req model.CreateCRMDealRequest, actorID string) (*model.CRMDeal, error) {
	return s.create(ctx, req, actorID)
}

func (s *CRMDealService) create(ctx context.Context, req model.CreateCRMDealRequest, actorID string) (*model.CRMDeal, error) {
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

	created, err := s.dealRepo.GetByID(ctx, deal.ID)
	if err != nil {
		return nil, err
	}
	if s.activity != nil {
		action := "created this deal in " + stage.Name
		if err := s.activity.LogEvent(ctx, deal.WorkspaceID, "deal", deal.ID, optionalActor(actorID), "deal.created", action, stringPtr("stage"), nil, &stage.ID, map[string]interface{}{"stage_name": stage.Name, "stage_type": stage.StageType}); err != nil {
			slog.ErrorContext(ctx, "log deal creation milestone", "error", err, "deal_id", deal.ID, "workspace_id", deal.WorkspaceID)
		}
	}
	s.trackProductEvent(ctx, ProductAnalyticsEvent{
		SemanticKey: "crm_deal_created:" + deal.ID,
		WorkspaceID: deal.WorkspaceID, Name: "crm_deal_created", Source: "api",
		OccurredAt: deal.CreatedAt,
		Attributes: map[string]any{"entity_id": deal.ID, "pipeline_id": deal.PipelineID, "stage_id": deal.StageID, "amount": deal.Amount, "currency": deal.Currency, "module": "crm"},
	})
	return created, nil
}

// Update updates a deal.
func (s *CRMDealService) Update(ctx context.Context, id string, req model.UpdateCRMDealRequest) (*model.CRMDeal, error) {
	return s.update(ctx, id, req, "")
}

// UpdateWithActor updates a deal and attributes timeline milestones to the actor.
func (s *CRMDealService) UpdateWithActor(ctx context.Context, id string, req model.UpdateCRMDealRequest, actorID string) (*model.CRMDeal, error) {
	return s.update(ctx, id, req, actorID)
}

func (s *CRMDealService) update(ctx context.Context, id string, req model.UpdateCRMDealRequest, actorID string) (*model.CRMDeal, error) {
	deal, err := s.dealRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if deal == nil {
		return nil, fmt.Errorf("deal not found")
	}

	previousStageID := deal.StageID
	previousStageName := previousStageID
	if deal.Stage != nil {
		previousStageName = deal.Stage.Name
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
	if req.ClearAmount {
		deal.Amount = nil
	} else if req.Amount != nil {
		deal.Amount = req.Amount
	}
	if req.Currency != nil {
		deal.Currency = *req.Currency
	}
	if req.ClearCloseDate {
		deal.CloseDate = nil
	} else if req.CloseDate != nil {
		deal.CloseDate = req.CloseDate
	}
	if req.ClearOwner {
		deal.OwnerMemberID = nil
	} else if req.OwnerMemberID != nil {
		deal.OwnerMemberID = req.OwnerMemberID
	}
	if req.ClearProbability {
		deal.Probability = nil
	} else if req.Probability != nil {
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
	updated, err := s.dealRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if previousStageID != updated.StageID {
		stageType := ""
		stageName := updated.StageID
		if updated.Stage != nil {
			stageType = updated.Stage.StageType
			stageName = updated.Stage.Name
		}
		if s.activity != nil {
			eventType := "deal.stage_changed"
			if stageType == model.CRMStageTypeWon {
				eventType = "deal.won"
			} else if stageType == model.CRMStageTypeLost {
				eventType = "deal.lost"
			}
			action := "moved this deal from " + previousStageName + " to " + stageName
			if err := s.activity.LogEvent(ctx, updated.WorkspaceID, "deal", updated.ID, optionalActor(actorID), eventType, action, stringPtr("stage"), &previousStageID, &updated.StageID, map[string]interface{}{"previous_stage_name": previousStageName, "stage_name": stageName, "stage_type": stageType}); err != nil {
				slog.ErrorContext(ctx, "log deal stage milestone", "error", err, "deal_id", updated.ID, "workspace_id", updated.WorkspaceID)
			}
		}
		s.trackProductEvent(ctx, ProductAnalyticsEvent{
			SemanticKey: fmt.Sprintf("crm_deal_stage_changed:%s:%s:%d", updated.ID, updated.StageID, updated.UpdatedAt.UnixNano()),
			WorkspaceID: updated.WorkspaceID, Name: "crm_deal_stage_changed", Source: "api",
			OccurredAt: updated.UpdatedAt,
			Attributes: map[string]any{"entity_id": updated.ID, "pipeline_id": updated.PipelineID, "previous_stage_id": previousStageID, "stage_id": updated.StageID, "stage_type": stageType, "amount": updated.Amount, "currency": updated.Currency, "module": "crm"},
		})
	}
	return updated, nil
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
