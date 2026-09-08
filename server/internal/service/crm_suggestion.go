package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMSuggestionService contains CRM suggestion business logic.
type CRMSuggestionService struct {
	suggestionRepo  *repository.CRMSuggestionRepository
	dealRepo        *repository.CRMDealRepository
	assocRepo       *repository.CRMAssociationRepository
	dealService     *CRMDealService
	playbookActions interface {
		AcceptSuggestionRevision(context.Context, string, string, string, map[string]interface{}) (*model.CRMSuggestion, error)
	}
	situationSources interface {
		ImportSuggestion(context.Context, model.CRMSuggestion) error
	}
	situationGuard interface {
		SituationActionBlocked(context.Context, string, string) (bool, error)
	}
}

// SetPlaybookActions routes typed proposals through exact canonical action authorization.
func (s *CRMSuggestionService) SetPlaybookActions(actions *CRMPlaybookActionService) {
	s.playbookActions = actions
}

// SetSituationGuard applies customer-work pause/closure checks to legacy approval
// entry points too, so navigation cannot bypass a paused customer process.
func (s *CRMSuggestionService) SetSituationGuard(guard interface {
	SituationActionBlocked(context.Context, string, string) (bool, error)
}) {
	s.situationGuard = guard
}

// SetSituationSources enables immediate projection; the reconciliation sweep
// repairs transient projection failures without retrying the source creation.
func (s *CRMSuggestionService) SetSituationSources(sources interface {
	ImportSuggestion(context.Context, model.CRMSuggestion) error
}) {
	s.situationSources = sources
}

// ErrCRMSuggestionStale requires the reviewer to reload the canonical proposal.
var ErrCRMSuggestionStale = errors.New("suggestion changed or is no longer pending")

// SetDealService routes accepted create-deal suggestions through the customer invariant.
func (s *CRMSuggestionService) SetDealService(dealService *CRMDealService) *CRMSuggestionService {
	s.dealService = dealService
	return s
}

// NewCRMSuggestionService creates a new CRMSuggestionService.
func NewCRMSuggestionService(suggestionRepo *repository.CRMSuggestionRepository, dealRepo *repository.CRMDealRepository, assocRepo *repository.CRMAssociationRepository) *CRMSuggestionService {
	return &CRMSuggestionService{suggestionRepo: suggestionRepo, dealRepo: dealRepo, assocRepo: assocRepo}
}

// List returns suggestions with filters and pagination.
func (s *CRMSuggestionService) List(ctx context.Context, workspaceID string, filters model.CRMSuggestionListFilters, pagination model.PMPagination) ([]model.CRMSuggestion, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.suggestionRepo.List(ctx, workspaceID, filters, pagination)
}

// GetByID returns a suggestion by ID.
func (s *CRMSuggestionService) GetByID(ctx context.Context, workspaceID, id string) (*model.CRMSuggestion, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("workspace_id and suggestion_id are required")
	}
	suggestion, err := s.suggestionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}
	return suggestion, nil
}

// Create creates a new suggestion.
func (s *CRMSuggestionService) Create(ctx context.Context, req model.CreateCRMSuggestionRequest) (*model.CRMSuggestion, error) {
	if req.WorkspaceID == "" || strings.TrimSpace(req.Title) == "" || req.SuggestionType == "" {
		return nil, fmt.Errorf("workspace_id, title, and suggestion_type are required")
	}
	if !validCRMSuggestionType(req.SuggestionType) {
		return nil, fmt.Errorf("invalid suggestion_type")
	}
	if (req.ObjectType == nil) != (req.ObjectID == nil) {
		return nil, fmt.Errorf("object_type and object_id must be provided together")
	}
	if req.ObjectType != nil && req.ObjectID != nil {
		exists, err := s.suggestionRepo.CRMObjectExists(ctx, req.WorkspaceID, strings.TrimSpace(*req.ObjectType), strings.TrimSpace(*req.ObjectID))
		if err != nil || !exists {
			return nil, fmt.Errorf("suggestion object not found in workspace")
		}
	}

	confidence := 0.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return nil, fmt.Errorf("confidence must be between 0 and 1")
	}
	signalIDs := make([]string, 0, len(req.SignalIDs))
	seenSignalIDs := map[string]bool{}
	for _, id := range req.SignalIDs {
		id = strings.TrimSpace(id)
		if id != "" && !seenSignalIDs[id] {
			seenSignalIDs[id] = true
			signalIDs = append(signalIDs, id)
		}
	}
	if err := s.suggestionRepo.ValidateSignalIDs(ctx, req.WorkspaceID, signalIDs); err != nil {
		return nil, err
	}

	suggestion := &model.CRMSuggestion{
		ID:             uuid.NewString(),
		WorkspaceID:    req.WorkspaceID,
		UserID:         req.UserID,
		SuggestionType: req.SuggestionType,
		ObjectType:     req.ObjectType,
		ObjectID:       req.ObjectID,
		Title:          strings.TrimSpace(req.Title),
		Description:    req.Description,
		Context:        model.JSONB(req.Context),
		SignalIDs:      model.StringArray(signalIDs),
		Status:         model.CRMSuggestionStatusPending,
		Confidence:     confidence,
	}

	if err := s.suggestionRepo.Create(ctx, suggestion); err != nil {
		return nil, err
	}
	if s.situationSources != nil {
		if err := s.situationSources.ImportSuggestion(ctx, *suggestion); err != nil {
			slog.WarnContext(ctx, "customer work projection will retry", "error", err, "workspace_id", suggestion.WorkspaceID, "suggestion_id", suggestion.ID)
		}
	}
	return suggestion, nil
}

func validCRMSuggestionType(suggestionType string) bool {
	switch suggestionType {
	case model.CRMSuggestionFollowUp, model.CRMSuggestionDealCreate, model.CRMSuggestionDealAdvance,
		model.CRMSuggestionEnrichment, model.CRMSuggestionRiskAlert:
		return true
	default:
		return false
	}
}

// Update updates a suggestion (typically to accept/dismiss).
func (s *CRMSuggestionService) Update(ctx context.Context, workspaceID, id string, req model.UpdateCRMSuggestionRequest) (*model.CRMSuggestion, error) {
	suggestion, err := s.suggestionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}
	if suggestion.SuggestionType == "playbook_action" && req.Status != nil && *req.Status == model.CRMSuggestionStatusAccepted {
		return nil, ErrCRMSuggestionStale
	}

	if req.Status != nil {
		status := *req.Status
		if status != model.CRMSuggestionStatusAccepted && status != model.CRMSuggestionStatusDismissed && status != model.CRMSuggestionStatusPending {
			return nil, fmt.Errorf("status must be pending, accepted, or dismissed")
		}
		if suggestion.Status != model.CRMSuggestionStatusPending {
			return nil, ErrCRMSuggestionStale
		}
		if status == model.CRMSuggestionStatusPending {
			return suggestion, nil
		}
		claimed, err := s.suggestionRepo.DecidePending(ctx, workspaceID, suggestion, status, nil)
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, ErrCRMSuggestionStale
		}
	}
	return s.suggestionRepo.GetByID(ctx, workspaceID, id)
}

// Delete removes a suggestion.
func (s *CRMSuggestionService) Delete(ctx context.Context, workspaceID, id string) error {
	suggestion, err := s.suggestionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if suggestion == nil {
		return fmt.Errorf("suggestion not found")
	}
	if suggestion.SuggestionType == "playbook_action" {
		return ErrCRMSuggestionStale // Preserve approval and provider-correlation history.
	}
	return s.suggestionRepo.Delete(ctx, workspaceID, id)
}

// AcceptSuggestion accepts a suggestion and optionally applies user edits, then executes the action.
func (s *CRMSuggestionService) AcceptSuggestion(ctx context.Context, workspaceID, id string, edits map[string]interface{}) (*model.CRMSuggestion, error) {
	return s.AcceptSuggestionRevision(ctx, workspaceID, id, "", edits)
}

// AcceptSuggestionRevision consumes the existing pending suggestion, optionally
// binding the caller's decision to the displayed revision.
func (s *CRMSuggestionService) AcceptSuggestionRevision(ctx context.Context, workspaceID, id, revision string, edits map[string]interface{}) (*model.CRMSuggestion, error) {
	suggestion, err := s.suggestionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}
	if suggestion.SuggestionType == "playbook_action" {
		if s.playbookActions == nil {
			return nil, ErrCRMPlaybookExecutionNotReady
		}
		return s.playbookActions.AcceptSuggestionRevision(ctx, workspaceID, id, revision, edits)
	}
	if suggestion.Status != model.CRMSuggestionStatusPending || suggestion.ExecutedAt != nil ||
		(suggestion.ExecutionStatus != "" && suggestion.ExecutionStatus != model.CRMSuggestionExecutionPending) {
		return nil, ErrCRMSuggestionStale
	}
	if s.situationGuard != nil {
		blocked, err := s.situationGuard.SituationActionBlocked(ctx, workspaceID, id)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, ErrCRMSuggestionStale
		}
	}
	if revision != "" && revision != model.CRMSuggestionRevision(*suggestion) {
		return nil, ErrCRMSuggestionStale
	}
	if len(suggestion.Signals) != len(suggestion.SignalIDs) {
		return nil, ErrCRMSuggestionStale
	}
	for _, signal := range suggestion.Signals {
		if signal.DismissedAt != nil || signal.SupersededAt != nil {
			return nil, ErrCRMSuggestionStale
		}
	}

	// Apply edits to context if provided
	if edits != nil {
		context := make(map[string]interface{}, len(suggestion.Context)+len(edits))
		for k, v := range suggestion.Context {
			context[k] = v
		}
		for k, v := range edits {
			context[k] = v
		}
		suggestion.Context = model.JSONB(context)
	}

	suggestion.Status = model.CRMSuggestionStatusAccepted
	suggestion.ExecutionStatus = model.CRMSuggestionExecutionInProgress
	manual := suggestion.SuggestionType != model.CRMSuggestionDealCreate && suggestion.SuggestionType != model.CRMSuggestionDealAdvance
	if manual {
		suggestion.ExecutionStatus = model.CRMSuggestionExecutionManualRequired
	}
	claimed, err := s.suggestionRepo.ClaimPending(ctx, workspaceID, suggestion)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, ErrCRMSuggestionStale
	}
	if manual {
		return s.suggestionRepo.GetByID(ctx, workspaceID, id)
	}

	// Execute the action based on suggestion type
	objectID, executionErr := s.executeSuggestionAction(ctx, suggestion)
	now := time.Now().UTC()
	suggestion.ExecutedAt = &now
	if executionErr != nil {
		suggestion.ExecutionStatus = model.CRMSuggestionExecutionFailed
		message := "The action did not complete. Review its targets and configuration before trying again."
		suggestion.ExecutionError = &message
	} else {
		suggestion.ExecutionStatus = model.CRMSuggestionExecutionSucceeded
		suggestion.ExecutionError = nil
		if objectID != nil {
			objectType := "deal"
			suggestion.ObjectType = &objectType
			suggestion.ObjectID = objectID
		}
	}
	if err := s.suggestionRepo.Update(ctx, workspaceID, suggestion); err != nil {
		return nil, err
	}
	if executionErr != nil {
		slog.Error("failed to execute suggestion action", "error", executionErr, "suggestion_id", id, "type", suggestion.SuggestionType)
		// Don't revert - the suggestion is accepted, action execution is best-effort
	}

	return s.suggestionRepo.GetByID(ctx, workspaceID, id)
}

// DismissSuggestion marks a suggestion as dismissed.
func (s *CRMSuggestionService) DismissSuggestion(ctx context.Context, workspaceID, id, reason string) (*model.CRMSuggestion, error) {
	return s.DismissSuggestionRevision(ctx, workspaceID, id, "", reason)
}

// DismissSuggestionRevision rejects exactly the displayed proposal when supplied.
func (s *CRMSuggestionService) DismissSuggestionRevision(ctx context.Context, workspaceID, id, revision, reason string) (*model.CRMSuggestion, error) {
	reason = strings.TrimSpace(reason)
	if !validSignalDismissalReason(reason) {
		return nil, fmt.Errorf("a valid dismissal reason is required")
	}
	suggestion, err := s.suggestionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}
	if revision != "" && revision != model.CRMSuggestionRevision(*suggestion) {
		return nil, ErrCRMSuggestionStale
	}
	claimed, err := s.suggestionRepo.DecidePending(ctx, workspaceID, suggestion, model.CRMSuggestionStatusDismissed, &reason)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, ErrCRMSuggestionStale
	}
	return s.suggestionRepo.GetByID(ctx, workspaceID, id)
}

func (s *CRMSuggestionService) executeSuggestionAction(ctx context.Context, suggestion *model.CRMSuggestion) (*string, error) {
	suggestionContext := map[string]interface{}(suggestion.Context)

	switch suggestion.SuggestionType {
	case model.CRMSuggestionDealCreate:
		return s.executeDealCreate(ctx, suggestion.WorkspaceID, suggestionContext)
	case model.CRMSuggestionDealAdvance:
		if err := s.executeDealAdvance(ctx, suggestion.WorkspaceID, suggestionContext); err != nil {
			return nil, err
		}
		dealID, _ := suggestionContext["deal_id"].(string)
		return &dealID, nil
	default:
		return nil, fmt.Errorf("this recommendation requires manual follow-through")
	}
}

func (s *CRMSuggestionService) executeDealCreate(ctx context.Context, workspaceID string, suggestionContext map[string]interface{}) (*string, error) {
	dealName, _ := suggestionContext["deal_name"].(string)
	pipelineID, _ := suggestionContext["pipeline_id"].(string)
	stageID, _ := suggestionContext["stage_id"].(string)
	contactID, _ := suggestionContext["contact_id"].(string)
	companyID, _ := suggestionContext["company_id"].(string)

	if dealName == "" || pipelineID == "" || stageID == "" {
		return nil, fmt.Errorf("missing required deal context fields")
	}
	pipeline, err := s.dealRepo.GetPipeline(ctx, pipelineID)
	if err != nil || pipeline == nil || pipeline.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("pipeline not found in workspace")
	}
	stage, err := s.dealRepo.GetStage(ctx, stageID)
	if err != nil || stage == nil || stage.PipelineID != pipelineID {
		return nil, fmt.Errorf("stage not found in pipeline")
	}
	if contactID != "" {
		exists, contactErr := s.suggestionRepo.CRMObjectExists(ctx, workspaceID, model.CRMObjectContact, contactID)
		if contactErr != nil || !exists {
			return nil, fmt.Errorf("contact not found in workspace")
		}
	}
	if contactID == "" && companyID == "" {
		return nil, fmt.Errorf("missing deal customer context")
	}
	if s.dealService == nil {
		return nil, fmt.Errorf("deal creation is not configured")
	}
	req := model.CreateCRMDealRequest{WorkspaceID: workspaceID, Name: dealName, PipelineID: pipelineID, StageID: stageID, ContactID: contactID, CompanyID: companyID}
	if amount, ok := suggestionContext["amount"].(float64); ok {
		req.Amount = &amount
	}
	deal, err := s.dealService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	slog.Info("executed deal_create suggestion", "deal_id", deal.ID, "deal_name", dealName)
	return &deal.ID, nil
}

func (s *CRMSuggestionService) executeDealAdvance(ctx context.Context, workspaceID string, suggestionContext map[string]interface{}) error {
	dealID, _ := suggestionContext["deal_id"].(string)
	targetStageID, _ := suggestionContext["target_stage_id"].(string)

	if dealID == "" || targetStageID == "" {
		return fmt.Errorf("missing deal_id or target_stage_id")
	}

	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil || deal == nil || deal.WorkspaceID != workspaceID {
		return fmt.Errorf("deal not found: %s", dealID)
	}
	targetStage, err := s.dealRepo.GetStage(ctx, targetStageID)
	if err != nil || targetStage == nil || targetStage.PipelineID != deal.PipelineID {
		return fmt.Errorf("target stage not found in deal pipeline")
	}

	deal.StageID = targetStageID
	deal.Pipeline = nil
	deal.Stage = nil
	if err := s.dealRepo.Update(ctx, deal); err != nil {
		return err
	}

	slog.Info("executed deal_advance suggestion", "deal_id", dealID, "new_stage_id", targetStageID)
	return nil
}
