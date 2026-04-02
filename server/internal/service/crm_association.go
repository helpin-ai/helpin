package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// CRMAssociationService contains CRM association business logic.
type CRMAssociationService struct {
	assocRepo *repository.CRMAssociationRepository
}

// NewCRMAssociationService creates a new CRMAssociationService.
func NewCRMAssociationService(assocRepo *repository.CRMAssociationRepository) *CRMAssociationService {
	return &CRMAssociationService{assocRepo: assocRepo}
}

// Create creates an association between two CRM objects.
func (s *CRMAssociationService) Create(ctx context.Context, req model.CreateCRMAssociationRequest) (*model.CRMAssociation, error) {
	if req.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if !isValidObjectType(req.FromObjectType) || !isValidObjectType(req.ToObjectType) {
		return nil, fmt.Errorf("invalid object type")
	}
	if req.FromObjectID == "" || req.ToObjectID == "" {
		return nil, fmt.Errorf("from_object_id and to_object_id are required")
	}

	assoc := &model.CRMAssociation{
		WorkspaceID:      req.WorkspaceID,
		FromObjectType:   req.FromObjectType,
		FromObjectID:     req.FromObjectID,
		ToObjectType:     req.ToObjectType,
		ToObjectID:       req.ToObjectID,
		AssociationLabel: req.AssociationLabel,
	}

	if err := s.assocRepo.Create(ctx, assoc); err != nil {
		return nil, err
	}
	return assoc, nil
}

// Delete removes an association.
func (s *CRMAssociationService) Delete(ctx context.Context, id string) error {
	return s.assocRepo.Delete(ctx, id)
}

// ListByObject returns all associations for a given object.
func (s *CRMAssociationService) ListByObject(ctx context.Context, workspaceID, objectType, objectID string) ([]model.CRMAssociation, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.assocRepo.ListByObject(ctx, workspaceID, objectType, objectID)
}

func isValidObjectType(t string) bool {
	switch t {
	case model.CRMObjectContact, model.CRMObjectCompany, model.CRMObjectDeal, model.CRMObjectEpic, model.CRMObjectTask, model.CRMObjectSupportConversation:
		return true
	default:
		return false
	}
}

// ListByObjectEnriched returns enriched associations for a given object.
func (s *CRMAssociationService) ListByObjectEnriched(ctx context.Context, workspaceID, objectType, objectID string) ([]model.CRMAssociationEnriched, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return s.assocRepo.ListByObjectEnriched(ctx, workspaceID, objectType, objectID)
}
