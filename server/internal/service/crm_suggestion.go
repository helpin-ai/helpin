package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMSuggestionService contains CRM suggestion business logic.
type CRMSuggestionService struct {
	suggestionRepo *repository.CRMSuggestionRepository
}

// NewCRMSuggestionService creates a new CRMSuggestionService.
func NewCRMSuggestionService(suggestionRepo *repository.CRMSuggestionRepository) *CRMSuggestionService {
	return &CRMSuggestionService{suggestionRepo: suggestionRepo}
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
