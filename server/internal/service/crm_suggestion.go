package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMSuggestionService contains CRM suggestion business logic.
type CRMSuggestionService struct {
	suggestionRepo *repository.CRMSuggestionRepository
	dealRepo       *repository.CRMDealRepository
	assocRepo      *repository.CRMAssociationRepository
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
func (s *CRMSuggestionService) GetByID(ctx context.Context, id string) (*model.CRMSuggestion, error) {
	suggestion, err := s.suggestionRepo.GetByID(ctx, id)
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

	confidence := 0.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}

	suggestion := &model.CRMSuggestion{
		WorkspaceID:    req.WorkspaceID,
		UserID:         req.UserID,
		SuggestionType: req.SuggestionType,
		ObjectType:     req.ObjectType,
		ObjectID:       req.ObjectID,
		Title:          strings.TrimSpace(req.Title),
		Description:    req.Description,
		Context:        model.JSONB(req.Context),
		Status:         model.CRMSuggestionStatusPending,
		Confidence:     confidence,
	}

	if err := s.suggestionRepo.Create(ctx, suggestion); err != nil {
		return nil, err
	}
	return suggestion, nil
}

// Update updates a suggestion (typically to accept/dismiss).
func (s *CRMSuggestionService) Update(ctx context.Context, id string, req model.UpdateCRMSuggestionRequest) (*model.CRMSuggestion, error) {
	suggestion, err := s.suggestionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}

	if req.Status != nil {
		status := *req.Status
		if status != model.CRMSuggestionStatusAccepted && status != model.CRMSuggestionStatusDismissed && status != model.CRMSuggestionStatusPending {
			return nil, fmt.Errorf("status must be pending, accepted, or dismissed")
		}
		suggestion.Status = status
	}

	if err := s.suggestionRepo.Update(ctx, suggestion); err != nil {
		return nil, err
	}
	return suggestion, nil
}

// Delete removes a suggestion.
func (s *CRMSuggestionService) Delete(ctx context.Context, id string) error {
	suggestion, err := s.suggestionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if suggestion == nil {
		return fmt.Errorf("suggestion not found")
	}
	return s.suggestionRepo.Delete(ctx, id)
}

// AcceptSuggestion accepts a suggestion and optionally applies user edits, then executes the action.
func (s *CRMSuggestionService) AcceptSuggestion(ctx context.Context, id string, edits map[string]interface{}) (*model.CRMSuggestion, error) {
	suggestion, err := s.suggestionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}
	if suggestion.Status != model.CRMSuggestionStatusPending {
		return nil, fmt.Errorf("suggestion is not pending")
	}

	// Apply edits to context if provided
	if edits != nil {
		context := map[string]interface{}(suggestion.Context)
		for k, v := range edits {
			context[k] = v
		}
		suggestion.Context = model.JSONB(context)
	}

	suggestion.Status = model.CRMSuggestionStatusAccepted
	if err := s.suggestionRepo.Update(ctx, suggestion); err != nil {
		return nil, err
	}

	// Execute the action based on suggestion type
	if err := s.executeSuggestionAction(ctx, suggestion); err != nil {
		slog.Error("failed to execute suggestion action", "error", err, "suggestion_id", id, "type", suggestion.SuggestionType)
		// Don't revert - the suggestion is accepted, action execution is best-effort
	}

	return suggestion, nil
}

// DismissSuggestion marks a suggestion as dismissed.
func (s *CRMSuggestionService) DismissSuggestion(ctx context.Context, id string) (*model.CRMSuggestion, error) {
	suggestion, err := s.suggestionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if suggestion == nil {
		return nil, fmt.Errorf("suggestion not found")
	}

	suggestion.Status = model.CRMSuggestionStatusDismissed
	if err := s.suggestionRepo.Update(ctx, suggestion); err != nil {
		return nil, err
	}
	return suggestion, nil
}

func (s *CRMSuggestionService) executeSuggestionAction(ctx context.Context, suggestion *model.CRMSuggestion) error {
	suggestionContext := map[string]interface{}(suggestion.Context)

	switch suggestion.SuggestionType {
	case model.CRMSuggestionDealCreate:
		return s.executeDealCreate(ctx, suggestion.WorkspaceID, suggestionContext)
	case model.CRMSuggestionDealAdvance:
		return s.executeDealAdvance(ctx, suggestionContext)
	default:
		return nil // Other types don't have automatic actions
	}
}

func (s *CRMSuggestionService) executeDealCreate(ctx context.Context, workspaceID string, suggestionContext map[string]interface{}) error {
	dealName, _ := suggestionContext["deal_name"].(string)
	pipelineID, _ := suggestionContext["pipeline_id"].(string)
	stageID, _ := suggestionContext["stage_id"].(string)
	contactID, _ := suggestionContext["contact_id"].(string)

	if dealName == "" || pipelineID == "" || stageID == "" {
		return fmt.Errorf("missing required deal context fields")
	}

	displayID, err := s.dealRepo.GetNextDisplayID(ctx, workspaceID)
	if err != nil {
		return err
	}

	deal := &model.CRMDeal{
		WorkspaceID: workspaceID,
		DisplayID:   displayID,
		Name:        dealName,
		PipelineID:  pipelineID,
		StageID:     stageID,
		Currency:    "USD",
	}

	if amount, ok := suggestionContext["amount"].(float64); ok {
		deal.Amount = &amount
	}

	if err := s.dealRepo.Create(ctx, deal); err != nil {
		return err
	}

	// Create contact association
	if contactID != "" {
		assoc := &model.CRMAssociation{
			WorkspaceID:    workspaceID,
			FromObjectType: "deal",
			FromObjectID:   deal.ID,
			ToObjectType:   "contact",
			ToObjectID:     contactID,
		}
		if err := s.assocRepo.Create(ctx, assoc); err != nil {
			slog.Error("failed to create deal-contact association", "error", err, "deal_id", deal.ID, "contact_id", contactID)
		}
	}

	slog.Info("executed deal_create suggestion", "deal_id", deal.ID, "deal_name", dealName)
	return nil
}

func (s *CRMSuggestionService) executeDealAdvance(ctx context.Context, suggestionContext map[string]interface{}) error {
	dealID, _ := suggestionContext["deal_id"].(string)
	targetStageID, _ := suggestionContext["target_stage_id"].(string)

	if dealID == "" || targetStageID == "" {
		return fmt.Errorf("missing deal_id or target_stage_id")
	}

	deal, err := s.dealRepo.GetByID(ctx, dealID)
	if err != nil || deal == nil {
		return fmt.Errorf("deal not found: %s", dealID)
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
