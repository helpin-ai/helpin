package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMEnrichmentService contains CRM enrichment business logic.
type CRMEnrichmentService struct {
	enrichmentRepo *repository.CRMEnrichmentRepository
}

// NewCRMEnrichmentService creates a new CRMEnrichmentService.
func NewCRMEnrichmentService(enrichmentRepo *repository.CRMEnrichmentRepository) *CRMEnrichmentService {
	return &CRMEnrichmentService{enrichmentRepo: enrichmentRepo}
}

// List returns enrichment results with filters and pagination.
func (s *CRMEnrichmentService) List(ctx context.Context, workspaceID string, filters model.CRMEnrichmentListFilters, pagination model.PMPagination) ([]model.CRMEnrichmentResult, int64, error) {
	if workspaceID == "" {
		return nil, 0, fmt.Errorf("workspace_id is required")
	}
	return s.enrichmentRepo.List(ctx, workspaceID, filters, pagination)
}

// Create creates a new enrichment result.
func (s *CRMEnrichmentService) Create(ctx context.Context, req model.CreateCRMEnrichmentRequest) (*model.CRMEnrichmentResult, error) {
	if req.WorkspaceID == "" || req.ObjectType == "" || req.ObjectID == "" {
		return nil, fmt.Errorf("workspace_id, object_type, and object_id are required")
	}

	confidence := 0.0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}

	enrichment := &model.CRMEnrichmentResult{
		WorkspaceID: req.WorkspaceID,
		ObjectType:  req.ObjectType,
		ObjectID:    req.ObjectID,
		Source:      req.Source,
		Data:        model.JSONB(req.Data),
		Confidence:  confidence,
	}

	if enrichment.Source == "" {
		enrichment.Source = model.CRMEnrichmentSourceManual
	}

	if err := s.enrichmentRepo.Create(ctx, enrichment); err != nil {
		return nil, err
	}
	return enrichment, nil
}

// Delete removes an enrichment result.
func (s *CRMEnrichmentService) Delete(ctx context.Context, id string) error {
	return s.enrichmentRepo.Delete(ctx, id)
}
