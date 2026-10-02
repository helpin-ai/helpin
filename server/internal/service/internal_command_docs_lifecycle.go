package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerDocsLifecycleCommands() {
	for _, name := range []string{"docs.archive_document", "docs.restore_document"} {
		s.register(InternalCommandDefinition{
			Name: name, Module: "docs", Mutating: true,
			SupportedTargetTypes: []string{"workspace", "document", "epic", "task", "repository", "support_conversation", "support_coverage_gap"},
			Tool:                 mustCommandToolMetadata(name),
			Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
				return s.executeDocsLifecycle(ctx, meta, input, name == "docs.archive_document")
			},
		})
	}
}

func (s *InternalCommandService) executeDocsLifecycle(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage, archive bool) (json.RawMessage, error) {
	var req struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse document lifecycle input: %w", err)
	}
	id, err := resolveCommandEntityID(meta, req.DocumentID, "document")
	if err != nil {
		return nil, err
	}
	if s.docsDocumentService == nil || s.docsSpaceService == nil {
		return nil, fmt.Errorf("docs services are not configured")
	}
	document, err := s.docsDocumentService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if document == nil || document.WorkspaceID != meta.WorkspaceID {
		return nil, errCommandNotFound("document")
	}
	space, err := s.docsSpaceService.Get(ctx, document.SpaceID, commandDocsActor(meta))
	if err != nil {
		return nil, err
	}
	if space == nil || space.WorkspaceID != meta.WorkspaceID {
		return nil, errCommandNotFound("document")
	}
	if err := checkLocked(document); err != nil {
		return nil, err
	}
	var updated *model.DocsDocument
	if archive {
		// Publication is independent of draft status: edits can coexist with a live article.
		helpcenter := s.docsDocumentService.helpcenterSvc
		if helpcenter == nil && space.Type == model.SpaceTypeExternalCapable {
			return nil, fmt.Errorf("help center service is not configured; cannot verify publication state")
		}
		if helpcenter != nil {
			article, err := helpcenter.GetArticle(ctx, id)
			if err != nil {
				return nil, err
			}
			if article != nil && article.PublicPublishedAt != nil {
				return nil, fmt.Errorf("document is live in the Help Center; unpublish it in Helpin before archiving")
			}
		}
		updated, err = s.docsDocumentService.Archive(ctx, id)
	} else {
		updated, err = s.docsDocumentService.Unarchive(ctx, id)
	}
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{"document_id": updated.ID, "title": updated.Title, "status": updated.Status}), nil
}
