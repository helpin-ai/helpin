package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	docsEntityRefStatusAvailable   = "available"
	docsEntityRefStatusUnavailable = "unavailable"
	docsEntityRefAccessGranted     = "granted"
	docsEntityRefAccessUnavailable = "unavailable"
	docsEntityRefAccessRedacted    = "redacted"
)

type DocsEntityReferenceResolverService struct {
	taskSvc     *PMTaskService
	epicSvc     *PMEpicService
	supportSvc  *SupportInboxService
	dealSvc     *CRMDealService
	contactSvc  *CRMContactService
	companySvc  *CRMCompanyService
	documentSvc *DocsDocumentService
	authzSvc    *authorization.AuthzService
}

func NewDocsEntityReferenceResolverService(taskSvc *PMTaskService, epicSvc *PMEpicService, supportSvc *SupportInboxService, dealSvc *CRMDealService, contactSvc *CRMContactService, companySvc *CRMCompanyService, documentSvc *DocsDocumentService, authzSvc *authorization.AuthzService) *DocsEntityReferenceResolverService {
	return &DocsEntityReferenceResolverService{
		taskSvc:     taskSvc,
		epicSvc:     epicSvc,
		supportSvc:  supportSvc,
		dealSvc:     dealSvc,
		contactSvc:  contactSvc,
		companySvc:  companySvc,
		documentSvc: documentSvc,
		authzSvc:    authzSvc,
	}
}

func (s *DocsEntityReferenceResolverService) Resolve(ctx context.Context, workspaceID string, req model.ResolveDocsEntityRefsRequest) (*model.ResolveDocsEntityRefsResponse, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if len(req.Refs) > 100 {
		return nil, fmt.Errorf("at most 100 references can be resolved at once")
	}

	refs := make([]model.DocsResolvedEntityRef, 0, len(req.Refs))
	for _, ref := range req.Refs {
		refs = append(refs, s.resolveOne(ctx, workspaceID, ref))
	}
	return &model.ResolveDocsEntityRefsResponse{Refs: refs}, nil
}

func (s *DocsEntityReferenceResolverService) resolveOne(ctx context.Context, workspaceID string, ref model.DocsEntityRefRequest) model.DocsResolvedEntityRef {
	entityType := normalizeDocsEntityRefType(ref.EntityType)
	entityID := strings.TrimSpace(ref.EntityID)
	if entityType == "" || entityID == "" {
		return unavailableDocsEntityRef(ref, entityType)
	}

	switch entityType {
	case "task", "story":
		if !s.canReadModule(ctx, model.ModulePM, authorization.PermPMRead) {
			return redactedDocsEntityRef(ref, entityType)
		}
		if s.taskSvc == nil {
			return unavailableDocsEntityRef(ref, entityType)
		}
		detail, err := s.taskSvc.GetByID(ctx, entityID)
		if err != nil || detail == nil || detail.Task.WorkspaceID != workspaceID {
			if s.taskExistsInWorkspace(ctx, workspaceID, entityID) {
				return redactedDocsEntityRef(ref, entityType)
			}
			return unavailableDocsEntityRef(ref, entityType)
		}
		title := detail.Task.Name
		state := ""
		if detail.State != nil {
			state = detail.State.Name
		} else if detail.Task.Completed {
			state = "Done"
		}
		displayID := interface{}(detail.Task.DisplayID)
		if detail.Task.TaskKey != "" {
			displayID = detail.Task.TaskKey
		}
		return availableDocsEntityRef(ref, entityType, title, displayID, firstNonBlank(detail.Task.TaskKey, fmt.Sprintf("#%d", detail.Task.DisplayID)), state)
	case "epic":
		if !s.canReadModule(ctx, model.ModulePM, authorization.PermPMRead) {
			return redactedDocsEntityRef(ref, entityType)
		}
		if s.epicSvc == nil {
			return unavailableDocsEntityRef(ref, entityType)
		}
		epic, err := s.epicSvc.GetByID(ctx, entityID)
		if err != nil || epic == nil || epic.Epic.WorkspaceID != workspaceID {
			if s.epicExistsInWorkspace(ctx, workspaceID, entityID) {
				return redactedDocsEntityRef(ref, entityType)
			}
			return unavailableDocsEntityRef(ref, entityType)
		}
		meta := fmt.Sprintf("%d/%d tasks done", epic.Stats.DoneTaskCount, epic.Stats.TaskCount)
		return availableDocsEntityRef(ref, entityType, epic.Epic.Name, ref.DisplayID, meta, epic.Epic.Health)
	case "support_conversation":
		if !s.canReadModule(ctx, model.ModuleSupport, authorization.PermSupportRead) {
			return redactedDocsEntityRef(ref, entityType)
		}
		if s.supportSvc == nil {
			return unavailableDocsEntityRef(ref, entityType)
		}
		conversation, err := s.supportSvc.GetConversation(ctx, workspaceID, entityID)
		if err != nil || conversation == nil || conversation.WorkspaceID != workspaceID {
			if s.supportConversationExistsInWorkspace(ctx, workspaceID, entityID) {
				return redactedDocsEntityRef(ref, entityType)
			}
			return unavailableDocsEntityRef(ref, entityType)
		}
		title := firstNonBlank(conversation.Subject, fmt.Sprintf("Conversation #%d", conversation.DisplayID))
		meta := firstNonBlank(strings.Join(nonBlankStrings(derefString(conversation.CustomerEmail), derefString(conversation.MailboxName)), " · "), fmt.Sprintf("#%d", conversation.DisplayID))
		return availableDocsEntityRef(ref, entityType, title, conversation.DisplayID, meta, conversation.Status)
	case "deal":
		if !s.canReadModule(ctx, model.ModuleCRM, authorization.PermCRMRead) {
			return redactedDocsEntityRef(ref, entityType)
		}
		if s.dealSvc == nil {
			return unavailableDocsEntityRef(ref, entityType)
		}
		deal, err := s.dealSvc.GetByID(ctx, entityID)
		if err != nil || deal == nil || deal.WorkspaceID != workspaceID {
			return unavailableDocsEntityRef(ref, entityType)
		}
		pipeline := ""
		stage := ""
		if deal.Pipeline != nil {
			pipeline = deal.Pipeline.Name
		}
		if deal.Stage != nil {
			stage = deal.Stage.Name
		}
		meta := strings.Join(nonBlankStrings(formatDocsDealAmount(deal), pipeline, stage), " · ")
		return availableDocsEntityRef(ref, entityType, deal.Name, deal.DisplayID, meta, stage)
	case "contact":
		if !s.canReadModule(ctx, model.ModuleCRM, authorization.PermCRMRead) {
			return redactedDocsEntityRef(ref, entityType)
		}
		if s.contactSvc == nil {
			return unavailableDocsEntityRef(ref, entityType)
		}
		contact, err := s.contactSvc.GetByID(ctx, entityID)
		if err != nil || contact == nil || contact.WorkspaceID != workspaceID {
			return unavailableDocsEntityRef(ref, entityType)
		}
		title := strings.TrimSpace(strings.Join(nonBlankStrings(contact.FirstName, derefString(contact.LastName)), " "))
		title = firstNonBlank(title, derefString(contact.Email), "Contact")
		meta := strings.Join(nonBlankStrings(derefString(contact.Email), derefString(contact.JobTitle)), " · ")
		return availableDocsEntityRef(ref, entityType, title, contact.DisplayID, meta, contact.LifecycleStage)
	case "company":
		if !s.canReadModule(ctx, model.ModuleCRM, authorization.PermCRMRead) {
			return redactedDocsEntityRef(ref, entityType)
		}
		if s.companySvc == nil {
			return unavailableDocsEntityRef(ref, entityType)
		}
		company, err := s.companySvc.GetByID(ctx, entityID)
		if err != nil || company == nil || company.WorkspaceID != workspaceID {
			return unavailableDocsEntityRef(ref, entityType)
		}
		meta := strings.Join(nonBlankStrings(derefString(company.Domain), derefString(company.Industry)), " · ")
		return availableDocsEntityRef(ref, entityType, company.Name, company.DisplayID, meta, "")
	case "document":
		if !s.canReadModule(ctx, model.ModuleDocs, authorization.PermDocsRead) {
			return redactedDocsEntityRef(ref, entityType)
		}
		if s.documentSvc == nil {
			return unavailableDocsEntityRef(ref, entityType)
		}
		doc, err := s.documentSvc.Get(ctx, entityID)
		if err != nil || doc == nil || doc.WorkspaceID != workspaceID {
			return unavailableDocsEntityRef(ref, entityType)
		}
		return availableDocsEntityRef(ref, entityType, doc.Title, ref.DisplayID, "Doc", doc.Status)
	default:
		return unavailableDocsEntityRef(ref, entityType)
	}
}

func (s *DocsEntityReferenceResolverService) canReadModule(ctx context.Context, module model.ModuleID, perm authorization.Permission) bool {
	actor := authorization.GetActor(ctx)
	if actor == nil {
		return true
	}
	if s.authzSvc != nil {
		if !s.authzSvc.Can(actor, perm) {
			return false
		}
		canAccess, err := s.authzSvc.CanAccessModule(ctx, actor, module)
		return err == nil && canAccess
	}
	return authorization.NewRBACEngine().Can(actor.Role, perm)
}

func (s *DocsEntityReferenceResolverService) taskExistsInWorkspace(ctx context.Context, workspaceID, entityID string) bool {
	if s.taskSvc == nil || s.taskSvc.taskRepo == nil {
		return false
	}
	detail, err := s.taskSvc.taskRepo.GetByID(ctx, entityID)
	return err == nil && detail != nil && detail.Task.WorkspaceID == workspaceID
}

func (s *DocsEntityReferenceResolverService) epicExistsInWorkspace(ctx context.Context, workspaceID, entityID string) bool {
	if s.epicSvc == nil || s.epicSvc.epicRepo == nil {
		return false
	}
	epic, err := s.epicSvc.epicRepo.GetWithStats(ctx, entityID)
	return err == nil && epic != nil && epic.Epic.WorkspaceID == workspaceID
}

func (s *DocsEntityReferenceResolverService) supportConversationExistsInWorkspace(ctx context.Context, workspaceID, entityID string) bool {
	if s.supportSvc == nil {
		return false
	}
	conversation, err := s.supportSvc.loadConversationUnscoped(ctx, workspaceID, entityID)
	return err == nil && conversation != nil && conversation.WorkspaceID == workspaceID
}

func availableDocsEntityRef(ref model.DocsEntityRefRequest, entityType, title string, displayID interface{}, meta, stateLabel string) model.DocsResolvedEntityRef {
	return model.DocsResolvedEntityRef{
		EntityType: entityType,
		EntityID:   strings.TrimSpace(ref.EntityID),
		Status:     docsEntityRefStatusAvailable,
		Access:     docsEntityRefAccessGranted,
		Title:      firstNonBlank(title, ref.Label, docsEntityTypeLabel(entityType)),
		DisplayID:  displayID,
		Meta:       strings.TrimSpace(meta),
		StateLabel: strings.TrimSpace(stateLabel),
	}
}

func unavailableDocsEntityRef(ref model.DocsEntityRefRequest, entityType string) model.DocsResolvedEntityRef {
	entityType = firstNonBlank(entityType, normalizeDocsEntityRefType(ref.EntityType), strings.TrimSpace(ref.EntityType))
	return model.DocsResolvedEntityRef{
		EntityType: entityType,
		EntityID:   strings.TrimSpace(ref.EntityID),
		Status:     docsEntityRefStatusUnavailable,
		Access:     docsEntityRefAccessUnavailable,
		Title:      firstNonBlank(ref.Label, docsEntityTypeLabel(entityType), "Linked entity"),
		DisplayID:  ref.DisplayID,
		Meta:       "Reference unavailable",
	}
}

func redactedDocsEntityRef(ref model.DocsEntityRefRequest, entityType string) model.DocsResolvedEntityRef {
	entityType = firstNonBlank(entityType, normalizeDocsEntityRefType(ref.EntityType), strings.TrimSpace(ref.EntityType))
	return model.DocsResolvedEntityRef{
		EntityType: entityType,
		EntityID:   strings.TrimSpace(ref.EntityID),
		Status:     docsEntityRefStatusUnavailable,
		Access:     docsEntityRefAccessRedacted,
		Title:      "Restricted reference",
		Meta:       "You do not have access to this reference",
	}
}

func normalizeDocsEntityRefType(entityType string) string {
	switch strings.TrimSpace(strings.ToLower(entityType)) {
	case "task", "story", "epic", "support_conversation", "deal", "contact", "company", "document":
		return strings.TrimSpace(strings.ToLower(entityType))
	case "conversation", "ticket", "support":
		return "support_conversation"
	case "doc", "docs":
		return "document"
	default:
		return ""
	}
}

func docsEntityTypeLabel(entityType string) string {
	switch entityType {
	case "epic":
		return "Epic"
	case "support_conversation":
		return "Conversation"
	case "deal":
		return "Deal"
	case "contact":
		return "Contact"
	case "company":
		return "Company"
	case "story":
		return "Story"
	case "document":
		return "Doc"
	default:
		return "Task"
	}
}

func nonBlankStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func formatDocsDealAmount(deal *model.CRMDeal) string {
	if deal == nil || deal.Amount == nil {
		return ""
	}
	currency := strings.TrimSpace(deal.Currency)
	if currency == "" {
		currency = "USD"
	}
	return fmt.Sprintf("%.0f %s", *deal.Amount, currency)
}
