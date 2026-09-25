package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerCRMAssociationCommands() {
	definitions := []InternalCommandDefinition{
		{Name: "crm.list_associations", Module: "crm", SupportedTargetTypes: []string{"workspace", "crm_contact", "crm_company", "crm_deal", "epic", "task", "support_conversation"}, Tool: mustCommandToolMetadata("crm.list_associations"), Execute: s.executeCRMListAssociations},
		{Name: "crm.link_objects", Module: "crm", Mutating: true, SupportedTargetTypes: []string{"workspace", "crm_contact", "crm_company", "crm_deal", "epic", "task", "support_conversation"}, Tool: mustCommandToolMetadata("crm.link_objects"), Execute: s.executeCRMLinkObjects},
		{Name: "crm.unlink_association", Module: "crm", Mutating: true, SupportedTargetTypes: []string{"workspace", "crm_contact", "crm_company", "crm_deal", "epic", "task", "support_conversation"}, Tool: mustCommandToolMetadata("crm.unlink_association"), Execute: s.executeCRMUnlinkAssociation},
		{Name: "crm.set_primary_contact_company", Module: "crm", Mutating: true, SupportedTargetTypes: []string{"workspace", "crm_contact", "crm_company"}, Tool: mustCommandToolMetadata("crm.set_primary_contact_company"), Execute: s.executeCRMSetPrimaryContactCompany},
	}
	for _, def := range definitions {
		s.register(def)
	}
}

func (s *InternalCommandService) executeCRMListAssociations(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmAssociationService == nil {
		return nil, fmt.Errorf("CRM association service is not configured")
	}
	var req struct {
		ObjectType string `json:"object_type"`
		ObjectID   string `json:"object_id"`
		Limit      int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse association list input: %w", err)
	}
	if err := s.validateCRMAssociationObject(ctx, meta, req.ObjectType, req.ObjectID); err != nil {
		return nil, err
	}
	limit, err := normalizeOperationalLimit(req.Limit)
	if err != nil {
		return nil, err
	}
	items, err := s.crmAssociationService.ListByObject(ctx, meta.WorkspaceID, strings.TrimSpace(req.ObjectType), strings.TrimSpace(req.ObjectID))
	if err != nil {
		return nil, err
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return mustJSON(map[string]any{"associations": items}), nil
}

func (s *InternalCommandService) executeCRMLinkObjects(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmAssociationService == nil {
		return nil, fmt.Errorf("CRM association service is not configured")
	}
	var req struct {
		FromObjectType   string  `json:"from_object_type"`
		FromObjectID     string  `json:"from_object_id"`
		ToObjectType     string  `json:"to_object_type"`
		ToObjectID       string  `json:"to_object_id"`
		AssociationLabel *string `json:"association_label"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse association input: %w", err)
	}
	if req.FromObjectType == req.ToObjectType && strings.TrimSpace(req.FromObjectID) == strings.TrimSpace(req.ToObjectID) {
		return nil, fmt.Errorf("an object cannot be associated with itself")
	}
	if req.AssociationLabel != nil && strings.EqualFold(strings.TrimSpace(*req.AssociationLabel), primaryCompanyAssociationLabel) {
		return nil, fmt.Errorf("use set_primary_contact_company for primary company associations")
	}
	if err := s.validateCRMAssociationObject(ctx, meta, req.FromObjectType, req.FromObjectID); err != nil {
		return nil, err
	}
	if err := s.validateCRMAssociationObject(ctx, meta, req.ToObjectType, req.ToObjectID); err != nil {
		return nil, err
	}
	assoc, err := s.crmAssociationService.Create(ctx, model.CreateCRMAssociationRequest{WorkspaceID: meta.WorkspaceID, FromObjectType: strings.TrimSpace(req.FromObjectType), FromObjectID: strings.TrimSpace(req.FromObjectID), ToObjectType: strings.TrimSpace(req.ToObjectType), ToObjectID: strings.TrimSpace(req.ToObjectID), AssociationLabel: req.AssociationLabel})
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMAssociation(assoc)), nil
}

func (s *InternalCommandService) executeCRMUnlinkAssociation(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmAssociationService == nil {
		return nil, fmt.Errorf("CRM association service is not configured")
	}
	var req struct {
		AssociationID string `json:"association_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse unlink association input: %w", err)
	}
	assoc, err := s.crmAssociationService.GetScoped(ctx, meta.WorkspaceID, strings.TrimSpace(req.AssociationID))
	if err != nil {
		return nil, err
	}
	if err := s.validateCRMAssociationObject(ctx, meta, assoc.FromObjectType, assoc.FromObjectID); err != nil {
		return nil, err
	}
	if err := s.validateCRMAssociationObject(ctx, meta, assoc.ToObjectType, assoc.ToObjectID); err != nil {
		return nil, err
	}
	if err := s.crmAssociationService.DeleteScoped(ctx, meta.WorkspaceID, assoc.ID); err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"association_id": assoc.ID, "unlinked": true}), nil
}

func (s *InternalCommandService) executeCRMSetPrimaryContactCompany(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	if s.crmAssociationService == nil {
		return nil, fmt.Errorf("CRM association service is not configured")
	}
	var req struct {
		ContactID string `json:"contact_id"`
		CompanyID string `json:"company_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse primary company input: %w", err)
	}
	if _, err := s.scopedCRMContact(ctx, meta.WorkspaceID, req.ContactID); err != nil {
		return nil, err
	}
	if _, err := s.scopedCRMCompany(ctx, meta.WorkspaceID, req.CompanyID); err != nil {
		return nil, err
	}
	assoc, err := s.crmAssociationService.SetPrimaryContactCompany(ctx, meta.WorkspaceID, req.ContactID, req.CompanyID)
	if err != nil {
		return nil, err
	}
	return mustJSON(compactCRMAssociation(assoc)), nil
}

func (s *InternalCommandService) validateCRMAssociationObject(ctx context.Context, meta model.InternalCommandContext, objectType, objectID string) error {
	objectType, objectID = strings.TrimSpace(objectType), strings.TrimSpace(objectID)
	if objectID == "" {
		return errCommandInput("object_id is required")
	}
	switch objectType {
	case model.CRMObjectContact:
		_, err := s.scopedCRMContact(ctx, meta.WorkspaceID, objectID)
		return err
	case model.CRMObjectCompany:
		_, err := s.scopedCRMCompany(ctx, meta.WorkspaceID, objectID)
		return err
	case model.CRMObjectDeal:
		_, err := s.scopedCRMDeal(ctx, meta.WorkspaceID, objectID)
		return err
	case model.CRMObjectEpic:
		if s.epicService == nil {
			return fmt.Errorf("epic service is not configured")
		}
		epic, err := s.epicService.GetByID(ctx, objectID)
		if err != nil || epic == nil || epic.Epic.WorkspaceID != meta.WorkspaceID {
			return errCommandNotFound("epic")
		}
		return requireCommandAgentTeam(meta, epic.Epic.TeamID)
	case model.CRMObjectTask:
		if s.taskService == nil {
			return fmt.Errorf("task service is not configured")
		}
		task, err := s.taskService.GetByID(ctx, objectID)
		if err != nil || task == nil || task.Task.WorkspaceID != meta.WorkspaceID {
			return errCommandNotFound("task")
		}
		return requireCommandAgentTeam(meta, task.Task.TeamID)
	case model.CRMObjectSupportConversation:
		if s.supportConversationRepo == nil {
			return fmt.Errorf("support conversation service is not configured")
		}
		conversation, err := s.supportConversationRepo.GetByID(ctx, meta.WorkspaceID, objectID, "", model.RoleOwner)
		if err != nil || conversation == nil {
			return errCommandNotFound("support conversation")
		}
		return nil
	default:
		return fmt.Errorf("invalid object type")
	}
}

func compactCRMAssociation(assoc *model.CRMAssociation) map[string]any {
	return map[string]any{"association_id": assoc.ID, "from_object_type": assoc.FromObjectType, "from_object_id": assoc.FromObjectID, "to_object_type": assoc.ToObjectType, "to_object_id": assoc.ToObjectID, "association_label": assoc.AssociationLabel}
}
