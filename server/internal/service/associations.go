package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AssociationsService aggregates task relationships, CRM/support links, and docs links into one read model.
type AssociationsService struct {
	assocRepo        *repository.CRMAssociationRepository
	contactRepo      *repository.CRMContactRepository
	workspaceRepo    *repository.WorkspaceRepository
	taskLinkRepo     *repository.PMTaskLinkRepository
	taskRepo         *repository.PMTaskRepository
	supportRepo      *repository.SupportConversationRepository
	docsLinkRepo     *repository.DocsLinkRepository
	docsDocumentRepo *repository.DocsDocumentRepository
}

// NewAssociationsService creates a new AssociationsService.
func NewAssociationsService(
	assocRepo *repository.CRMAssociationRepository,
	contactRepo *repository.CRMContactRepository,
	workspaceRepo *repository.WorkspaceRepository,
	taskLinkRepo *repository.PMTaskLinkRepository,
	taskRepo *repository.PMTaskRepository,
	supportRepo *repository.SupportConversationRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	docsDocumentRepo *repository.DocsDocumentRepository,
) *AssociationsService {
	return &AssociationsService{
		assocRepo:        assocRepo,
		contactRepo:      contactRepo,
		workspaceRepo:    workspaceRepo,
		taskLinkRepo:     taskLinkRepo,
		taskRepo:         taskRepo,
		supportRepo:      supportRepo,
		docsLinkRepo:     docsLinkRepo,
		docsDocumentRepo: docsDocumentRepo,
	}
}

// ListGrouped returns grouped associations for a task, epic, or support ticket.
func (s *AssociationsService) ListGrouped(ctx context.Context, workspaceID, objectType, objectID string) (*model.GroupedAssociationsResponse, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if !isSupportedAssociationsObjectType(objectType) {
		return nil, fmt.Errorf("unsupported associations object type %q", objectType)
	}
	if objectID == "" {
		return nil, fmt.Errorf("object id is required")
	}

	response := &model.GroupedAssociationsResponse{
		TaskRelationships: model.TaskRelationshipGroups{
			BlockedBy:    []model.TaskRelationshipSummary{},
			Blocking:     []model.TaskRelationshipSummary{},
			RelatesTo:    []model.TaskRelationshipSummary{},
			RelatedBy:    []model.TaskRelationshipSummary{},
			Duplicates:   []model.TaskRelationshipSummary{},
			DuplicatedBy: []model.TaskRelationshipSummary{},
		},
		Tasks:                []model.AssociationObjectSummary{},
		SupportConversations: []model.AssociationObjectSummary{},
		CRMRecords:           []model.AssociationObjectSummary{},
		Docs:                 []model.AssociationObjectSummary{},
	}

	workspaceKey := s.workspaceKey(ctx, workspaceID)

	if objectType == model.CRMObjectTask {
		relationships, err := s.loadTaskRelationships(ctx, workspaceID, objectID, workspaceKey)
		if err != nil {
			return nil, err
		}
		response.TaskRelationships = relationships
	}

	if err := s.populateCrossObjectAssociations(ctx, workspaceID, objectType, objectID, workspaceKey, response); err != nil {
		return nil, err
	}
	if err := s.populateInferredCRMContext(ctx, workspaceID, objectType, response); err != nil {
		return nil, err
	}
	if err := s.populateDocsAssociations(ctx, workspaceID, objectType, objectID, response); err != nil {
		return nil, err
	}
	if err := s.populateLegacySupportLinks(ctx, workspaceID, objectType, objectID, workspaceKey, response); err != nil {
		return nil, err
	}

	return response, nil
}

// CreateTaskRelationship creates a canonical task relationship from the current task's point of view.
func (s *AssociationsService) CreateTaskRelationship(ctx context.Context, workspaceID, currentTaskID, actorID string, req model.CreateTaskRelationshipRequest) (*model.PMTaskLink, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if currentTaskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if actorID == "" {
		return nil, fmt.Errorf("actor_id is required")
	}
	if req.OtherTaskID == "" {
		return nil, fmt.Errorf("other_task_id is required")
	}

	tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, []string{currentTaskID, req.OtherTaskID})
	if err != nil {
		return nil, err
	}
	if len(tasks) != 2 {
		return nil, fmt.Errorf("both tasks must exist in the workspace")
	}

	sourceTaskID, targetTaskID, linkType, err := resolveRelationshipInput(currentTaskID, req)
	if err != nil {
		return nil, err
	}
	if sourceTaskID == targetTaskID {
		return nil, fmt.Errorf("a task cannot relate to itself")
	}

	existingLinks, err := s.taskLinkRepo.ListByTask(ctx, workspaceID, currentTaskID)
	if err != nil {
		return nil, err
	}
	for _, link := range existingLinks {
		if link.LinkType != linkType {
			continue
		}
		if isSameRelationshipPair(link, sourceTaskID, targetTaskID, linkType) || isReverseSymmetricPair(link, sourceTaskID, targetTaskID, linkType) {
			return &link, nil
		}
	}

	if linkType == model.PMTaskLinkTypeBlocks {
		links, err := s.taskLinkRepo.ListByWorkspaceAndType(ctx, workspaceID, model.PMTaskLinkTypeBlocks)
		if err != nil {
			return nil, err
		}
		if wouldCreateBlockCycle(links, sourceTaskID, targetTaskID) {
			return nil, fmt.Errorf("this relationship would create a circular blocking chain")
		}
	}

	link := &model.PMTaskLink{
		WorkspaceID:  workspaceID,
		SourceTaskID: sourceTaskID,
		TargetTaskID: targetTaskID,
		LinkType:     linkType,
		CreatedBy:    actorID,
	}
	if err := s.taskLinkRepo.Create(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// DeleteTaskRelationship removes a task relationship by ID.
func (s *AssociationsService) DeleteTaskRelationship(ctx context.Context, workspaceID, relationshipID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	link, err := s.taskLinkRepo.GetByID(ctx, relationshipID)
	if err != nil {
		return err
	}
	if link == nil || link.WorkspaceID != workspaceID {
		return fmt.Errorf("relationship not found")
	}
	return s.taskLinkRepo.Delete(ctx, relationshipID)
}

func (s *AssociationsService) populateCrossObjectAssociations(ctx context.Context, workspaceID, objectType, objectID, workspaceKey string, response *model.GroupedAssociationsResponse) error {
	assocs, err := s.assocRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return err
	}
	if len(assocs) == 0 {
		return nil
	}

	enrichedAssocs, err := s.assocRepo.ListByObjectEnriched(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return err
	}
	enrichedByID := make(map[string]model.CRMAssociationEnriched, len(enrichedAssocs))
	for _, assoc := range enrichedAssocs {
		enrichedByID[assoc.ID] = assoc
	}

	taskIDs := make([]string, 0)
	supportConversationIDs := make([]string, 0)
	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc, objectType, objectID)
		switch otherType {
		case model.CRMObjectTask:
			taskIDs = append(taskIDs, otherID)
		case model.CRMObjectSupportConversation:
			supportConversationIDs = append(supportConversationIDs, otherID)
		}
	}

	tasksByID := make(map[string]model.PMTask)
	taskStatusByID := make(map[string]string)
	if len(taskIDs) > 0 {
		tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, uniqueStrings(taskIDs))
		if err != nil {
			return err
		}
		taskStatusByID, err = s.taskStatusByID(ctx, tasks)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			tasksByID[task.ID] = task
		}
	}

	conversationsByID := make(map[string]model.SupportConversation)
	if len(supportConversationIDs) > 0 {
		conversations, err := s.supportRepo.ListByIDs(ctx, workspaceID, uniqueStrings(supportConversationIDs), "", model.RoleOwner)
		if err != nil {
			return err
		}
		for _, conversation := range conversations {
			conversationsByID[conversation.ID] = conversation
		}
	}

	seenTasks := make(map[string]struct{})
	seenSupport := make(map[string]struct{})
	seenCRM := make(map[string]struct{})

	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc, objectType, objectID)
		switch otherType {
		case model.CRMObjectTask:
			task, ok := tasksByID[otherID]
			if !ok {
				continue
			}
			if _, exists := seenTasks[task.ID]; exists {
				continue
			}
			seenTasks[task.ID] = struct{}{}
			response.Tasks = append(response.Tasks, taskAssociationSummary(assoc.ID, task, workspaceKey, taskStatusByID[task.ID]))
		case model.CRMObjectSupportConversation:
			conversation, ok := conversationsByID[otherID]
			if !ok {
				continue
			}
			if _, exists := seenSupport[conversation.ID]; exists {
				continue
			}
			seenSupport[conversation.ID] = struct{}{}
			response.SupportConversations = append(response.SupportConversations, supportAssociationSummary(assoc.ID, conversation))
		case model.CRMObjectContact, model.CRMObjectCompany, model.CRMObjectDeal:
			if _, exists := seenCRM[assoc.ID]; exists {
				continue
			}
			enriched, ok := enrichedByID[assoc.ID]
			if !ok {
				continue
			}
			seenCRM[assoc.ID] = struct{}{}
			response.CRMRecords = append(response.CRMRecords, crmAssociationSummary(enriched, otherType, otherID))
		}
	}

	sortAssociationObjects(response.Tasks)
	sortAssociationObjects(response.SupportConversations)
	sortAssociationObjects(response.CRMRecords)

	return nil
}

func (s *AssociationsService) populateInferredCRMContext(ctx context.Context, workspaceID, objectType string, response *model.GroupedAssociationsResponse) error {
	if objectType != model.CRMObjectTask && objectType != model.CRMObjectSupportConversation {
		return nil
	}
	if len(response.CRMRecords) == 0 {
		return nil
	}

	directSources := make([]model.AssociationObjectSummary, 0, len(response.CRMRecords))
	seen := make(map[string]struct{}, len(response.CRMRecords))
	for _, item := range response.CRMRecords {
		key := item.ObjectType + ":" + item.ObjectID
		seen[key] = struct{}{}
		if item.Inferred {
			continue
		}
		if !isCRMRecordObjectType(item.ObjectType) {
			continue
		}
		directSources = append(directSources, item)
	}

	for _, source := range directSources {
		enrichedAssocs, err := s.assocRepo.ListByObjectEnriched(ctx, workspaceID, source.ObjectType, source.ObjectID)
		if err != nil {
			return err
		}
		enrichedAssocs = preferredCRMInferredAssociations(source, enrichedAssocs)
		for _, assoc := range enrichedAssocs {
			otherType, otherID := otherAssociationSide(assoc.CRMAssociation, source.ObjectType, source.ObjectID)
			if !isCRMRecordObjectType(otherType) {
				continue
			}
			key := otherType + ":" + otherID
			if _, exists := seen[key]; exists {
				continue
			}
			summary := crmAssociationSummary(assoc, otherType, otherID)
			summary.AssociationID = ""
			summary.Inferred = true
			label := "via " + source.Title
			summary.ContextLabel = &label
			response.CRMRecords = append(response.CRMRecords, summary)
			seen[key] = struct{}{}
		}
	}

	sortAssociationObjects(response.CRMRecords)
	return nil
}

func preferredCRMInferredAssociations(source model.AssociationObjectSummary, assocs []model.CRMAssociationEnriched) []model.CRMAssociationEnriched {
	if source.ObjectType != model.CRMObjectContact || len(assocs) == 0 {
		return assocs
	}

	hasPrimaryCompany := false
	for _, assoc := range assocs {
		otherType, _ := otherAssociationSide(assoc.CRMAssociation, source.ObjectType, source.ObjectID)
		if otherType == model.CRMObjectCompany && isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
			hasPrimaryCompany = true
			break
		}
	}
	if !hasPrimaryCompany {
		return assocs
	}

	filtered := make([]model.CRMAssociationEnriched, 0, len(assocs))
	for _, assoc := range assocs {
		otherType, _ := otherAssociationSide(assoc.CRMAssociation, source.ObjectType, source.ObjectID)
		if otherType != model.CRMObjectCompany || isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
			filtered = append(filtered, assoc)
		}
	}
	return filtered
}

func (s *AssociationsService) populateDocsAssociations(ctx context.Context, workspaceID, objectType, objectID string, response *model.GroupedAssociationsResponse) error {
	links, err := s.docsLinkRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}

	docIDs := make([]string, 0, len(links))
	for _, link := range links {
		docIDs = append(docIDs, link.DocumentID)
	}
	docs, err := s.docsDocumentRepo.ListByIDs(ctx, workspaceID, uniqueStrings(docIDs))
	if err != nil {
		return err
	}
	docsByID := make(map[string]model.DocsDocument, len(docs))
	for _, doc := range docs {
		docsByID[doc.ID] = doc
	}

	for _, link := range links {
		doc, ok := docsByID[link.DocumentID]
		if !ok {
			continue
		}
		alreadyIncluded := false
		for _, existing := range response.Docs {
			if existing.ObjectID == doc.ID {
				alreadyIncluded = true
				break
			}
		}
		if alreadyIncluded {
			continue
		}
		status := doc.Status
		response.Docs = append(response.Docs, model.AssociationObjectSummary{
			AssociationID: link.ID,
			ObjectType:    "document",
			ObjectID:      doc.ID,
			Title:         doc.Title,
			Status:        &status,
		})
	}
	sortAssociationObjects(response.Docs)
	return nil
}

func (s *AssociationsService) populateLegacySupportLinks(ctx context.Context, workspaceID, objectType, objectID, workspaceKey string, response *model.GroupedAssociationsResponse) error {
	switch objectType {
	case model.CRMObjectTask:
		conversations, err := s.supportRepo.ListByLinkedTaskIDs(ctx, workspaceID, []string{objectID})
		if err != nil {
			return err
		}
		existing := make(map[string]struct{}, len(response.SupportConversations))
		for _, item := range response.SupportConversations {
			existing[item.ObjectID] = struct{}{}
		}
		for _, conversation := range conversations {
			if _, ok := existing[conversation.ID]; ok {
				continue
			}
			response.SupportConversations = append(response.SupportConversations, supportAssociationSummary("", conversation))
		}
		sortAssociationObjects(response.SupportConversations)
	case model.CRMObjectSupportConversation:
		conversation, err := s.supportRepo.GetByID(ctx, workspaceID, objectID, "", model.RoleOwner)
		if err != nil {
			return err
		}
		if conversation == nil {
			return nil
		}
		existingCRM := make(map[string]struct{}, len(response.CRMRecords))
		for _, item := range response.CRMRecords {
			existingCRM[item.ObjectType+":"+item.ObjectID] = struct{}{}
		}
		if conversation.CRMContactID != nil && *conversation.CRMContactID != "" {
			key := model.CRMObjectContact + ":" + *conversation.CRMContactID
			if _, ok := existingCRM[key]; !ok && s.contactRepo != nil {
				contact, err := s.contactRepo.GetByID(ctx, *conversation.CRMContactID)
				if err != nil {
					return err
				}
				if contact != nil && contact.WorkspaceID == workspaceID {
					response.CRMRecords = append(response.CRMRecords, contactAssociationSummary("", *contact))
					sortAssociationObjects(response.CRMRecords)
				}
			}
		}
		if conversation.LinkedTaskID == nil || *conversation.LinkedTaskID == "" {
			return nil
		}
		existing := make(map[string]struct{}, len(response.Tasks))
		for _, item := range response.Tasks {
			existing[item.ObjectID] = struct{}{}
		}
		if _, ok := existing[*conversation.LinkedTaskID]; ok {
			return nil
		}
		tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, []string{*conversation.LinkedTaskID})
		if err != nil {
			return err
		}
		if len(tasks) == 1 {
			taskStatusByID, err := s.taskStatusByID(ctx, tasks)
			if err != nil {
				return err
			}
			response.Tasks = append(response.Tasks, taskAssociationSummary("", tasks[0], workspaceKey, taskStatusByID[tasks[0].ID]))
			sortAssociationObjects(response.Tasks)
		}
	}
	return nil
}

func (s *AssociationsService) loadTaskRelationships(ctx context.Context, workspaceID, taskID, workspaceKey string) (model.TaskRelationshipGroups, error) {
	response := model.TaskRelationshipGroups{
		BlockedBy:    []model.TaskRelationshipSummary{},
		Blocking:     []model.TaskRelationshipSummary{},
		RelatesTo:    []model.TaskRelationshipSummary{},
		RelatedBy:    []model.TaskRelationshipSummary{},
		Duplicates:   []model.TaskRelationshipSummary{},
		DuplicatedBy: []model.TaskRelationshipSummary{},
	}

	links, err := s.taskLinkRepo.ListByTask(ctx, workspaceID, taskID)
	if err != nil {
		return response, err
	}
	if len(links) == 0 {
		return response, nil
	}

	otherIDs := make([]string, 0, len(links))
	for _, link := range links {
		if link.SourceTaskID == taskID {
			otherIDs = append(otherIDs, link.TargetTaskID)
		} else {
			otherIDs = append(otherIDs, link.SourceTaskID)
		}
	}
	tasks, err := s.taskRepo.ListByIDs(ctx, workspaceID, uniqueStrings(otherIDs))
	if err != nil {
		return response, err
	}
	taskStatusByID, err := s.taskStatusByID(ctx, tasks)
	if err != nil {
		return response, err
	}
	tasksByID := make(map[string]model.PMTask, len(tasks))
	for _, task := range tasks {
		tasksByID[task.ID] = task
	}

	for _, link := range links {
		otherID := link.SourceTaskID
		if otherID == taskID {
			otherID = link.TargetTaskID
		}
		otherTask, ok := tasksByID[otherID]
		if !ok {
			continue
		}

		summary := model.TaskRelationshipSummary{
			RelationshipID: link.ID,
			LinkType:       link.LinkType,
			IsActive:       !otherTask.Completed,
			Task:           taskAssociationSummary("", otherTask, workspaceKey, taskStatusByID[otherTask.ID]),
		}
		switch link.LinkType {
		case model.PMTaskLinkTypeBlocks:
			if link.TargetTaskID == taskID {
				response.BlockedBy = append(response.BlockedBy, summary)
			} else {
				response.Blocking = append(response.Blocking, summary)
			}
		case model.PMTaskLinkTypeRelatesTo:
			if link.SourceTaskID == taskID {
				response.RelatesTo = append(response.RelatesTo, summary)
			} else {
				response.RelatedBy = append(response.RelatedBy, summary)
			}
		case model.PMTaskLinkTypeDuplicates:
			if link.SourceTaskID == taskID {
				response.Duplicates = append(response.Duplicates, summary)
			} else {
				response.DuplicatedBy = append(response.DuplicatedBy, summary)
			}
		}
	}

	sortTaskRelationshipGroup(response.BlockedBy)
	sortTaskRelationshipGroup(response.Blocking)
	sortTaskRelationshipGroup(response.RelatesTo)
	sortTaskRelationshipGroup(response.RelatedBy)
	sortTaskRelationshipGroup(response.Duplicates)
	sortTaskRelationshipGroup(response.DuplicatedBy)

	return response, nil
}

func resolveRelationshipInput(currentTaskID string, req model.CreateTaskRelationshipRequest) (string, string, string, error) {
	switch req.RelationshipType {
	case model.TaskRelationshipActionRelatesTo:
		return currentTaskID, req.OtherTaskID, model.PMTaskLinkTypeRelatesTo, nil
	case model.TaskRelationshipActionBlocks:
		return currentTaskID, req.OtherTaskID, model.PMTaskLinkTypeBlocks, nil
	case model.TaskRelationshipActionIsBlockedBy:
		return req.OtherTaskID, currentTaskID, model.PMTaskLinkTypeBlocks, nil
	case model.TaskRelationshipActionDuplicates:
		return currentTaskID, req.OtherTaskID, model.PMTaskLinkTypeDuplicates, nil
	case model.TaskRelationshipActionIsDuplicatedBy:
		return req.OtherTaskID, currentTaskID, model.PMTaskLinkTypeDuplicates, nil
	default:
		return "", "", "", fmt.Errorf("unsupported relationship type %q", req.RelationshipType)
	}
}

func isSameRelationshipPair(link model.PMTaskLink, sourceTaskID, targetTaskID, linkType string) bool {
	return link.LinkType == linkType && link.SourceTaskID == sourceTaskID && link.TargetTaskID == targetTaskID
}

func isReverseSymmetricPair(link model.PMTaskLink, sourceTaskID, targetTaskID, linkType string) bool {
	if link.LinkType != linkType {
		return false
	}
	if linkType != model.PMTaskLinkTypeRelatesTo && linkType != model.PMTaskLinkTypeDuplicates {
		return false
	}
	return link.SourceTaskID == targetTaskID && link.TargetTaskID == sourceTaskID
}

func wouldCreateBlockCycle(existing []model.PMTaskLink, sourceTaskID, targetTaskID string) bool {
	graph := make(map[string][]string)
	for _, link := range existing {
		graph[link.SourceTaskID] = append(graph[link.SourceTaskID], link.TargetTaskID)
	}
	graph[sourceTaskID] = append(graph[sourceTaskID], targetTaskID)

	seen := map[string]struct{}{}
	var visit func(string) bool
	visit = func(node string) bool {
		if node == sourceTaskID {
			return true
		}
		if _, ok := seen[node]; ok {
			return false
		}
		seen[node] = struct{}{}
		for _, next := range graph[node] {
			if visit(next) {
				return true
			}
		}
		return false
	}

	return visit(targetTaskID)
}

func otherAssociationSide(assoc model.CRMAssociation, objectType, objectID string) (string, string) {
	if assoc.FromObjectType == objectType && assoc.FromObjectID == objectID {
		return assoc.ToObjectType, assoc.ToObjectID
	}
	return assoc.FromObjectType, assoc.FromObjectID
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	set := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := set[value]; ok {
			continue
		}
		set[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *AssociationsService) taskStatusByID(ctx context.Context, tasks []model.PMTask) (map[string]string, error) {
	if len(tasks) == 0 {
		return map[string]string{}, nil
	}

	stateIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.WorkflowStateID != "" {
			stateIDs = append(stateIDs, task.WorkflowStateID)
		}
	}

	stateNamesByID, err := s.taskRepo.ListStateNamesByIDs(ctx, uniqueStrings(stateIDs))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return map[string]string{}, nil
		}
		return nil, err
	}

	result := make(map[string]string, len(tasks))
	for _, task := range tasks {
		if name := stateNamesByID[task.WorkflowStateID]; name != "" {
			result[task.ID] = name
		}
	}
	return result, nil
}

func (s *AssociationsService) workspaceKey(ctx context.Context, workspaceID string) string {
	if s.workspaceRepo == nil || workspaceID == "" {
		return ""
	}
	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return ""
		}
		return ""
	}
	if ws == nil {
		return ""
	}
	return ws.WorkspaceKey
}

func taskAssociationSummary(associationID string, task model.PMTask, workspaceKey, status string) model.AssociationObjectSummary {
	displayID := fmt.Sprintf("%d", task.DisplayID)
	if workspaceKey != "" {
		displayID = model.FormatTaskKey(workspaceKey, task.DisplayID)
	}
	workflowStateID := task.WorkflowStateID
	taskType := task.TaskType
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	return model.AssociationObjectSummary{
		AssociationID:   associationID,
		ObjectType:      model.CRMObjectTask,
		ObjectID:        task.ID,
		DisplayID:       &displayID,
		Title:           task.Name,
		Status:          statusPtr,
		WorkflowStateID: &workflowStateID,
		Completed:       task.Completed,
		TaskType:        &taskType,
	}
}

func supportAssociationSummary(associationID string, conversation model.SupportConversation) model.AssociationObjectSummary {
	displayID := fmt.Sprintf("T-%d", conversation.DisplayID)
	status := conversation.Status
	return model.AssociationObjectSummary{
		AssociationID: associationID,
		ObjectType:    model.CRMObjectSupportConversation,
		ObjectID:      conversation.ID,
		DisplayID:     &displayID,
		Title:         conversation.Subject,
		Status:        &status,
	}
}

func crmAssociationSummary(assoc model.CRMAssociationEnriched, objectType, objectID string) model.AssociationObjectSummary {
	title := assoc.LinkedObjectName
	if title == "" {
		title = objectType
	}
	var displayID *string
	if assoc.LinkedObjectDisplayID != "" {
		displayID = &assoc.LinkedObjectDisplayID
	}
	return model.AssociationObjectSummary{
		AssociationID: assoc.ID,
		ObjectType:    objectType,
		ObjectID:      objectID,
		DisplayID:     displayID,
		Title:         title,
	}
}

func contactAssociationSummary(associationID string, contact model.CRMContact) model.AssociationObjectSummary {
	displayID := contact.DisplayID
	return model.AssociationObjectSummary{
		AssociationID: associationID,
		ObjectType:    model.CRMObjectContact,
		ObjectID:      contact.ID,
		DisplayID:     &displayID,
		Title:         contactDisplayName(contact),
	}
}

func sortAssociationObjects(items []model.AssociationObjectSummary) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Inferred != items[j].Inferred {
			return !items[i].Inferred
		}
		return items[i].Title < items[j].Title
	})
}

func sortTaskRelationshipGroup(items []model.TaskRelationshipSummary) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsActive != items[j].IsActive {
			return items[i].IsActive
		}
		return items[i].Task.Title < items[j].Task.Title
	})
}

func isSupportedAssociationsObjectType(objectType string) bool {
	switch objectType {
	case model.CRMObjectTask, model.CRMObjectEpic, model.CRMObjectSupportConversation:
		return true
	default:
		return false
	}
}

func isCRMRecordObjectType(objectType string) bool {
	switch objectType {
	case model.CRMObjectContact, model.CRMObjectCompany, model.CRMObjectDeal:
		return true
	default:
		return false
	}
}
