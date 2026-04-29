package service

import (
	"context"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
)

// DocsLinkService handles business logic for document links.
type DocsLinkService struct {
	linkRepo    *repository.DocsLinkRepository
	taskRepo    *repository.PMTaskRepository
	docRepo     *repository.DocsDocumentRepository
	wsPublisher *websocket.Publisher
}

// NewDocsLinkService creates a new DocsLinkService.
func NewDocsLinkService(linkRepo *repository.DocsLinkRepository, taskRepo *repository.PMTaskRepository, docRepo *repository.DocsDocumentRepository, wsPublisher *websocket.Publisher) *DocsLinkService {
	return &DocsLinkService{linkRepo: linkRepo, taskRepo: taskRepo, docRepo: docRepo, wsPublisher: wsPublisher}
}

// Create creates a new link between a document and a PM/Support object.
func (s *DocsLinkService) Create(ctx context.Context, workspaceID, documentID string, req model.CreateDocsLinkRequest, userID string) (*model.DocsLink, error) {
	if req.LinkedObjectType == "" || req.LinkedObjectID == "" {
		return nil, fmt.Errorf("linked_object_type and linked_object_id are required")
	}
	if req.LinkContext == "" {
		req.LinkContext = model.LinkContextAttached
	}
	existingLinks, err := s.linkRepo.ListByObject(ctx, workspaceID, req.LinkedObjectType, req.LinkedObjectID)
	if err != nil {
		return nil, err
	}
	for _, existing := range existingLinks {
		if existing.DocumentID == documentID && sameOptionalString(existing.BlockID, req.BlockID) {
			return &existing, nil
		}
	}

	link := &model.DocsLink{
		WorkspaceID:      workspaceID,
		DocumentID:       documentID,
		BlockID:          req.BlockID,
		LinkedObjectType: req.LinkedObjectType,
		LinkedObjectID:   req.LinkedObjectID,
		LinkContext:      req.LinkContext,
		CreatedBy:        userID,
	}
	created, err := s.linkRepo.Create(ctx, link)
	if err == nil && created != nil {
		publishWorkspaceEventWithParent(s.wsPublisher, "created", "docs_link", created.ID, workspaceID, userID, "docs_document", documentID, map[string]any{
			"linked_object_type": created.LinkedObjectType,
			"linked_object_id":   created.LinkedObjectID,
			"link_context":       created.LinkContext,
		})
	}
	return created, err
}

func sameOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ListByDocument returns all links for a document, enriched with object names.
func (s *DocsLinkService) ListByDocument(ctx context.Context, documentID string) ([]model.DocsLink, error) {
	links, err := s.linkRepo.ListByDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	s.enrichLinks(ctx, links)
	return links, nil
}

// ListByObject returns all document links for a PM/Support object (reverse lookup), enriched with document titles.
func (s *DocsLinkService) ListByObject(ctx context.Context, workspaceID, objectType, objectID string) ([]model.DocsLink, error) {
	links, err := s.linkRepo.ListByObject(ctx, workspaceID, objectType, objectID)
	if err != nil {
		return nil, err
	}
	s.enrichDocumentTitles(ctx, links)
	return links, nil
}

// enrichLinks populates transient LinkedObjectName and LinkedObjectDisplayID fields.
func (s *DocsLinkService) enrichLinks(ctx context.Context, links []model.DocsLink) {
	if len(links) == 0 || s.taskRepo == nil {
		return
	}

	// Collect task IDs.
	var storyIDs []string
	for _, l := range links {
		if l.LinkedObjectType == "task" || l.LinkedObjectType == "story" {
			storyIDs = append(storyIDs, l.LinkedObjectID)
		}
	}
	if len(storyIDs) == 0 {
		return
	}

	// Batch-fetch stories. Use the workspace from the first link.
	wsID := links[0].WorkspaceID
	stories, err := s.taskRepo.ListByIDs(ctx, wsID, storyIDs)
	if err != nil {
		return // best-effort enrichment
	}

	storyMap := make(map[string]*model.PMTask, len(stories))
	for i := range stories {
		storyMap[stories[i].ID] = &stories[i]
	}

	for i := range links {
		if links[i].LinkedObjectType == "task" || links[i].LinkedObjectType == "story" {
			if st, ok := storyMap[links[i].LinkedObjectID]; ok {
				links[i].LinkedObjectName = st.Name
				links[i].LinkedObjectDisplayID = st.DisplayID
			}
		}
	}
}

// enrichDocumentTitles populates the transient DocumentTitle field from the docs table.
func (s *DocsLinkService) enrichDocumentTitles(ctx context.Context, links []model.DocsLink) {
	if len(links) == 0 || s.docRepo == nil {
		return
	}

	var docIDs []string
	for _, l := range links {
		docIDs = append(docIDs, l.DocumentID)
	}

	wsID := links[0].WorkspaceID
	docs, err := s.docRepo.ListByIDs(ctx, wsID, docIDs)
	if err != nil {
		return // best-effort
	}

	docMap := make(map[string]string, len(docs))
	for _, d := range docs {
		docMap[d.ID] = d.Title
	}

	for i := range links {
		if title, ok := docMap[links[i].DocumentID]; ok {
			links[i].DocumentTitle = title
		}
	}
}

// Delete removes a link.
func (s *DocsLinkService) Delete(ctx context.Context, id string) error {
	link, err := s.linkRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if link == nil {
		return fmt.Errorf("link not found")
	}
	if err := s.linkRepo.Delete(ctx, id); err != nil {
		return err
	}
	publishWorkspaceEventWithParent(s.wsPublisher, "deleted", "docs_link", id, link.WorkspaceID, "", "docs_document", link.DocumentID, map[string]any{
		"linked_object_type": link.LinkedObjectType,
		"linked_object_id":   link.LinkedObjectID,
		"link_context":       link.LinkContext,
	})
	return nil
}
