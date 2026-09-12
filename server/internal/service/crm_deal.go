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
	dealRepo       *repository.CRMDealRepository
	assocRepo      *repository.CRMAssociationRepository
	activity       *PMActivityService
	timelineRepo   *repository.CRMCompanyTimelineRepository
	summaryRefresh CompanySummaryRefreshRequester
	motionSignals  interface {
		ReconcileDealMotionSignals(ctx context.Context, workspaceID, dealID string) error
		ReconcilePipelineMotionSignals(ctx context.Context, workspaceID, pipelineID string) error
	}
}

// SetCompanySummaryRefresh enables account-summary invalidation after deal changes.
func (s *CRMDealService) SetCompanySummaryRefresh(refresh CompanySummaryRefreshRequester) *CRMDealService {
	s.summaryRefresh = refresh
	return s
}

// SetMotionSignalReconciler enables immediate signal supersession on motion exit.
func (s *CRMDealService) SetMotionSignalReconciler(reconciler interface {
	ReconcileDealMotionSignals(ctx context.Context, workspaceID, dealID string) error
	ReconcilePipelineMotionSignals(ctx context.Context, workspaceID, pipelineID string) error
}) *CRMDealService {
	s.motionSignals = reconciler
	return s
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

// SetTimelineRepository enables the unified deal timeline read model.
func (s *CRMDealService) SetTimelineRepository(
	repo *repository.CRMCompanyTimelineRepository,
) *CRMDealService {
	s.timelineRepo = repo
	return s
}

// ListTimeline returns one cursor-paginated page of events directly related to a deal.
func (s *CRMDealService) ListTimeline(
	ctx context.Context,
	workspaceID, dealID, filter, cursor string,
	limit int,
) (*model.CRMTimelinePage, error) {
	if workspaceID == "" || dealID == "" {
		return nil, fmt.Errorf("workspace_id and deal_id are required")
	}
	if s.timelineRepo == nil {
		return nil, fmt.Errorf("deal timeline is not configured")
	}
	deal, err := s.GetByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if deal.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("deal not found")
	}
	filter = strings.TrimSpace(filter)
	if filter == "" {
		filter = model.CRMTimelineFilterAll
	}
	if !validCRMTimelineFilter(filter) {
		return nil, fmt.Errorf("invalid timeline filter")
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	query := model.CRMTimelineQuery{Filter: filter, Limit: limit + 1}
	if cursor != "" {
		decoded, err := decodeCRMTimelineCursor(cursor)
		if err != nil {
			return nil, err
		}
		query.CursorAt = &decoded.At
		query.CursorID = decoded.ID
	}
	items, err := s.timelineRepo.ListDeal(ctx, workspaceID, dealID, query)
	if err != nil {
		return nil, err
	}
	page := &model.CRMTimelinePage{Data: items}
	if len(items) > limit {
		page.Data = items[:limit]
		last := page.Data[len(page.Data)-1]
		next, err := encodeCRMTimelineCursor(crmTimelineCursor{
			Version: 1,
			At:      last.OccurredAt,
			ID:      last.ID,
		})
		if err != nil {
			return nil, err
		}
		page.NextCursor = &next
	}
	return page, nil
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
	return pipelines, nil
}

// GetPipeline returns a pipeline by ID.
func (s *CRMDealService) GetPipeline(ctx context.Context, id string) (*model.CRMPipeline, error) {
	pipeline, err := s.dealRepo.GetPipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		return nil, model.ErrCRMPipelineNotFound
	}
	return pipeline, nil
}

// CreatePipeline creates a pipeline with stages.
func (s *CRMDealService) CreatePipeline(ctx context.Context, req model.CreateCRMPipelineRequest) (*model.CRMPipeline, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Name) == "" {
		return nil, &model.CRMPipelineValidationError{Message: "workspace_id and name are required"}
	}

	isDefault := false
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}
	defaultMotion := model.CRMDealMotionNewBusiness
	if req.DefaultCommercialMotion != nil {
		defaultMotion = strings.TrimSpace(*req.DefaultCommercialMotion)
	}
	if !validCRMDealCommercialMotion(defaultMotion) {
		return nil, &model.CRMPipelineValidationError{Message: "invalid default_commercial_motion"}
	}

	pipeline := &model.CRMPipeline{
		WorkspaceID:             req.WorkspaceID,
		Name:                    strings.TrimSpace(req.Name),
		IsDefault:               isDefault,
		DefaultCommercialMotion: defaultMotion,
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

// UpdatePipeline updates metadata and stages atomically while preserving deal references.
func (s *CRMDealService) UpdatePipeline(ctx context.Context, id string, req model.UpdateCRMPipelineRequest) (*model.CRMPipeline, error) {
	pipeline, err := s.GetPipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, &model.CRMPipelineValidationError{Message: "name cannot be empty"}
		}
		pipeline.Name = name
	}
	if req.IsDefault != nil {
		pipeline.IsDefault = *req.IsDefault
	}
	if req.DefaultCommercialMotion != nil {
		motion := strings.TrimSpace(*req.DefaultCommercialMotion)
		if !validCRMDealCommercialMotion(motion) {
			return nil, &model.CRMPipelineValidationError{Message: "invalid default_commercial_motion"}
		}
		pipeline.DefaultCommercialMotion = motion
	}
	opts := repository.CRMPipelineUpdateOptions{StageMigrations: req.StageMigrations, ExpectedUpdatedAt: req.ExpectedUpdatedAt}
	if req.Stages != nil {
		opts.Stages = make([]model.CRMPipelineStage, 0, len(req.Stages))
		for _, item := range req.Stages {
			stage := model.CRMPipelineStage{Name: item.Name, StageType: item.StageType, Position: item.Position, Probability: item.Probability}
			if item.ID != nil {
				if strings.TrimSpace(*item.ID) == "" {
					return nil, &model.CRMPipelineValidationError{Message: "stage ID cannot be empty; omit it for a new stage"}
				}
				stage.ID = *item.ID
			}
			opts.Stages = append(opts.Stages, stage)
		}
		if err := model.ValidateCRMPipelineStages(opts.Stages); err != nil {
			return nil, err
		}
	}
	if err := s.dealRepo.UpdatePipeline(ctx, pipeline, opts); err != nil {
		return nil, err
	}
	if s.motionSignals != nil {
		if err := s.motionSignals.ReconcilePipelineMotionSignals(ctx, pipeline.WorkspaceID, pipeline.ID); err != nil {
			slog.ErrorContext(ctx, "reconcile pipeline commercial-motion signals", "error", err, "pipeline_id", pipeline.ID)
		}
	}
	return s.dealRepo.GetPipeline(ctx, id)
}

// DeletePipeline removes an empty pipeline, protecting its default status transactionally.
func (s *CRMDealService) DeletePipeline(ctx context.Context, id string) error {
	if _, err := s.GetPipeline(ctx, id); err != nil {
		return err
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
	revenueType := req.RevenueType
	if revenueType == "" {
		revenueType = "one_time"
	}
	if !validCRMRevenueType(revenueType) {
		return nil, fmt.Errorf("invalid revenue_type")
	}
	for _, id := range req.ContactIDs {
		exists, err := s.dealRepo.ObjectExists(ctx, req.WorkspaceID, model.CRMObjectContact, id)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("participant not found in workspace")
		}
	}
	customer, err := s.resolveCustomer(ctx, req.WorkspaceID, req.ContactID, req.CompanyID)
	if err != nil {
		return nil, err
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

	var commercialMotion *string
	if req.CommercialMotion != nil {
		motion := strings.TrimSpace(*req.CommercialMotion)
		if !validCRMDealCommercialMotion(motion) {
			return nil, fmt.Errorf("invalid commercial_motion")
		}
		commercialMotion = &motion
	}
	deal := &model.CRMDeal{
		RevenueType:      revenueType,
		WorkspaceID:      req.WorkspaceID,
		DisplayID:        displayID,
		Name:             strings.TrimSpace(req.Name),
		PipelineID:       req.PipelineID,
		StageID:          req.StageID,
		Amount:           req.Amount,
		Currency:         currency,
		CloseDate:        req.CloseDate,
		OwnerMemberID:    req.OwnerMemberID,
		CommercialMotion: commercialMotion,
		Probability:      req.Probability,
		CustomProperties: model.JSONB(req.CustomProperties),
	}
	if err := s.dealRepo.CreateWithCustomer(ctx, deal, customer, req.ContactIDs...); err != nil {
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
		Attributes: map[string]any{"entity_id": deal.ID, "pipeline_id": deal.PipelineID, "stage_id": deal.StageID, "amount": deal.Amount, "currency": deal.Currency, "customer_type": customer.CustomerType, "customer_id": customer.CustomerID, "module": "crm"},
	})
	s.requestCompanySummaryRefresh(ctx, deal.WorkspaceID, deal.ID)
	return created, nil
}

func (s *CRMDealService) resolveCustomer(ctx context.Context, workspaceID, contactID, companyID string) (model.CRMDealCustomer, error) {
	contactID = strings.TrimSpace(contactID)
	companyID = strings.TrimSpace(companyID)
	if contactID == "" && companyID == "" {
		return model.CRMDealCustomer{}, fmt.Errorf("contact_id or company_id is required")
	}
	if contactID != "" {
		exists, err := s.dealRepo.ObjectExists(ctx, workspaceID, model.CRMObjectContact, contactID)
		if err != nil {
			return model.CRMDealCustomer{}, err
		}
		if !exists {
			return model.CRMDealCustomer{}, fmt.Errorf("contact not found in workspace")
		}
	}
	if companyID != "" {
		exists, err := s.dealRepo.ObjectExists(ctx, workspaceID, model.CRMObjectCompany, companyID)
		if err != nil {
			return model.CRMDealCustomer{}, err
		}
		if !exists {
			return model.CRMDealCustomer{}, fmt.Errorf("company not found in workspace")
		}
		return model.CRMDealCustomer{CustomerType: model.CRMObjectCompany, CustomerID: companyID, PrimaryContactID: contactID}, nil
	}
	primaryCompanyID, err := s.dealRepo.GetPrimaryCompanyIDForContact(ctx, workspaceID, contactID)
	if err != nil {
		return model.CRMDealCustomer{}, err
	}
	if primaryCompanyID != "" {
		return model.CRMDealCustomer{CustomerType: model.CRMObjectCompany, CustomerID: primaryCompanyID, PrimaryContactID: contactID}, nil
	}
	return model.CRMDealCustomer{CustomerType: model.CRMObjectContact, CustomerID: contactID, PrimaryContactID: contactID}, nil
}

// GetCustomer returns the canonical customer relationship for a deal.
func (s *CRMDealService) GetCustomer(ctx context.Context, workspaceID, dealID string) (*model.CRMDealCustomer, error) {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if deal == nil || deal.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("deal not found")
	}
	return s.dealRepo.GetCustomer(ctx, workspaceID, dealID)
}

// SetCustomer replaces the canonical customer while preserving unrelated participants.
func (s *CRMDealService) SetCustomer(ctx context.Context, dealID string, req model.SetCRMDealCustomerRequest, actorID string) (*model.CRMDealCustomer, error) {
	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if deal == nil || deal.WorkspaceID != req.WorkspaceID {
		return nil, fmt.Errorf("deal not found")
	}
	previous, err := s.dealRepo.GetCustomer(ctx, req.WorkspaceID, dealID)
	if err != nil {
		return nil, err
	}
	// Omitting contact_id while selecting a company means "keep the current
	// primary person", not "silently remove the person". Primary contact removal
	// remains an explicit association action.
	if strings.TrimSpace(req.CompanyID) != "" && strings.TrimSpace(req.ContactID) == "" && previous != nil {
		req.ContactID = previous.PrimaryContactID
	}
	customer, err := s.resolveCustomer(ctx, req.WorkspaceID, req.ContactID, req.CompanyID)
	if err != nil {
		return nil, err
	}
	if err := s.dealRepo.ReplaceCustomer(ctx, req.WorkspaceID, dealID, customer); err != nil {
		return nil, err
	}
	if s.activity != nil {
		metadata := map[string]interface{}{"customer_type": customer.CustomerType, "customer_id": customer.CustomerID}
		if previous != nil {
			metadata["previous_customer_type"] = previous.CustomerType
			metadata["previous_customer_id"] = previous.CustomerID
		}
		if err := s.activity.LogEvent(ctx, deal.WorkspaceID, "deal", deal.ID, optionalActor(actorID), "deal.customer_changed", "changed the customer for this deal", stringPtr("customer"), nil, &customer.CustomerID, metadata); err != nil {
			slog.ErrorContext(ctx, "log deal customer change", "error", err, "deal_id", deal.ID)
		}
	}
	s.requestCompanySummaryRefresh(ctx, deal.WorkspaceID, deal.ID)
	return &customer, nil
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

	if req.RevenueType != nil {
		if !validCRMRevenueType(*req.RevenueType) {
			return nil, fmt.Errorf("invalid revenue_type")
		}
		deal.RevenueType = *req.RevenueType
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
	if req.ClearCommercialMotion {
		deal.CommercialMotion = nil
	} else if req.CommercialMotion != nil {
		motion := strings.TrimSpace(*req.CommercialMotion)
		if !validCRMDealCommercialMotion(motion) {
			return nil, fmt.Errorf("invalid commercial_motion")
		}
		deal.CommercialMotion = &motion
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
	s.afterDealUpdate(ctx, updated, previousStageID, previousStageName, actorID)
	return updated, nil
}

// UpdateStageForPlaybook preserves reviewed facts and an atomic canonical result.
func (s *CRMDealService) UpdateStageForPlaybook(ctx context.Context, expected model.CRMDeal, stageID, actorID string, intent model.CRMPlaybookActionIntent) (*model.CRMDeal, error) {
	if err := s.dealRepo.UpdateStageForPlaybook(ctx, expected, stageID, intent); err != nil {
		return nil, err
	}
	updated, err := s.dealRepo.GetByID(ctx, expected.ID)
	if err != nil {
		return nil, err
	}
	previousName := expected.StageID
	if expected.Stage != nil {
		previousName = expected.Stage.Name
	}
	s.afterDealUpdate(ctx, updated, expected.StageID, previousName, actorID)
	return updated, nil
}

func (s *CRMDealService) afterDealUpdate(ctx context.Context, updated *model.CRMDeal, previousStageID, previousStageName, actorID string) {
	if s.motionSignals != nil {
		if err := s.motionSignals.ReconcileDealMotionSignals(ctx, updated.WorkspaceID, updated.ID); err != nil {
			slog.ErrorContext(ctx, "reconcile deal commercial-motion signals", "error", err, "deal_id", updated.ID)
		}
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
	s.requestCompanySummaryRefresh(ctx, updated.WorkspaceID, updated.ID)
}

func validCRMDealCommercialMotion(value string) bool {
	switch value {
	case model.CRMDealMotionNewBusiness, model.CRMDealMotionExistingBusiness, model.CRMDealMotionExpansion, model.CRMDealMotionRenewal:
		return true
	default:
		return false
	}
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
	s.requestCompanySummaryRefresh(ctx, deal.WorkspaceID, deal.ID)
	return s.dealRepo.Delete(ctx, id)
}

func (s *CRMDealService) requestCompanySummaryRefresh(ctx context.Context, workspaceID, dealID string) {
	if s == nil || s.summaryRefresh == nil {
		return
	}
	if err := s.summaryRefresh.RequestCompanyRefreshForObject(ctx, workspaceID, model.CRMObjectDeal, dealID); err != nil {
		slog.ErrorContext(ctx, "failed to request company summary refresh from deal", "error", err, "workspace_id", workspaceID, "deal_id", dealID)
	}
}

func validCRMRevenueType(value string) bool {
	return value == "one_time" || value == "monthly" || value == "annual"
}
