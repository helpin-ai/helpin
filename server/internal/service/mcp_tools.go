package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
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
		output, err := s.commands.Execute(ctx, model.InternalCommandContext{
			WorkspaceID: principal.WorkspaceID,
			ActorID:     principal.UserID,
			ActorRole:   actor.Role,
		}, tool.CommandName, arguments)
		if err != nil {
			return nil, err
		}
		var data any
		if err := json.Unmarshal(output, &data); err != nil {
			return nil, fmt.Errorf("decode command output: %w", err)
		}
		return &MCPToolResult{Summary: tool.Title + " completed.", Data: data}, nil
	}
	return s.executeSpecialMCPTool(ctx, principal, actor, tool.Name, arguments)
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
		task, err := s.tasks.GetByID(ctx, input.TaskID)
		if err != nil {
			return nil, err
		}
		if task.Task.WorkspaceID != principal.WorkspaceID {
			return nil, ErrMCPNotFound
		}
		return &MCPToolResult{Summary: "Task " + task.Task.Name + " loaded.", Data: task}, nil

	case "get_document":
		var input struct {
			DocumentID string `json:"document_id"`
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		document, err := s.documents.Get(ctx, input.DocumentID)
		if err != nil {
			return nil, err
		}
		if document.WorkspaceID != principal.WorkspaceID {
			return nil, ErrMCPNotFound
		}
		return &MCPToolResult{Summary: "Document " + document.Title + " loaded.", Data: document}, nil

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
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		input.Limit = normalizeMCPLimit(input.Limit)
		items, total, err := s.support.ListConversations(ctx, principal.WorkspaceID, input.Status, "", model.PMPagination{Page: 1, PerPage: input.Limit}, input.Search)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: fmt.Sprintf("Returned %d of %d support conversations.", len(items), total), Data: map[string]any{"items": items, "total": total}}, nil

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
		}
		if err := decodeMCPArguments(arguments, &input); err != nil {
			return nil, err
		}
		messages, err := s.support.ListConversationMessages(ctx, principal.WorkspaceID, input.ConversationID, false)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: fmt.Sprintf("Returned %d support messages.", len(messages)), Data: messages}, nil

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
		run, err := s.agents.CancelRun(ctx, principal.WorkspaceID, input.RunID, principal.UserID)
		if err != nil {
			return nil, err
		}
		return &MCPToolResult{Summary: "Agent run cancellation requested.", Data: map[string]any{"run_id": run.ID, "status": run.Status}}, nil
	default:
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

func mcpPrincipalKey(principal *model.MCPPrincipal) string {
	if principal.ConnectionID != "" {
		return "connection:" + principal.ConnectionID
	}
	return "service:" + principal.ServicePrincipalID
}

func mcpReasonCode(err error) string {
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
