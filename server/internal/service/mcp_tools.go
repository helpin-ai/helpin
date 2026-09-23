package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/iconcatalog"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// MCPToolResult is the normalized, bounded public tool result envelope.
type MCPToolResult struct {
	Summary string            `json:"summary"`
	Data    any               `json:"data"`
	Links   map[string]string `json:"links,omitempty"`
}

// ExecuteTool repeats authorization and executes one curated public tool.
func (s *MCPService) ExecuteTool(ctx context.Context, principal *model.MCPPrincipal, name string, arguments json.RawMessage) (*MCPToolResult, error) {
	started := time.Now()
	effective, actor, err := s.resolveEffectivePrincipal(ctx, principal)
	if err != nil {
		s.audit(ctx, principal, "tool.call", name, "denied", mcpReasonCode(err), arguments, nil)
		return nil, err
	}
	tool, ok := s.tools[name]
	if !ok || !s.canUseTool(ctx, effective, actor, tool) {
		s.audit(ctx, effective, "tool.call", name, "denied", "tool_not_allowed", arguments, nil)
		return nil, ErrMCPForbidden
	}
	if err := s.validateMCPToolArguments(tool.Name, arguments); err != nil {
		s.audit(ctx, effective, "tool.call", name, "error", "invalid_arguments", arguments, nil)
		return nil, err
	}
	cleanArguments, idempotencyKey, err := prepareMCPArguments(arguments, tool.Mutating)
	if err != nil {
		s.audit(ctx, effective, "tool.call", name, "error", "invalid_arguments", arguments, nil)
		return nil, err
	}
	requestHash := mcpHashBytes(append([]byte(name+":"), cleanArguments...))
	principalKey := mcpPrincipalKey(effective)
	if tool.Mutating {
		existing, err := s.repo.GetIdempotencyRecord(ctx, principalKey, idempotencyKey, time.Now())
		if err != nil {
			return nil, err
		}
		if existing != nil {
			if existing.ToolName != name || existing.RequestHash != requestHash {
				return nil, ErrMCPConflict
			}
			var replay MCPToolResult
			if err := json.Unmarshal(existing.Result, &replay); err != nil {
				return nil, fmt.Errorf("decode MCP idempotent result: %w", err)
			}
			return &replay, nil
		}
	}

	executionContext := authorization.WithActor(ctx, actor)
	result, err := s.executeAuthorizedMCPTool(executionContext, effective, actor, tool, cleanArguments)
	if err != nil {
		s.audit(ctx, effective, "tool.call", name, "error", mcpReasonCode(err), cleanArguments, nil)
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode MCP tool result: %w", err)
	}
	if len(encoded) > _mcpResultMaxBytes {
		s.audit(ctx, effective, "tool.call", name, "error", "result_too_large", cleanArguments, nil)
		return nil, fmt.Errorf("tool result exceeds %d bytes; narrow the request", _mcpResultMaxBytes)
	}
	if tool.Mutating {
		record := &model.MCPIdempotencyRecord{
			ID: uuid.NewString(), PrincipalKey: principalKey, Key: idempotencyKey,
			ToolName: name, RequestHash: requestHash, Result: encoded,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := s.repo.CreateIdempotencyRecord(ctx, record); err != nil {
			existing, getErr := s.repo.GetIdempotencyRecord(ctx, principalKey, idempotencyKey, time.Now())
			if getErr != nil || existing == nil || existing.RequestHash != requestHash || existing.ToolName != name {
				return nil, ErrMCPConflict
			}
		}
	}
	s.auditToolSuccess(ctx, effective, name, cleanArguments, encoded, time.Since(started))
	return result, nil
}

func (s *MCPService) validateMCPToolArguments(toolName string, arguments json.RawMessage) error {
	resolved := s.toolSchemas[toolName]
	if resolved == nil {
		return fmt.Errorf("%w: tool schema is unavailable", ErrMCPInvalidArguments)
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage("{}")
	}
	var value any
	if err := json.Unmarshal(arguments, &value); err != nil {
		return fmt.Errorf("%w: arguments must be JSON", ErrMCPInvalidArguments)
	}
	if err := resolved.Validate(value); err != nil {
		return fmt.Errorf("%w: input does not match the published schema", ErrMCPInvalidArguments)
	}
	return nil
}

func (s *MCPService) executeAuthorizedMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	tool MCPToolDefinition,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	if tool.CommandName != "" {
		if err := s.requireMCPCommandDocumentAccess(ctx, principal, actor, tool, arguments); err != nil {
			return nil, err
		}
		if tool.Name == "search_workspace" {
			var err error
			arguments, err = s.prepareMCPWorkspaceSearchArguments(principal, actor, arguments)
			if err != nil {
				return nil, err
			}
		}
		var taskContextInput *model.GetTaskContextRequest
		if tool.Name == "get_task_context" {
			var input model.GetTaskContextRequest
			if err := decodeMCPArguments(arguments, &input); err != nil {
				return nil, err
			}
			for _, taskID := range input.TaskIDs {
				task, err := s.accessibleMCPTask(ctx, principal, taskID)
				if err != nil {
					return nil, err
				}
				if task == nil {
					return nil, ErrMCPNotFound
				}
			}
			for _, taskKey := range input.TaskKeys {
				taskID, err := s.commands.resolveCommandTaskKey(ctx, model.InternalCommandContext{
					WorkspaceID: principal.WorkspaceID,
					ActorID:     principal.UserID,
					ActorRole:   actor.Role,
				}, taskKey)
				if err != nil {
					return nil, err
				}
				task, err := s.accessibleMCPTask(ctx, principal, taskID)
				if err != nil {
					return nil, err
				}
				if task == nil {
					return nil, ErrMCPNotFound
				}
			}
			if input.IncludeLinkedDocs {
				if !containsMCPValue(principal.Toolsets, MCPToolsetDocs) || !containsMCPValue(principal.Scopes, MCPScopeDocsRead) ||
					!s.authz.Can(actor, authorization.PermDocsRead) {
					return nil, ErrMCPForbidden
				}
				allowed, err := s.authz.CanAccessModule(ctx, actor, model.ModuleDocs)
				if err != nil || !allowed {
					return nil, ErrMCPForbidden
				}
			}
			if input.IncludeGitLinks && (!containsMCPValue(principal.Toolsets, MCPToolsetContext) ||
				!containsMCPValue(principal.Scopes, MCPScopeContextRead) || !s.authz.Can(actor, authorization.PermIntegrationsEnumerate)) {
				return nil, ErrMCPForbidden
			}
			taskContextInput = &input
		}
		output, err := s.commands.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: principal.WorkspaceID,
			ActorID:     principal.UserID,
			ActorRole:   actor.Role,
		}, tool.CommandName, arguments)
		if err != nil {
			return nil, err
		}
		if taskContextInput != nil {
			var data model.GetTaskContextResult
			if err := json.Unmarshal(output, &data); err != nil {
				return nil, fmt.Errorf("decode task context command output: %w", err)
			}
			if taskContextInput.IncludeLinkedDocs {
				for taskIndex := range data.Tasks {
					visible := make([]model.TaskContextDocument, 0, len(data.Tasks[taskIndex].LinkedDocs))
					for _, linkedDocument := range data.Tasks[taskIndex].LinkedDocs {
						document, err := s.accessibleMCPDocument(ctx, principal, actor, linkedDocument.DocumentID)
						if err != nil {
							return nil, err
						}
						if document != nil {
							visible = append(visible, linkedDocument)
						}
					}
					data.Tasks[taskIndex].LinkedDocs = visible
				}
			}
			return &MCPToolResult{Summary: tool.Title + " completed.", Data: data}, nil
		}
		var data any
		if err := json.Unmarshal(output, &data); err != nil {
			return nil, fmt.Errorf("decode command output: %w", err)
		}
		return &MCPToolResult{Summary: tool.Title + " completed.", Data: data}, nil
	}
	return s.executeSpecialMCPTool(ctx, principal, actor, tool.Name, arguments)
}

func (s *MCPService) prepareMCPWorkspaceSearchArguments(principal *model.MCPPrincipal, actor *authorization.Actor, arguments json.RawMessage) (json.RawMessage, error) {
	var input workspaceSearchInput
	if err := decodeMCPArguments(arguments, &input); err != nil {
		return nil, err
	}
	allowed := map[string]bool{
		"task": true, "epic": true, "sprint": true, "objective": true,
		"document": true, "workspace_member": true,
	}
	if containsMCPValue(principal.Toolsets, MCPToolsetCRM) && containsMCPValue(principal.Scopes, MCPScopeCRMRead) && s.authz.Can(actor, authorization.PermCRMRead) {
		allowed["crm_contact"], allowed["crm_company"], allowed["crm_deal"] = true, true, true
	}
	if containsMCPValue(principal.Toolsets, MCPToolsetSupport) && containsMCPValue(principal.Scopes, MCPScopeSupportRead) && s.authz.Can(actor, authorization.PermSupportRead) {
		allowed["support_conversation"] = true
	}
	if len(input.EntityTypes) == 0 {
		input.EntityTypes = orderedAllowedWorkspaceSearchTypes(allowed)
	} else {
		for _, entityType := range input.EntityTypes {
			if !allowed[strings.ToLower(strings.TrimSpace(entityType))] {
				return nil, ErrMCPForbidden
			}
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode workspace search arguments: %w", err)
	}
	return encoded, nil
}

func (s *MCPService) executeSpecialMCPTool(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	name string,
	arguments json.RawMessage,
) (*MCPToolResult, error) {
	switch name {
	case "get_current_context":
		workspace, err := s.workspaceRepo.GetByID(ctx, principal.WorkspaceID)
		if err != nil || workspace == nil {
			return nil, fmt.Errorf("get connected workspace: %w", err)
		}
		user, err := s.userRepo.GetByID(ctx, principal.UserID)
		if err != nil || user == nil {
			return nil, fmt.Errorf("get connected user: %w", err)
		}
		modules, err := s.authz.AccessibleModules(ctx, actor)
		if err != nil {
			return nil, err
		}
		data := map[string]any{
			"principal_kind": principal.Kind,
			"workspace":      map[string]any{"id": workspace.ID, "name": workspace.Name, "slug": workspace.Slug},
			"actor":          map[string]any{"id": user.ID, "name": user.FullName, "email": user.Email, "role": actor.Role},
			"client":         map[string]any{"id": principal.ClientID, "name": principal.ClientName},
			"scopes":         principal.Scopes, "toolsets": principal.Toolsets, "read_only": principal.ReadOnly,
			"modules": modules,
		}
		return &MCPToolResult{Summary: "Connected to " + workspace.Name + " as " + user.FullName + ".", Data: data, Links: map[string]string{"workspace": s.config.AppBaseURL + "/w/" + workspace.Slug}}, nil

	case "search_icons":
		var input struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		items := iconcatalog.Search(input.Query, input.Limit)
		return &MCPToolResult{
			Summary: fmt.Sprintf("Returned %d canonical icons.", len(items)),
			Data: map[string]any{
				"items":           items,
				"total":           len(items),
				"catalog_version": iconcatalog.Version(),
			},
		}, nil

	case "search_workspace":
		var input struct {
			Query string `json:"query"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Query) == "" {
			return nil, fmt.Errorf("query is required")
		}
		result, err := s.search.Search(ctx, principal.WorkspaceID, strings.TrimSpace(input.Query))
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Workspace search completed.", Data: result}, nil

	case "get_task":
		var input struct {
			TaskID string `json:"task_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		task, err := s.accessibleMCPTask(ctx, principal, input.TaskID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, ErrMCPNotFound
		}
		return &MCPToolResult{Summary: "Task " + task.Task.Name + " loaded.", Data: task}, nil

	case "update_task":
		var input struct {
			TaskID string `json:"task_id"`
			model.UpdateTaskRequest
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		current, err := s.accessibleMCPTask(ctx, principal, input.TaskID)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, ErrMCPNotFound
		}
		updated, err := s.tasks.Update(ctx, input.TaskID, input.UpdateTaskRequest, principal.UserID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Task " + updated.Task.Name + " updated.", Data: updated}, nil

	case "create_task_batch":
		var input struct {
			EpicID string               `json:"epic_id"`
			Tasks  []model.ProposedTask `json:"tasks"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		tasks, err := s.taskBatches.CreateEpicTaskBatch(ctx, principal.WorkspaceID, input.EpicID, principal.UserID, input.Tasks)
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(tasks))
		for index, task := range tasks {
			items = append(items, map[string]any{"ref": input.Tasks[index].Ref, "task_id": task.ID, "name": task.Name})
		}
		return &MCPToolResult{Summary: fmt.Sprintf("Created %d tasks.", len(items)), Data: map[string]any{"items": items, "total": len(items)}}, nil

	case "list_task_checklist":
		var input struct {
			TaskID string `json:"task_id"`
			Limit  int    `json:"limit"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		task, err := s.accessibleMCPTask(ctx, principal, input.TaskID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, ErrMCPNotFound
		}
		items, total, hasMore, err := s.checklists.ListBounded(ctx, input.TaskID, principal.WorkspaceID, input.Limit)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: fmt.Sprintf("Returned %d checklist items.", len(items)), Data: map[string]any{"task_id": input.TaskID, "items": items, "total": total, "has_more": hasMore}}, nil

	case "create_task_checklist_item":
		var input struct {
			TaskID   string  `json:"task_id"`
			Text     string  `json:"text"`
			Position *int    `json:"position"`
			DueDate  *string `json:"due_date"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		task, err := s.accessibleMCPTask(ctx, principal, input.TaskID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, ErrMCPNotFound
		}
		var dueDate *time.Time
		if input.DueDate != nil {
			dueDate, err = parseStrictPMCommandDate(*input.DueDate, "due_date", false)
			if err != nil {
				return nil, err
			}
		}
		item, err := s.checklists.Create(ctx, input.TaskID, model.CreateChecklistItemRequest{Text: input.Text, Position: input.Position, DueDate: dueDate}, principal.WorkspaceID, principal.UserID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Checklist item created.", Data: item}, nil

	case "update_task_checklist_item":
		var input struct {
			TaskID          string          `json:"task_id"`
			ChecklistItemID string          `json:"checklist_item_id"`
			Text            *string         `json:"text"`
			Completed       *bool           `json:"completed"`
			Position        *int            `json:"position"`
			DueDate         json.RawMessage `json:"due_date"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		task, err := s.accessibleMCPTask(ctx, principal, input.TaskID)
		if err != nil {
			return nil, err
		}
		if task == nil {
			return nil, ErrMCPNotFound
		}
		existing, err := s.checklists.Get(ctx, input.ChecklistItemID, principal.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if existing == nil || existing.TaskID != input.TaskID {
			return nil, ErrMCPNotFound
		}
		update := model.UpdateChecklistItemRequest{Text: input.Text, Completed: input.Completed, Position: input.Position}
		if input.DueDate != nil {
			update.DueDateSet = true
			if !bytes.Equal(bytes.TrimSpace(input.DueDate), []byte("null")) {
				var value string
				if err := json.Unmarshal(input.DueDate, &value); err != nil {
					return nil, fmt.Errorf("due_date must be YYYY-MM-DD or null")
				}
				update.DueDate, err = parseStrictPMCommandDate(value, "due_date", false)
				if err != nil {
					return nil, err
				}
			}
		}
		item, err := s.checklists.Update(ctx, input.ChecklistItemID, update, principal.WorkspaceID, principal.UserID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Checklist item updated.", Data: item}, nil

	case "list_spaces":
		spaces, err := s.spaces.List(ctx, principal.WorkspaceID, actor)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{
			Summary: fmt.Sprintf("Returned %d Docs spaces.", len(spaces)),
			Data:    map[string]any{"items": spaces, "total": len(spaces)},
		}, nil

	case "list_collections":
		var input struct {
			SpaceID string `json:"space_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		if input.SpaceID != "" {
			space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, input.SpaceID)
			if err != nil {
				return nil, err
			}
			if space == nil {
				return nil, ErrMCPNotFound
			}
			collections, err := s.collections.List(ctx, space.ID)
			if err != nil {
				return nil, err
			}
			return &MCPToolResult{
				Summary: fmt.Sprintf("Returned %d Docs collections.", len(collections)),
				Data:    map[string]any{"items": collections, "total": len(collections)},
			}, nil
		}
		spaces, err := s.spaces.List(ctx, principal.WorkspaceID, actor)
		if err != nil {
			return nil, err
		}
		collections := make([]model.DocsCollection, 0)
		for _, space := range spaces {
			items, err := s.collections.List(ctx, space.ID)
			if err != nil {
				return nil, err
			}
			collections = append(collections, items...)
		}
		return &MCPToolResult{
			Summary: fmt.Sprintf("Returned %d Docs collections.", len(collections)),
			Data:    map[string]any{"items": collections, "total": len(collections)},
		}, nil

	case "get_document":
		var input struct {
			DocumentID string `json:"document_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		document, err := s.accessibleMCPDocument(ctx, principal, actor, input.DocumentID)
		if err != nil {
			return nil, err
		}
		if document == nil {
			return nil, ErrMCPNotFound
		}
		return &MCPToolResult{Summary: "Document " + document.Title + " loaded.", Data: document}, nil

	case "create_space":
		var input model.CreateDocsSpaceRequest
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		input.Name = strings.TrimSpace(input.Name)
		space, err := s.spaces.Create(ctx, principal.WorkspaceID, input, principal.UserID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{
			Summary: "Docs space " + space.Name + " created.",
			Data:    space,
		}, nil

	case "create_collection":
		var input struct {
			SpaceID            string  `json:"space_id"`
			Name               string  `json:"name"`
			Slug               *string `json:"slug"`
			Description        *string `json:"description"`
			Icon               *string `json:"icon"`
			ParentCollectionID *string `json:"parent_collection_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, input.SpaceID)
		if err != nil {
			return nil, err
		}
		if space == nil {
			return nil, ErrMCPNotFound
		}
		collection, err := s.collections.Create(
			ctx,
			principal.WorkspaceID,
			space.ID,
			model.CreateDocsCollectionRequest{
				Name:               strings.TrimSpace(input.Name),
				Slug:               input.Slug,
				Description:        input.Description,
				Icon:               input.Icon,
				ParentCollectionID: input.ParentCollectionID,
			},
			principal.UserID,
		)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{
			Summary: "Docs collection " + collection.Name + " created.",
			Data:    collection,
		}, nil

	case "update_space":
		var input struct {
			SpaceID string `json:"space_id"`
			model.UpdateDocsSpaceRequest
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, input.SpaceID)
		if err != nil {
			return nil, err
		}
		if space == nil {
			return nil, ErrMCPNotFound
		}
		updated, err := s.spaces.Update(ctx, space.ID, input.UpdateDocsSpaceRequest)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Docs space " + updated.Name + " updated.", Data: updated}, nil

	case "update_collection":
		var input struct {
			CollectionID string `json:"collection_id"`
			model.UpdateDocsCollectionRequest
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		collection, err := s.accessibleMCPDocsCollection(ctx, principal, actor, input.CollectionID)
		if err != nil {
			return nil, err
		}
		if collection == nil {
			return nil, ErrMCPNotFound
		}
		if input.ParentCollectionID != nil {
			space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, collection.SpaceID)
			if err != nil {
				return nil, err
			}
			if space == nil {
				return nil, ErrMCPNotFound
			}
			if space.Type == model.SpaceTypeExternalCapable {
				return nil, fmt.Errorf("public Help Center collections cannot be reparented through public MCP")
			}
		}
		updated, err := s.collections.Update(ctx, collection.ID, input.UpdateDocsCollectionRequest)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Docs collection " + updated.Name + " updated.", Data: updated}, nil

	case "move_document":
		var input struct {
			DocumentID   string  `json:"document_id"`
			SpaceID      string  `json:"space_id"`
			CollectionID *string `json:"collection_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		document, err := s.accessibleMCPDocument(ctx, principal, actor, input.DocumentID)
		if err != nil {
			return nil, err
		}
		if document == nil {
			return nil, ErrMCPNotFound
		}
		targetSpace, err := s.accessibleMCPDocsSpace(ctx, principal, actor, input.SpaceID)
		if err != nil {
			return nil, err
		}
		if targetSpace == nil {
			return nil, ErrMCPNotFound
		}
		if document.Status == model.DocStatusPublished && document.SpaceID != targetSpace.ID {
			return nil, fmt.Errorf("published documents cannot be moved between spaces through public MCP")
		}
		if input.CollectionID != nil && strings.TrimSpace(*input.CollectionID) != "" {
			collection, err := s.accessibleMCPDocsCollection(ctx, principal, actor, *input.CollectionID)
			if err != nil {
				return nil, err
			}
			if collection == nil || collection.SpaceID != targetSpace.ID {
				return nil, ErrMCPNotFound
			}
		}
		updated, err := s.documents.Move(ctx, document.ID, model.MoveDocsDocumentRequest{SpaceID: targetSpace.ID, CollectionID: input.CollectionID})
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Document " + updated.Title + " moved.", Data: updated}, nil

	case "link_document_to_object":
		var input struct {
			DocumentID       string `json:"document_id"`
			LinkedObjectType string `json:"linked_object_type"`
			LinkedObjectID   string `json:"linked_object_id"`
			LinkContext      string `json:"link_context"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		document, err := s.accessibleMCPDocument(ctx, principal, actor, input.DocumentID)
		if err != nil {
			return nil, err
		}
		if document == nil {
			return nil, ErrMCPNotFound
		}
		switch input.LinkedObjectType {
		case model.LinkedObjectEpic, model.LinkedObjectTask:
			if !containsMCPValue(principal.Toolsets, MCPToolsetPM) || !containsMCPValue(principal.Scopes, MCPScopePMRead) {
				return nil, ErrMCPForbidden
			}
		case model.LinkedObjectSupportConversation:
			if !containsMCPValue(principal.Toolsets, MCPToolsetSupport) || !containsMCPValue(principal.Scopes, MCPScopeSupportRead) {
				return nil, ErrMCPForbidden
			}
		case model.LinkedObjectDeal, model.LinkedObjectContact, model.LinkedObjectCompany:
			if !containsMCPValue(principal.Toolsets, MCPToolsetCRM) || !containsMCPValue(principal.Scopes, MCPScopeCRMRead) {
				return nil, ErrMCPForbidden
			}
		default:
			return nil, ErrMCPInvalidArguments
		}
		resolved, err := s.docsRefs.Resolve(ctx, principal.WorkspaceID, model.ResolveDocsEntityRefsRequest{Refs: []model.DocsEntityRefRequest{{EntityType: input.LinkedObjectType, EntityID: input.LinkedObjectID}}})
		if err != nil {
			return nil, err
		}
		if len(resolved.Refs) != 1 || resolved.Refs[0].Status != docsEntityRefStatusAvailable {
			if len(resolved.Refs) == 1 && resolved.Refs[0].Access == docsEntityRefAccessRedacted {
				return nil, ErrMCPForbidden
			}
			return nil, ErrMCPNotFound
		}
		if input.LinkContext == "" {
			input.LinkContext = model.LinkContextAttached
		}
		link, err := s.docsLinks.Create(ctx, principal.WorkspaceID, document.ID, model.CreateDocsLinkRequest{LinkedObjectType: input.LinkedObjectType, LinkedObjectID: input.LinkedObjectID, LinkContext: input.LinkContext}, principal.UserID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Document linked.", Data: link}, nil

	case "get_crm_contact":
		var input struct {
			ContactID string `json:"contact_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		contact, err := s.crmContacts.GetByID(ctx, input.ContactID)
		if err != nil {
			return nil, err
		}
		if contact.WorkspaceID != principal.WorkspaceID {
			return nil, ErrMCPNotFound
		}
		return &MCPToolResult{Summary: "CRM contact loaded.", Data: contact}, nil

	case "get_crm_deal":
		var input struct {
			DealID string `json:"deal_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		deal, err := s.crmDeals.GetByID(ctx, input.DealID)
		if err != nil {
			return nil, err
		}
		if deal.WorkspaceID != principal.WorkspaceID {
			return nil, ErrMCPNotFound
		}
		return &MCPToolResult{Summary: "CRM deal " + deal.Name + " loaded.", Data: deal}, nil

	case "list_support_conversations":
		var input struct {
			Status string `json:"status"`
			Search string `json:"search"`
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		input.Limit = normalizeMCPLimit(input.Limit)
		if input.Offset < 0 {
			return nil, fmt.Errorf("offset must be zero or greater")
		}
		items, total, err := s.support.ListConversations(ctx, principal.WorkspaceID, input.Status, "", model.PMPagination{Page: 1, PerPage: input.Limit, Offset: &input.Offset}, input.Search)
		if err != nil {
			return nil, err
		}
		paging := commandPaginationOutput(total, input.Offset, input.Limit, len(items))
		paging["items"] = items
		return &MCPToolResult{Summary: fmt.Sprintf("Returned %d of %d support conversations.", len(items), total), Data: paging}, nil

	case "get_support_conversation":
		var input struct {
			ConversationID string `json:"conversation_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		conversation, err := s.support.GetConversation(ctx, principal.WorkspaceID, input.ConversationID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Support conversation loaded.", Data: conversation}, nil

	case "list_conversation_messages":
		var input struct {
			ConversationID string `json:"conversation_id"`
			Limit          int    `json:"limit"`
			Offset         int    `json:"offset"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		messages, err := s.support.ListConversationMessages(ctx, principal.WorkspaceID, input.ConversationID, false)
		if err != nil {
			return nil, err
		}
		input.Limit = normalizeMCPLimit(input.Limit)
		if input.Offset < 0 {
			return nil, fmt.Errorf("offset must be zero or greater")
		}
		start := min(input.Offset, len(messages))
		end := min(start+input.Limit, len(messages))
		items := messages[start:end]
		paging := commandPaginationOutput(int64(len(messages)), input.Offset, input.Limit, len(items))
		paging["items"] = items
		return &MCPToolResult{Summary: fmt.Sprintf("Returned %d of %d support messages.", len(items), len(messages)), Data: paging}, nil

	case "list_agents":
		agents, err := s.agents.ListAgentsForActor(ctx, principal.WorkspaceID, actor)
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(agents))
		for _, agent := range agents {
			items = append(items, map[string]any{
				"id": agent.ID, "name": agent.Name, "role": agent.Role, "is_system": agent.IsSystem,
				"preset_key": agent.PresetKey, "runtime_kind": agent.RuntimeKind,
				"supported_modes": agent.SupportedModes, "allowed_targets": json.RawMessage(agent.AllowedTargets),
			})
		}
		return &MCPToolResult{Summary: fmt.Sprintf("Returned %d available agents.", len(items)), Data: items}, nil

	case "start_agent_run":
		var input struct {
			AgentID           string  `json:"agent_id"`
			TargetType        string  `json:"target_type"`
			TargetID          string  `json:"target_id"`
			AdditionalContext *string `json:"additional_context"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		if err := s.agents.RequireActorCanUseAgent(ctx, principal.WorkspaceID, input.AgentID, actor); err != nil {
			return nil, ErrMCPForbidden
		}
		recentStarts, err := s.repo.CountRecentToolCalls(ctx, principal.WorkspaceID, principal.UserID, "start_agent_run", time.Now().Add(-time.Hour))
		if err != nil {
			return nil, err
		}
		if recentStarts >= 10 {
			return nil, ErrMCPRateLimited
		}
		userActive, workspaceActive, err := s.repo.CountActiveMCPAgentRuns(ctx, principal.WorkspaceID, principal.UserID)
		if err != nil {
			return nil, err
		}
		if userActive >= 3 || workspaceActive >= 10 {
			return nil, ErrMCPRateLimited
		}
		run, err := s.agents.StartTargetRun(ctx, principal.WorkspaceID, input.TargetType, input.TargetID, model.StartAgentRunRequest{AgentID: input.AgentID, AdditionalContext: input.AdditionalContext}, principal.UserID)
		if err != nil {
			return nil, err
		}
		if !mcpPrincipalCanSeeRun(principal, run) {
			return nil, newMCPToolError(MCPErrorCodeRunNotOwned,
				"Another private run is already active for this agent and target. Wait for it to finish or choose another target.")
		}
		attribution := &model.MCPAgentRunAttribution{RunID: run.ID, WorkspaceID: principal.WorkspaceID, ClientName: principal.ClientName}
		if principal.ConnectionID != "" {
			attribution.ConnectionID = &principal.ConnectionID
		}
		if principal.ServicePrincipalID != "" {
			attribution.ServicePrincipalID = &principal.ServicePrincipalID
		}
		if err := s.repo.CreateRunAttribution(ctx, attribution); err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Agent run started. Poll get_agent_run with the returned run_id.", Data: map[string]any{"run_id": run.ID, "status": run.Status, "agent_id": run.AgentID, "target_type": run.TargetType, "target_id": run.TargetID}}, nil

	case "get_agent_run":
		var input struct {
			RunID string `json:"run_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		run, err := s.agents.GetAgentRun(ctx, principal.WorkspaceID, input.RunID)
		if err != nil {
			return nil, err
		}
		if !mcpPrincipalCanSeeRun(principal, run) {
			return nil, ErrMCPNotFound
		}
		artifacts, err := s.agents.ListRunArtifacts(ctx, principal.WorkspaceID, input.RunID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Agent run is " + run.Status + ".", Data: map[string]any{"run": run, "artifacts": artifacts}}, nil

	case "cancel_agent_run":
		var input struct {
			RunID string `json:"run_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		existing, err := s.agents.GetAgentRun(ctx, principal.WorkspaceID, input.RunID)
		if err != nil {
			return nil, err
		}
		if !mcpPrincipalCanSeeRun(principal, existing) {
			return nil, ErrMCPNotFound
		}
		run, err := s.agents.CancelRun(ctx, principal.WorkspaceID, input.RunID, principal.UserID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Agent run cancellation requested.", Data: map[string]any{"run_id": run.ID, "status": run.Status}}, nil
	default:
		if result, handled, err := s.executeDocsLifecycleMCPTool(ctx, principal, actor, name, arguments); handled {
			return result, err
		}
		if result, handled, err := s.executeUploadMCPTool(ctx, principal, actor, name, arguments); handled {
			return result, err
		}
		if result, handled, err := s.executeDocsBatchMCPTool(ctx, principal, actor, name, arguments); handled {
			return result, err
		}
		if result, handled, err := s.executeHelpcenterMCPTool(ctx, principal, actor, name, arguments); handled {
			return result, err
		}
		return nil, ErrMCPNotFound
	}
}

func prepareMCPArguments(arguments json.RawMessage, mutation bool) (json.RawMessage, string, error) {
	if len(arguments) == 0 {
		arguments = json.RawMessage("{}")
	}
	var values map[string]any
	if err := json.Unmarshal(arguments, &values); err != nil {
		return nil, "", fmt.Errorf("arguments must be a JSON object")
	}
	var key string
	if mutation {
		key, _ = values["idempotency_key"].(string)
		key = strings.TrimSpace(key)
		if len(key) < 8 || len(key) > 128 {
			return nil, "", fmt.Errorf("idempotency_key must be between 8 and 128 characters")
		}
		delete(values, "idempotency_key")
	}
	clean, err := json.Marshal(values)
	if err != nil {
		return nil, "", fmt.Errorf("encode MCP arguments: %w", err)
	}
	return clean, key, nil
}

func decodeMCPArguments(arguments json.RawMessage, output any) error {
	if err := json.Unmarshal(arguments, output); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	return nil
}

func normalizeMCPLimit(limit int) int {
	if limit <= 0 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (s *MCPService) accessibleMCPDocsSpace(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	spaceID string,
) (*model.DocsSpaceWithTeams, error) {
	space, err := s.spaces.Get(ctx, spaceID, actor)
	if err != nil {
		return nil, err
	}
	if space == nil || space.WorkspaceID != principal.WorkspaceID {
		return nil, nil
	}
	return space, nil
}

func (s *MCPService) accessibleMCPDocsCollection(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	collectionID string,
) (*model.DocsCollection, error) {
	collection, err := s.collections.Get(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	if collection == nil || collection.WorkspaceID != principal.WorkspaceID {
		return nil, nil
	}
	space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, collection.SpaceID)
	if err != nil || space == nil {
		return nil, err
	}
	return collection, nil
}

func (s *MCPService) accessibleMCPDocument(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	documentID string,
) (*model.DocsDocument, error) {
	document, err := s.documents.Get(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if document == nil || document.WorkspaceID != principal.WorkspaceID {
		return nil, nil
	}
	space, err := s.accessibleMCPDocsSpace(ctx, principal, actor, document.SpaceID)
	if err != nil || space == nil {
		return nil, err
	}
	return document, nil
}

func (s *MCPService) accessibleMCPTask(
	ctx context.Context,
	principal *model.MCPPrincipal,
	taskID string,
) (*model.TaskDetail, error) {
	task, err := s.tasks.GetByID(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}
	if task == nil || task.Task.WorkspaceID != principal.WorkspaceID {
		return nil, nil
	}
	return task, nil
}

// requireMCPCommandDocumentAccess applies Docs space access to command-backed
// document tools, which otherwise only verify the workspace.
func (s *MCPService) requireMCPCommandDocumentAccess(
	ctx context.Context,
	principal *model.MCPPrincipal,
	actor *authorization.Actor,
	tool MCPToolDefinition,
	arguments json.RawMessage,
) error {
	if tool.Toolset != MCPToolsetDocs {
		return nil
	}
	var input struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(arguments, &input); err != nil || strings.TrimSpace(input.DocumentID) == "" {
		return nil
	}
	document, err := s.accessibleMCPDocument(ctx, principal, actor, strings.TrimSpace(input.DocumentID))
	if err != nil {
		return err
	}
	if document == nil {
		return ErrMCPNotFound
	}
	return nil
}

// mcpPrincipalCanSeeRun hides dock chat runs, which are private to the user
// who owns the chat, from every other MCP principal.
func mcpPrincipalCanSeeRun(principal *model.MCPPrincipal, run *model.AgentRun) bool {
	if run == nil {
		return false
	}
	if run.DockChatID == nil {
		return true
	}
	return run.TriggeredByUserID != nil && *run.TriggeredByUserID == principal.UserID
}

func mcpPrincipalKey(principal *model.MCPPrincipal) string {
	if principal.ConnectionID != "" {
		return "connection:" + principal.ConnectionID
	}
	return "service:" + principal.ServicePrincipalID
}

func mcpReasonCode(err error) string {
	var toolErr *MCPToolError
	if errors.As(err, &toolErr) {
		return strings.ToLower(toolErr.Code)
	}
	switch {
	case errors.Is(err, ErrMCPUnauthorized):
		return "unauthorized"
	case errors.Is(err, ErrMCPForbidden):
		return "forbidden"
	case errors.Is(err, ErrMCPDisabled):
		return "workspace_disabled"
	case errors.Is(err, ErrMCPNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		return "not_found"
	case errors.Is(err, ErrMCPConflict):
		return "idempotency_conflict"
	case errors.Is(err, ErrMCPInvalidArguments):
		return "invalid_arguments"
	case errors.Is(err, ErrMCPRateLimited):
		return "rate_limited"
	default:
		return "execution_error"
	}
}

func (s *MCPService) auditToolSuccess(ctx context.Context, principal *model.MCPPrincipal, tool string, request, result []byte, duration time.Duration) {
	event := &model.MCPAuditEvent{
		ID: uuid.NewString(), WorkspaceID: principal.WorkspaceID, ClientName: principal.ClientName,
		EventType: "tool.call", ToolName: &tool, Outcome: "success",
		RequestHash: mcpStringPointer(mcpHashBytes(request)), ResultHash: mcpStringPointer(mcpHashBytes(result)),
		DurationMS: mcpInt64Pointer(duration.Milliseconds()),
	}
	if principal.ConnectionID != "" {
		event.ConnectionID = &principal.ConnectionID
	}
	if principal.ServicePrincipalID != "" {
		event.ServicePrincipalID = &principal.ServicePrincipalID
	}
	if principal.UserID != "" {
		event.UserID = &principal.UserID
	}
	_ = s.repo.CreateAuditEvent(ctx, event)
}

func mcpStringPointer(value string) *string { return &value }

func mcpInt64Pointer(value int64) *int64 { return &value }
