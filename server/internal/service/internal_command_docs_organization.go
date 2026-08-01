package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *InternalCommandService) registerDocsOrganizationCommands() {
	s.register(InternalCommandDefinition{
		Name:                 "docs.create_space",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "epic", "task", "repository"},
		Tool:                 mustCommandToolMetadata("docs.create_space"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.docsSpaceService == nil {
				return nil, fmt.Errorf("docs space service is not available")
			}
			var req model.CreateDocsSpaceRequest
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse create space input: %w", err)
			}
			req.Name = strings.TrimSpace(req.Name)
			space, err := s.docsSpaceService.Create(ctx, meta.WorkspaceID, req, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(space), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "docs.create_collection",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "epic", "task", "repository"},
		Tool:                 mustCommandToolMetadata("docs.create_collection"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.docsSpaceService == nil || s.docsCollectionService == nil {
				return nil, fmt.Errorf("docs organization services are not available")
			}
			var req struct {
				SpaceID            string  `json:"space_id"`
				Name               string  `json:"name"`
				Slug               *string `json:"slug"`
				Description        *string `json:"description"`
				Icon               *string `json:"icon"`
				ParentCollectionID *string `json:"parent_collection_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse create collection input: %w", err)
			}
			space, err := s.docsSpaceService.GetUnfiltered(ctx, strings.TrimSpace(req.SpaceID))
			if err != nil {
				return nil, err
			}
			if space == nil || space.WorkspaceID != meta.WorkspaceID {
				return nil, fmt.Errorf("space not found")
			}
			collection, err := s.docsCollectionService.Create(ctx, meta.WorkspaceID, space.ID, model.CreateDocsCollectionRequest{
				Name: strings.TrimSpace(req.Name), Slug: req.Slug, Description: req.Description,
				Icon: req.Icon, ParentCollectionID: req.ParentCollectionID,
			}, fallbackActor(meta))
			if err != nil {
				return nil, err
			}
			return mustJSON(collection), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "docs.update_space",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "epic", "task", "repository"},
		Tool:                 mustCommandToolMetadata("docs.update_space"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.docsSpaceService == nil {
				return nil, fmt.Errorf("docs space service is not available")
			}
			var req struct {
				SpaceID string `json:"space_id"`
				model.UpdateDocsSpaceRequest
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse update space input: %w", err)
			}
			space, err := s.docsSpaceService.GetUnfiltered(ctx, strings.TrimSpace(req.SpaceID))
			if err != nil {
				return nil, err
			}
			if space == nil || space.WorkspaceID != meta.WorkspaceID {
				return nil, fmt.Errorf("space not found")
			}
			updated, err := s.docsSpaceService.Update(ctx, space.ID, req.UpdateDocsSpaceRequest)
			if err != nil {
				return nil, err
			}
			return mustJSON(updated), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "docs.update_collection",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "epic", "task", "repository"},
		Tool:                 mustCommandToolMetadata("docs.update_collection"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.docsCollectionService == nil {
				return nil, fmt.Errorf("docs collection service is not available")
			}
			var req struct {
				CollectionID string `json:"collection_id"`
				model.UpdateDocsCollectionRequest
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse update collection input: %w", err)
			}
			collection, err := s.docsCollectionService.Get(ctx, strings.TrimSpace(req.CollectionID))
			if err != nil {
				return nil, err
			}
			if collection == nil || collection.WorkspaceID != meta.WorkspaceID {
				return nil, fmt.Errorf("collection not found")
			}
			if req.ParentCollectionID != nil {
				if s.docsSpaceService == nil {
					return nil, fmt.Errorf("docs space service is not available")
				}
				space, err := s.docsSpaceService.GetUnfiltered(ctx, collection.SpaceID)
				if err != nil {
					return nil, err
				}
				if space == nil || space.WorkspaceID != meta.WorkspaceID {
					return nil, fmt.Errorf("space not found")
				}
				if space.Type == model.SpaceTypeExternalCapable {
					return nil, fmt.Errorf("public Help Center collections cannot be reparented by an agent")
				}
			}
			updated, err := s.docsCollectionService.Update(ctx, collection.ID, req.UpdateDocsCollectionRequest)
			if err != nil {
				return nil, err
			}
			return mustJSON(updated), nil
		},
	})

	s.register(InternalCommandDefinition{
		Name:                 "docs.move_document",
		Module:               "docs",
		Mutating:             true,
		SupportedTargetTypes: []string{"workspace", "document", "epic", "task", "repository"},
		Tool:                 mustCommandToolMetadata("docs.move_document"),
		Execute: func(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
			if s.docsDocumentService == nil || s.docsSpaceService == nil || s.docsCollectionService == nil {
				return nil, fmt.Errorf("docs organization services are not available")
			}
			var req struct {
				DocumentID   string  `json:"document_id"`
				SpaceID      string  `json:"space_id"`
				CollectionID *string `json:"collection_id"`
			}
			if err := json.Unmarshal(input, &req); err != nil {
				return nil, fmt.Errorf("parse move document input: %w", err)
			}
			document, err := s.docsDocumentService.Get(ctx, strings.TrimSpace(req.DocumentID))
			if err != nil {
				return nil, err
			}
			if document == nil || document.WorkspaceID != meta.WorkspaceID {
				return nil, fmt.Errorf("document not found")
			}
			space, err := s.docsSpaceService.GetUnfiltered(ctx, strings.TrimSpace(req.SpaceID))
			if err != nil {
				return nil, err
			}
			if space == nil || space.WorkspaceID != meta.WorkspaceID {
				return nil, fmt.Errorf("space not found")
			}
			if document.Status == model.DocStatusPublished && document.SpaceID != space.ID {
				return nil, fmt.Errorf("published documents cannot be moved between spaces by an agent")
			}
			if req.CollectionID != nil && strings.TrimSpace(*req.CollectionID) != "" {
				collection, err := s.docsCollectionService.Get(ctx, strings.TrimSpace(*req.CollectionID))
				if err != nil {
					return nil, err
				}
				if collection == nil || collection.WorkspaceID != meta.WorkspaceID || collection.SpaceID != space.ID {
					return nil, fmt.Errorf("collection not found in target space")
				}
			}
			updated, err := s.docsDocumentService.Move(ctx, document.ID, model.MoveDocsDocumentRequest{SpaceID: space.ID, CollectionID: req.CollectionID})
			if err != nil {
				return nil, err
			}
			return mustJSON(updated), nil
		},
	})
}
