package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AssociationsService aggregates task relationships, CRM/support links, and docs links into one read model.
type AssociationsService struct {
	assocRepo        *repository.CRMAssociationRepository
	storyLinkRepo    *repository.PMStoryLinkRepository
	storyRepo        *repository.PMStoryRepository
	supportRepo      *repository.SupportConversationRepository
	docsLinkRepo     *repository.DocsLinkRepository
	docsDocumentRepo *repository.DocsDocumentRepository
}

// NewAssociationsService creates a new AssociationsService.
func NewAssociationsService(
	assocRepo *repository.CRMAssociationRepository,
	storyLinkRepo *repository.PMStoryLinkRepository,
	storyRepo *repository.PMStoryRepository,
	supportRepo *repository.SupportConversationRepository,
	docsLinkRepo *repository.DocsLinkRepository,
	docsDocumentRepo *repository.DocsDocumentRepository,
) *AssociationsService {
	return &AssociationsService{
		assocRepo:        assocRepo,
		storyLinkRepo:    storyLinkRepo,
		storyRepo:        storyRepo,
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
		TaskRelationships: model.StoryRelationshipGroups{
			BlockedBy:    []model.StoryRelationshipSummary{},
			Blocking:     []model.StoryRelationshipSummary{},
			RelatesTo:    []model.StoryRelationshipSummary{},
			RelatedBy:    []model.StoryRelationshipSummary{},
			Duplicates:   []model.StoryRelationshipSummary{},
			DuplicatedBy: []model.StoryRelationshipSummary{},
		},
		Stories:              []model.AssociationObjectSummary{},
		SupportConversations: []model.AssociationObjectSummary{},
		CRMRecords:           []model.AssociationObjectSummary{},
		Docs:                 []model.AssociationObjectSummary{},
	}

	if objectType == model.CRMObjectTask || objectType == model.CRMObjectStory {
		relationships, err := s.loadStoryRelationships(ctx, workspaceID, objectID)
		if err != nil {
			return nil, err
		}
		response.TaskRelationships = relationships
	}

	if err := s.populateCrossObjectAssociations(ctx, workspaceID, objectType, objectID, response); err != nil {
		return nil, err
	}
	if err := s.populateDocsAssociations(ctx, workspaceID, objectType, objectID, response); err != nil {
		return nil, err
	}
	if err := s.populateLegacySupportLinks(ctx, workspaceID, objectType, objectID, response); err != nil {
		return nil, err
	}

	return response, nil
}

// CreateStoryRelationship creates a canonical task relationship from the current task's point of view.
func (s *AssociationsService) CreateStoryRelationship(ctx context.Context, workspaceID, currentStoryID, actorID string, req model.CreateStoryRelationshipRequest) (*model.PMStoryLink, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if currentStoryID == "" {
		return nil, fmt.Errorf("story id is required")
	}
	if actorID == "" {
		return nil, fmt.Errorf("actor_id is required")
	}
	if req.OtherTaskID == "" {
		return nil, fmt.Errorf("other_task_id is required")
	}

	stories, err := s.storyRepo.ListByIDs(ctx, workspaceID, []string{currentStoryID, req.OtherTaskID})
	if err != nil {
		return nil, err
	}
	if len(stories) != 2 {
		return nil, fmt.Errorf("both stories must exist in the workspace")
	}

	sourceStoryID, targetStoryID, linkType, err := resolveRelationshipInput(currentStoryID, req)
	if err != nil {
		return nil, err
	}
	if sourceStoryID == targetStoryID {
		return nil, fmt.Errorf("a story cannot relate to itself")
	}

	existingLinks, err := s.storyLinkRepo.ListByStory(ctx, workspaceID, currentStoryID)
	if err != nil {
		return nil, err
	}
	for _, link := range existingLinks {
		if link.LinkType != linkType {
			continue
		}
		if isSameRelationshipPair(link, sourceStoryID, targetStoryID, linkType) || isReverseSymmetricPair(link, sourceStoryID, targetStoryID, linkType) {
			return &link, nil
		}
	}

	if linkType == model.PMStoryLinkTypeBlocks {
		links, err := s.storyLinkRepo.ListByWorkspaceAndType(ctx, workspaceID, model.PMStoryLinkTypeBlocks)
		if err != nil {
			return nil, err
		}
		if wouldCreateBlockCycle(links, sourceStoryID, targetStoryID) {
			return nil, fmt.Errorf("this relationship would create a circular blocking chain")
		}
	}

	link := &model.PMStoryLink{
		WorkspaceID:   workspaceID,
		SourceStoryID: sourceStoryID,
		TargetStoryID: targetStoryID,
		LinkType:      linkType,
		CreatedBy:     actorID,
	}
	if err := s.storyLinkRepo.Create(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// DeleteStoryRelationship removes a story relationship by ID.
func (s *AssociationsService) DeleteStoryRelationship(ctx context.Context, workspaceID, relationshipID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	link, err := s.storyLinkRepo.GetByID(ctx, relationshipID)
	if err != nil {
		return err
	}
	if link == nil || link.WorkspaceID != workspaceID {
		return fmt.Errorf("relationship not found")
	}
	return s.storyLinkRepo.Delete(ctx, relationshipID)
}

func (s *AssociationsService) populateCrossObjectAssociations(ctx context.Context, workspaceID, objectType, objectID string, response *model.GroupedAssociationsResponse) error {
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

	storyIDs := make([]string, 0)
	supportConversationIDs := make([]string, 0)
	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc, objectType, objectID)
		switch otherType {
		case model.CRMObjectTask, model.CRMObjectStory:
			storyIDs = append(storyIDs, otherID)
		case model.CRMObjectSupportConversation:
			supportConversationIDs = append(supportConversationIDs, otherID)
		}
	}

	storiesByID := make(map[string]model.PMStory)
	if len(storyIDs) > 0 {
		stories, err := s.storyRepo.ListByIDs(ctx, workspaceID, uniqueStrings(storyIDs))
		if err != nil {
			return err
		}
		for _, story := range stories {
			storiesByID[story.ID] = story
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

	seenStories := make(map[string]struct{})
	seenSupport := make(map[string]struct{})
	seenCRM := make(map[string]struct{})

	for _, assoc := range assocs {
		otherType, otherID := otherAssociationSide(assoc, objectType, objectID)
		switch otherType {
		case model.CRMObjectTask, model.CRMObjectStory:
			story, ok := storiesByID[otherID]
			if !ok {
				continue
			}
			if _, exists := seenStories[story.ID]; exists {
				continue
			}
			seenStories[story.ID] = struct{}{}
			response.Stories = append(response.Stories, storyAssociationSummary(assoc.ID, story))
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

	sortAssociationObjects(response.Stories)
	sortAssociationObjects(response.SupportConversations)
	sortAssociationObjects(response.CRMRecords)

	return nil
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

func (s *AssociationsService) populateLegacySupportLinks(ctx context.Context, workspaceID, objectType, objectID string, response *model.GroupedAssociationsResponse) error {
	switch objectType {
	case model.CRMObjectTask, model.CRMObjectStory:
		conversations, err := s.supportRepo.ListByLinkedStoryIDs(ctx, workspaceID, []string{objectID})
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
		if conversation == nil || conversation.LinkedTaskID == nil || *conversation.LinkedTaskID == "" {
			return nil
		}
		existing := make(map[string]struct{}, len(response.Stories))
		for _, item := range response.Stories {
			existing[item.ObjectID] = struct{}{}
		}
		if _, ok := existing[*conversation.LinkedTaskID]; ok {
			return nil
		}
		stories, err := s.storyRepo.ListByIDs(ctx, workspaceID, []string{*conversation.LinkedTaskID})
		if err != nil {
			return err
		}
		if len(stories) == 1 {
			response.Stories = append(response.Stories, storyAssociationSummary("", stories[0]))
			sortAssociationObjects(response.Stories)
		}
	}
	return nil
}

func (s *AssociationsService) loadStoryRelationships(ctx context.Context, workspaceID, storyID string) (model.StoryRelationshipGroups, error) {
	response := model.StoryRelationshipGroups{
		BlockedBy:    []model.StoryRelationshipSummary{},
		Blocking:     []model.StoryRelationshipSummary{},
		RelatesTo:    []model.StoryRelationshipSummary{},
		RelatedBy:    []model.StoryRelationshipSummary{},
		Duplicates:   []model.StoryRelationshipSummary{},
		DuplicatedBy: []model.StoryRelationshipSummary{},
	}

	links, err := s.storyLinkRepo.ListByStory(ctx, workspaceID, storyID)
	if err != nil {
		return response, err
	}
	if len(links) == 0 {
		return response, nil
	}

	otherIDs := make([]string, 0, len(links))
	for _, link := range links {
		if link.SourceStoryID == storyID {
			otherIDs = append(otherIDs, link.TargetStoryID)
		} else {
			otherIDs = append(otherIDs, link.SourceStoryID)
		}
	}
	stories, err := s.storyRepo.ListByIDs(ctx, workspaceID, uniqueStrings(otherIDs))
	if err != nil {
		return response, err
	}
	storiesByID := make(map[string]model.PMStory, len(stories))
	for _, story := range stories {
		storiesByID[story.ID] = story
	}

	for _, link := range links {
		otherID := link.SourceStoryID
		if otherID == storyID {
			otherID = link.TargetStoryID
		}
		otherStory, ok := storiesByID[otherID]
		if !ok {
			continue
		}

		summary := model.StoryRelationshipSummary{
			RelationshipID: link.ID,
			LinkType:       link.LinkType,
			IsActive:       !otherStory.Completed,
			Task:           storyAssociationSummary("", otherStory),
		}
		switch link.LinkType {
		case model.PMStoryLinkTypeBlocks:
			if link.TargetStoryID == storyID {
				response.BlockedBy = append(response.BlockedBy, summary)
			} else {
				response.Blocking = append(response.Blocking, summary)
			}
		case model.PMStoryLinkTypeRelatesTo:
			if link.SourceStoryID == storyID {
				response.RelatesTo = append(response.RelatesTo, summary)
			} else {
				response.RelatedBy = append(response.RelatedBy, summary)
			}
		case model.PMStoryLinkTypeDuplicates:
			if link.SourceStoryID == storyID {
				response.Duplicates = append(response.Duplicates, summary)
			} else {
				response.DuplicatedBy = append(response.DuplicatedBy, summary)
			}
		}
	}

	sortStoryRelationshipGroup(response.BlockedBy)
	sortStoryRelationshipGroup(response.Blocking)
	sortStoryRelationshipGroup(response.RelatesTo)
	sortStoryRelationshipGroup(response.RelatedBy)
	sortStoryRelationshipGroup(response.Duplicates)
	sortStoryRelationshipGroup(response.DuplicatedBy)

	return response, nil
}

func resolveRelationshipInput(currentStoryID string, req model.CreateStoryRelationshipRequest) (string, string, string, error) {
	switch req.RelationshipType {
	case model.StoryRelationshipActionRelatesTo:
		return currentStoryID, req.OtherTaskID, model.PMStoryLinkTypeRelatesTo, nil
	case model.StoryRelationshipActionBlocks:
		return currentStoryID, req.OtherTaskID, model.PMStoryLinkTypeBlocks, nil
	case model.StoryRelationshipActionIsBlockedBy:
		return req.OtherTaskID, currentStoryID, model.PMStoryLinkTypeBlocks, nil
	case model.StoryRelationshipActionDuplicates:
		return currentStoryID, req.OtherTaskID, model.PMStoryLinkTypeDuplicates, nil
	case model.StoryRelationshipActionIsDuplicatedBy:
		return req.OtherTaskID, currentStoryID, model.PMStoryLinkTypeDuplicates, nil
	default:
		return "", "", "", fmt.Errorf("unsupported relationship type %q", req.RelationshipType)
	}
}

func isSameRelationshipPair(link model.PMStoryLink, sourceStoryID, targetStoryID, linkType string) bool {
	return link.LinkType == linkType && link.SourceStoryID == sourceStoryID && link.TargetStoryID == targetStoryID
}

func isReverseSymmetricPair(link model.PMStoryLink, sourceStoryID, targetStoryID, linkType string) bool {
	if link.LinkType != linkType {
		return false
	}
	if linkType != model.PMStoryLinkTypeRelatesTo && linkType != model.PMStoryLinkTypeDuplicates {
		return false
	}
	return link.SourceStoryID == targetStoryID && link.TargetStoryID == sourceStoryID
}

func wouldCreateBlockCycle(existing []model.PMStoryLink, sourceStoryID, targetStoryID string) bool {
	graph := make(map[string][]string)
	for _, link := range existing {
		graph[link.SourceStoryID] = append(graph[link.SourceStoryID], link.TargetStoryID)
	}
	graph[sourceStoryID] = append(graph[sourceStoryID], targetStoryID)

	seen := map[string]struct{}{}
	var visit func(string) bool
	visit = func(node string) bool {
		if node == sourceStoryID {
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

	return visit(targetStoryID)
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

func storyAssociationSummary(associationID string, story model.PMStory) model.AssociationObjectSummary {
	displayID := fmt.Sprintf("%d", story.DisplayID)
	workflowStateID := story.WorkflowStateID
	storyType := story.TaskType
	return model.AssociationObjectSummary{
		AssociationID:   associationID,
		ObjectType:      model.CRMObjectTask,
		ObjectID:        story.ID,
		DisplayID:       &displayID,
		Title:           story.Name,
		WorkflowStateID: &workflowStateID,
		Completed:       story.Completed,
		TaskType:        &storyType,
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

func sortAssociationObjects(items []model.AssociationObjectSummary) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Title < items[j].Title
	})
}

func sortStoryRelationshipGroup(items []model.StoryRelationshipSummary) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsActive != items[j].IsActive {
			return items[i].IsActive
		}
		return items[i].Task.Title < items[j].Task.Title
	})
}

func isSupportedAssociationsObjectType(objectType string) bool {
	switch objectType {
	case model.CRMObjectTask, model.CRMObjectStory, model.CRMObjectEpic, model.CRMObjectSupportConversation:
		return true
	default:
		return false
	}
}
