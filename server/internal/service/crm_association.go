package service

import (
	"context"
	"fmt"
	"strings"

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
	normalizePrimaryCompanyLabel(&req)

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
	if err := s.applyAssociationSemantics(ctx, assoc, req); err != nil {
		return nil, err
	}
	return assoc, nil
}

func normalizeContactCompanyAssociation(req model.CreateCRMAssociationRequest) (contactID string, companyID string, ok bool) {
	switch {
	case req.FromObjectType == model.CRMObjectContact && req.ToObjectType == model.CRMObjectCompany:
		return req.FromObjectID, req.ToObjectID, true
	case req.FromObjectType == model.CRMObjectCompany && req.ToObjectType == model.CRMObjectContact:
		return req.ToObjectID, req.FromObjectID, true
	default:
		return "", "", false
	}
}

func normalizePrimaryCompanyLabel(req *model.CreateCRMAssociationRequest) {
	if req == nil || req.AssociationLabel == nil {
		return
	}
	label := strings.TrimSpace(*req.AssociationLabel)
	switch label {
	case "":
		req.AssociationLabel = nil
	case primaryCompanyAssociationLabel:
		req.AssociationLabel = crmAssociationStringPtr(primaryCompanyAssociationLabel)
	default:
		req.AssociationLabel = &label
	}
}

const primaryCompanyAssociationLabel = "primary"

func isPrimaryCompanyAssociationLabel(label *string) bool {
	return label != nil && strings.TrimSpace(*label) == primaryCompanyAssociationLabel
}

func (s *CRMAssociationService) applyAssociationSemantics(ctx context.Context, assoc *model.CRMAssociation, req model.CreateCRMAssociationRequest) error {
	contactID, _, isContactCompany := normalizeContactCompanyAssociation(req)
	if !isContactCompany {
		return nil
	}

	shouldBePrimary, err := s.shouldPromoteCompanyAssociationToPrimary(ctx, req.WorkspaceID, contactID, assoc.ID, req.AssociationLabel)
	if err != nil {
		return err
	}
	if shouldBePrimary {
		return s.setPrimaryCompanyAssociation(ctx, req.WorkspaceID, contactID, assoc.ID)
	}
	return nil
}

func (s *CRMAssociationService) shouldPromoteCompanyAssociationToPrimary(ctx context.Context, workspaceID, contactID, associationID string, label *string) (bool, error) {
	if isPrimaryCompanyAssociationLabel(label) {
		return true, nil
	}

	existing, err := s.assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contactID)
	if err != nil {
		return false, fmt.Errorf("list contact associations: %w", err)
	}

	hasOtherPrimary := false
	hasCurrentPrimary := false
	for _, assoc := range existing {
		otherType, _ := otherAssociationSide(assoc, model.CRMObjectContact, contactID)
		if otherType != model.CRMObjectCompany {
			continue
		}
		if assoc.ID == associationID && isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
			hasCurrentPrimary = true
		}
		if assoc.ID != associationID && isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
			hasOtherPrimary = true
		}
	}
	if hasCurrentPrimary {
		return true, nil
	}
	return !hasOtherPrimary, nil
}

func (s *CRMAssociationService) setPrimaryCompanyAssociation(ctx context.Context, workspaceID, contactID, associationID string) error {
	existing, err := s.assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contactID)
	if err != nil {
		return fmt.Errorf("list contact associations: %w", err)
	}

	for _, assoc := range existing {
		otherType, _ := otherAssociationSide(assoc, model.CRMObjectContact, contactID)
		if otherType != model.CRMObjectCompany || assoc.ID == associationID {
			continue
		}
		if isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
			if err := s.assocRepo.UpdateLabel(ctx, assoc.ID, nil); err != nil {
				return err
			}
		}
	}

	if err := s.assocRepo.UpdateLabel(ctx, associationID, crmAssociationStringPtr(primaryCompanyAssociationLabel)); err != nil {
		return err
	}
	return nil
}

func crmAssociationStringPtr(value string) *string {
	return &value
}

// Delete removes an association.
func (s *CRMAssociationService) Delete(ctx context.Context, id string) error {
	return s.assocRepo.Delete(ctx, id)
}

// GetScoped returns an association only when it belongs to the supplied workspace.
func (s *CRMAssociationService) GetScoped(ctx context.Context, workspaceID, id string) (*model.CRMAssociation, error) {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("association not found")
	}
	assoc, err := s.assocRepo.GetByID(ctx, strings.TrimSpace(id))
	if err != nil || assoc == nil || assoc.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("association not found")
	}
	return assoc, nil
}

// DeleteScoped removes an association without permitting cross-workspace IDs.
func (s *CRMAssociationService) DeleteScoped(ctx context.Context, workspaceID, id string) error {
	assoc, err := s.GetScoped(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	return s.assocRepo.Delete(ctx, assoc.ID)
}

// SetPrimaryContactCompany creates or reuses a contact-company association and
// makes it the only primary company association for that contact.
func (s *CRMAssociationService) SetPrimaryContactCompany(ctx context.Context, workspaceID, contactID, companyID string) (*model.CRMAssociation, error) {
	label := primaryCompanyAssociationLabel
	assoc, err := s.Create(ctx, model.CreateCRMAssociationRequest{
		WorkspaceID: workspaceID, FromObjectType: model.CRMObjectContact, FromObjectID: strings.TrimSpace(contactID),
		ToObjectType: model.CRMObjectCompany, ToObjectID: strings.TrimSpace(companyID), AssociationLabel: &label,
	})
	if err != nil {
		return nil, err
	}
	return s.GetScoped(ctx, workspaceID, assoc.ID)
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
	assocs, err := s.assocRepo.ListByObjectEnriched(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return nil, err
	}
	if objectID == "" {
		return assocs, nil
	}
	switch objectType {
	case model.CRMObjectCompany:
		return s.appendInferredCompanyContactAssociations(ctx, workspaceID, objectID, assocs)
	case model.CRMObjectDeal:
		return s.appendInferredDealContactCompanies(ctx, workspaceID, objectID, assocs)
	default:
		return assocs, nil
	}
}

func (s *CRMAssociationService) appendInferredDealContactCompanies(
	ctx context.Context,
	workspaceID, dealID string,
	assocs []model.CRMAssociationEnriched,
) ([]model.CRMAssociationEnriched, error) {
	if len(assocs) == 0 {
		return assocs, nil
	}

	seen := make(map[string]struct{}, len(assocs))
	enriched := append([]model.CRMAssociationEnriched(nil), assocs...)
	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc.CRMAssociation, model.CRMObjectDeal, dealID)
		seen[otherType+":"+otherID] = struct{}{}
	}

	for _, assoc := range assocs {
		otherType, contactID := otherAssociationSide(assoc.CRMAssociation, model.CRMObjectDeal, dealID)
		if otherType != model.CRMObjectContact {
			continue
		}

		contactAssocs, err := s.assocRepo.ListByObjectEnriched(ctx, workspaceID, model.CRMObjectContact, contactID)
		if err != nil {
			return nil, err
		}
		contextName := strings.TrimSpace(assoc.LinkedObjectName)
		if contextName == "" {
			contextName = "contact"
		}
		contextLabel := "via " + contextName

		for _, contactAssoc := range contactAssocs {
			inferredType, companyID := otherAssociationSide(contactAssoc.CRMAssociation, model.CRMObjectContact, contactID)
			if inferredType != model.CRMObjectCompany {
				continue
			}
			key := inferredType + ":" + companyID
			if _, exists := seen[key]; exists {
				continue
			}

			enriched = append(enriched, model.CRMAssociationEnriched{
				CRMAssociation: model.CRMAssociation{
					WorkspaceID:    workspaceID,
					FromObjectType: model.CRMObjectDeal,
					FromObjectID:   dealID,
					ToObjectType:   model.CRMObjectCompany,
					ToObjectID:     companyID,
				},
				LinkedObjectName:        contactAssoc.LinkedObjectName,
				LinkedObjectDisplayID:   contactAssoc.LinkedObjectDisplayID,
				LinkedObjectStatus:      contactAssoc.LinkedObjectStatus,
				LinkedObjectStatusColor: contactAssoc.LinkedObjectStatusColor,
				Inferred:                true,
				ContextLabel:            &contextLabel,
			})
			seen[key] = struct{}{}
		}
	}

	return enriched, nil
}

func (s *CRMAssociationService) appendInferredCompanyContactAssociations(
	ctx context.Context,
	workspaceID, companyID string,
	assocs []model.CRMAssociationEnriched,
) ([]model.CRMAssociationEnriched, error) {
	if len(assocs) == 0 {
		return assocs, nil
	}

	seen := make(map[string]struct{}, len(assocs))
	enriched := make([]model.CRMAssociationEnriched, 0, len(assocs))
	enriched = append(enriched, assocs...)

	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc.CRMAssociation, model.CRMObjectCompany, companyID)
		seen[otherType+":"+otherID] = struct{}{}
		if otherType != model.CRMObjectContact {
			continue
		}

		contactAssocs, err := s.assocRepo.ListByObjectEnriched(ctx, workspaceID, model.CRMObjectContact, otherID)
		if err != nil {
			return nil, err
		}

		contextName := strings.TrimSpace(assoc.LinkedObjectName)
		if contextName == "" {
			contextName = "contact"
		}
		contextLabel := "via " + contextName

		for _, contactAssoc := range contactAssocs {
			inferredType, inferredID := otherAssociationSide(contactAssoc.CRMAssociation, model.CRMObjectContact, otherID)
			if inferredType == model.CRMObjectContact || (inferredType == model.CRMObjectCompany && inferredID == companyID) {
				continue
			}

			key := inferredType + ":" + inferredID
			if _, exists := seen[key]; exists {
				continue
			}

			enriched = append(enriched, model.CRMAssociationEnriched{
				CRMAssociation: model.CRMAssociation{
					WorkspaceID:    workspaceID,
					FromObjectType: model.CRMObjectCompany,
					FromObjectID:   companyID,
					ToObjectType:   inferredType,
					ToObjectID:     inferredID,
				},
				LinkedObjectName:        contactAssoc.LinkedObjectName,
				LinkedObjectDisplayID:   contactAssoc.LinkedObjectDisplayID,
				LinkedObjectStatus:      contactAssoc.LinkedObjectStatus,
				LinkedObjectStatusColor: contactAssoc.LinkedObjectStatusColor,
				Inferred:                true,
				ContextLabel:            &contextLabel,
			})
			seen[key] = struct{}{}
		}
	}

	return enriched, nil
}
