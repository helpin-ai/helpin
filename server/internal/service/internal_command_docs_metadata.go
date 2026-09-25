package service

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type docsMetadataCommandInput struct {
	DocumentID   string    `json:"document_id"`
	Title        *string   `json:"title"`
	OwnerID      *string   `json:"owner_id"`
	ClearOwner   bool      `json:"clear_owner"`
	Excerpt      *string   `json:"excerpt"`
	ClearExcerpt bool      `json:"clear_excerpt"`
	Icon         *string   `json:"icon"`
	ClearIcon    bool      `json:"clear_icon"`
	Tags         *[]string `json:"tags"`
	IsPinned     *bool     `json:"is_pinned"`
}

func (s *InternalCommandService) registerDocsMetadataCommands() {
	s.register(InternalCommandDefinition{
		Name: "docs.update_document_metadata", Module: "docs", Mutating: true,
		SupportedTargetTypes: []string{"workspace", "document", "epic", "task", "repository", "support_coverage_gap"},
		Tool:                 mustCommandToolMetadata("docs.update_document_metadata"), Execute: s.executeDocsUpdateDocumentMetadata,
	})
}

func (s *InternalCommandService) executeDocsUpdateDocumentMetadata(ctx context.Context, meta model.InternalCommandContext, input json.RawMessage) (json.RawMessage, error) {
	var req docsMetadataCommandInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("parse document metadata input: %w", err)
	}
	documentID, err := resolveCommandEntityID(meta, req.DocumentID, "document")
	if err != nil {
		return nil, err
	}
	if s.docsDocumentService == nil || s.docsSpaceService == nil {
		return nil, fmt.Errorf("docs metadata services are not configured")
	}
	document, err := s.docsDocumentService.Get(ctx, documentID)
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
	if req.Title != nil {
		trimmed := strings.TrimSpace(html.UnescapeString(*req.Title))
		if trimmed == "" {
			return nil, errCommandInput("title must not be empty")
		}
		req.Title = &trimmed
	}
	if req.ClearOwner && req.OwnerID != nil {
		return nil, fmt.Errorf("clear_owner cannot be combined with owner_id")
	}
	if req.ClearExcerpt && req.Excerpt != nil {
		return nil, fmt.Errorf("clear_excerpt cannot be combined with excerpt")
	}
	if req.ClearIcon && req.Icon != nil {
		return nil, fmt.Errorf("clear_icon cannot be combined with icon")
	}
	ownerID := normalizeOptionalCommandString(req.OwnerID)
	if ownerID != nil {
		if s.workspaceRepo == nil {
			return nil, fmt.Errorf("workspace membership service is not configured")
		}
		membership, err := s.workspaceRepo.GetMembership(ctx, meta.WorkspaceID, *ownerID)
		if err != nil {
			return nil, err
		}
		if membership == nil || membership.Status != model.WorkspaceMemberStatusActive {
			return nil, fmt.Errorf("owner_id is not an active workspace user")
		}
	}
	if req.ClearOwner {
		empty := ""
		ownerID = &empty
	}
	icon := req.Icon
	if req.ClearIcon {
		empty := ""
		icon = &empty
	}
	var tags []string
	if req.Tags != nil {
		tags = normalizeDocsMetadataTags(*req.Tags)
	}
	updated, err := s.docsDocumentService.Update(ctx, document.ID, model.UpdateDocsDocumentRequest{
		Title: req.Title, OwnerID: ownerID, Excerpt: req.Excerpt, ClearExcerpt: req.ClearExcerpt,
		Icon: icon, Tags: tags, IsPinned: req.IsPinned,
	})
	if err != nil {
		return nil, err
	}
	return mustJSON(map[string]any{
		"document_id": updated.ID, "title": updated.Title, "owner_id": updated.OwnerID,
		"excerpt": updated.Excerpt, "icon": updated.Icon, "tags": updated.Tags, "is_pinned": updated.IsPinned,
	}), nil
}

func normalizeDocsMetadataTags(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func commandDocsActor(meta model.InternalCommandContext) *authorization.Actor {
	if strings.TrimSpace(meta.ActorRole) != "" {
		return internalCommandActor(meta)
	}
	if meta.AgentScopeResolved && len(meta.AgentTeamIDs) > 0 {
		memberships := make([]authorization.TeamRole, 0, len(meta.AgentTeamIDs))
		for _, teamID := range normalizeCommandTeamIDs(meta.AgentTeamIDs) {
			memberships = append(memberships, authorization.TeamRole{TeamID: teamID})
		}
		return &authorization.Actor{WorkspaceID: strings.TrimSpace(meta.WorkspaceID), Role: model.RoleMember, TeamMemberships: memberships}
	}
	return nil
}
