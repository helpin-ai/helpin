package service

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const (
	MCPToolsetContext = "context"
	MCPToolsetPM      = "pm"
	MCPToolsetDocs    = "docs"
	MCPToolsetCRM     = "crm"
	MCPToolsetSupport = "support"
	MCPToolsetAgents  = "agents"

	MCPScopeContextRead = "helpin.context.read"
	MCPScopePMRead      = "helpin.pm.read"
	MCPScopePMWrite     = "helpin.pm.write"
	MCPScopeDocsRead    = "helpin.docs.read"
	MCPScopeDocsWrite   = "helpin.docs.write"
	// MCPScopeDocsPublish allows publishing to and unpublishing from the public Help Center.
	MCPScopeDocsPublish = "helpin.docs.publish"
	MCPScopeCRMRead     = "helpin.crm.read"
	MCPScopeCRMWrite    = "helpin.crm.write"
	MCPScopeSupportRead = "helpin.support.read"
	// MCPScopeSupportWrite allows organizing conversations (assign, move, tag, link, rename); it never sends replies.
	MCPScopeSupportWrite = "helpin.support.write"
	MCPScopeAgentsRead   = "helpin.agents.read"
	MCPScopeAgentsRun    = "helpin.agents.run"
)

var (
	_defaultMCPToolsets = []string{MCPToolsetContext, MCPToolsetPM, MCPToolsetDocs, MCPToolsetAgents}
	_defaultMCPScopes   = []string{MCPScopeContextRead, MCPScopePMRead, MCPScopeDocsRead, MCPScopeAgentsRead}
	_allMCPToolsets     = []string{MCPToolsetContext, MCPToolsetPM, MCPToolsetDocs, MCPToolsetCRM, MCPToolsetSupport, MCPToolsetAgents}
	_allMCPScopes       = []string{
		MCPScopeContextRead,
		MCPScopePMRead, MCPScopePMWrite,
		MCPScopeDocsRead, MCPScopeDocsWrite, MCPScopeDocsPublish,
		MCPScopeCRMRead, MCPScopeCRMWrite,
		MCPScopeSupportRead, MCPScopeSupportWrite,
		MCPScopeAgentsRead, MCPScopeAgentsRun,
	}
)

// MCPToolDefinition describes one public MCP tool and its authorization requirements.
type MCPToolDefinition struct {
	Name           string
	Title          string
	Description    string
	InputSchema    map[string]any
	Toolset        string
	Scope          string
	Permission     authorization.Permission
	Module         model.ModuleID
	Mutating       bool
	CommandName    string
	Destructive    bool
	IdempotentHint bool
}

// AllMCPToolsets returns the stable public toolset identifiers.
func AllMCPToolsets() []string { return append([]string(nil), _allMCPToolsets...) }

// AllMCPScopes returns the stable public OAuth scopes.
func AllMCPScopes() []string { return append([]string(nil), _allMCPScopes...) }

// DefaultMCPToolsets returns the recommended read-oriented toolsets.
func DefaultMCPToolsets() []string { return append([]string(nil), _defaultMCPToolsets...) }

// DefaultMCPScopes returns the recommended read-oriented scopes.
func DefaultMCPScopes() []string { return append([]string(nil), _defaultMCPScopes...) }

func (s *MCPService) buildToolCatalog() []MCPToolDefinition {
	commandRequirements := map[string]MCPToolDefinition{
		"list_workspace_teams":  {Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermWorkspaceRead},
		"list_tasks":            {Toolset: MCPToolsetPM, Scope: MCPScopePMRead, Permission: authorization.PermPMRead, Module: model.ModulePM},
		"create_task":           {Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true},
		"create_epic":           {Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true},
		"add_task_comment":      {Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true},
		"update_task_state":     {Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true},
		"set_task_dependencies": {Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true},
		"get_task_context":      {Toolset: MCPToolsetPM, Scope: MCPScopePMRead, Permission: authorization.PermPMRead, Module: model.ModulePM},
		"search_documents":      {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"list_documents":        {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"read_document":         {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"get_document_blocks":   {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"create_document":       {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true},
		"update_document_block": {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true},
		"insert_document_block": {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true},
		"edit_document":         {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true},
		"list_repositories":     {Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermIntegrationsEnumerate},
		"list_contacts":         {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		"list_deals":            {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		"list_crm_signals":      {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		"create_crm_deal":       {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMWrite, Permission: authorization.PermCRMEdit, Module: model.ModuleCRM, Mutating: true},
		"add_deal_note":         {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMWrite, Permission: authorization.PermCRMEdit, Module: model.ModuleCRM, Mutating: true},
		"update_deal_stage":     {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMWrite, Permission: authorization.PermCRMEdit, Module: model.ModuleCRM, Mutating: true},
	}
	for alias, requirement := range parityMCPCommandRequirements() {
		commandRequirements[alias] = requirement
	}

	defs := make([]MCPToolDefinition, 0, len(commandRequirements)+27)
	for _, command := range s.commands.ToolDefinitions() {
		if command.Tool == nil {
			continue
		}
		requirement, ok := commandRequirements[command.Tool.Alias]
		if !ok {
			continue
		}
		requirement.Name = command.Tool.Alias
		requirement.Title = mcpToolTitle(command.Tool.Alias)
		requirement.Description = command.Tool.Description
		requirement.InputSchema = cloneMCPSchema(command.Tool.InputSchema)
		requirement.CommandName = command.Name
		requirement.IdempotentHint = requirement.Mutating
		if requirement.Mutating {
			requirement.InputSchema = withMCPIdempotencyKey(requirement.InputSchema)
		}
		defs = append(defs, requirement)
	}
	defs = append(defs, specialMCPToolDefinitions()...)
	defs = append(defs, supportSetupMCPToolDefinition())
	defs = append(defs, workspaceSetupMCPToolDefinition())
	defs = append(defs, docsLifecycleMCPToolDefinitions()...)
	defs = append(defs, uploadMCPToolDefinitions()...)
	defs = append(defs, docsBatchMCPToolDefinitions()...)
	defs = append(defs, helpcenterMCPToolDefinitions()...)
	defs = append(defs, helpcenterBulkMCPToolDefinitions()...)
	defs = append(defs, docsProposalMCPToolDefinitions()...)
	defs = append(defs, pmParityMCPToolDefinitions()...)
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}

func specialMCPToolDefinitions() []MCPToolDefinition {
	object := func(properties map[string]any, required ...string) map[string]any {
		schema := map[string]any{
			"type":                 "object",
			"properties":           properties,
			"additionalProperties": false,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	page := map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 25}
	identifierArray := func(maxItems int) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}, "uniqueItems": true, "maxItems": maxItems}
	}
	iconID := map[string]any{
		"type":        "string",
		"maxLength":   100,
		"description": "Canonical icon ID. Call search_icons to discover valid values; omit this field when no icon is needed.",
	}
	proposedTask := object(map[string]any{
		"ref":                 map[string]any{"type": "string", "maxLength": 100},
		"name":                map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		"description":         map[string]any{"type": "string", "maxLength": 100000},
		"task_type":           map[string]any{"type": "string", "enum": []string{model.PMTaskTypeFeature, model.PMTaskTypeBug, model.PMTaskTypeChore}},
		"estimate":            map[string]any{"type": "integer", "minimum": 0, "maximum": 1000},
		"priority":            map[string]any{"type": "string", "enum": []string{model.PMTaskPriorityNone, model.PMTaskPriorityLow, model.PMTaskPriorityMedium, model.PMTaskPriorityHigh, model.PMTaskPriorityUrgent}},
		"acceptance_criteria": map[string]any{"type": "array", "items": map[string]any{"type": "string", "maxLength": 5000}, "maxItems": 50},
		"dependency_refs":     identifierArray(50),
	}, "name")
	return []MCPToolDefinition{
		{Name: "get_current_context", Title: "Get current Helpin context", Description: "Return the connected user or service identity, workspace, role, scopes, toolsets, modules, and Helpin URL.", InputSchema: object(map[string]any{}), Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermWorkspaceRead},
		{Name: "search_workspace", Title: "Search Helpin", Description: "Search accessible workspace tasks, planning objects, documents, members, CRM records, and support conversations.", InputSchema: mustCommandToolMetadata("workspace.search").InputSchema, Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermSearchRead, CommandName: "workspace.search"},
		{Name: "get_task", Title: "Get task", Description: "Get one Helpin task by ID.", InputSchema: object(map[string]any{"task_id": map[string]any{"type": "string"}}, "task_id"), Toolset: MCPToolsetPM, Scope: MCPScopePMRead, Permission: authorization.PermPMRead, Module: model.ModulePM},
		{Name: "update_task", Title: "Update task", Description: "Update bounded editable fields on one task without deleting, archiving, or moving it between teams.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"task_id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}, "description": map[string]any{"type": "string", "maxLength": 100000}, "task_type": map[string]any{"type": "string", "enum": []string{model.PMTaskTypeFeature, model.PMTaskTypeBug, model.PMTaskTypeChore}}, "epic_id": map[string]any{"type": "string"}, "sprint_id": map[string]any{"type": "string"}, "owner_member_ids": identifierArray(50), "estimate": map[string]any{"type": "integer", "minimum": -1, "maximum": 1000}, "priority": map[string]any{"type": "string", "enum": []string{model.PMTaskPriorityNone, model.PMTaskPriorityLow, model.PMTaskPriorityMedium, model.PMTaskPriorityHigh, model.PMTaskPriorityUrgent}}, "severity": map[string]any{"type": "string", "enum": []string{model.PMTaskSeverityNone, model.PMTaskSeverityMinor, model.PMTaskSeverityMajor, model.PMTaskSeverityCritical}}, "blocked": map[string]any{"type": "boolean"}, "blocker": map[string]any{"type": "string", "maxLength": 10000}, "label_ids": identifierArray(100)}, "task_id")), Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true, IdempotentHint: true},
		{Name: "create_task_batch", Title: "Create task batch", Description: "Create an idempotent bounded batch of implementation-ready tasks in one epic.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"epic_id": map[string]any{"type": "string"}, "tasks": map[string]any{"type": "array", "items": proposedTask, "minItems": 1, "maxItems": 50}}, "epic_id", "tasks")), Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true, IdempotentHint: true},
		{Name: "list_task_checklist", Title: "List task checklist", Description: "List up to 100 checklist items for one accessible task.", InputSchema: object(map[string]any{"task_id": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 100}}, "task_id"), Toolset: MCPToolsetPM, Scope: MCPScopePMRead, Permission: authorization.PermPMRead, Module: model.ModulePM},
		{Name: "create_task_checklist_item", Title: "Create task checklist item", Description: "Create one checklist item on an accessible task.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"task_id": map[string]any{"type": "string"}, "text": map[string]any{"type": "string", "minLength": 1, "maxLength": 10000}, "position": map[string]any{"type": "integer", "minimum": 0}, "due_date": map[string]any{"type": "string", "format": "date"}}, "task_id", "text")), Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true, IdempotentHint: true},
		{Name: "update_task_checklist_item", Title: "Update task checklist item", Description: "Update the text, completion state, position, or due date of one checklist item.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"task_id": map[string]any{"type": "string"}, "checklist_item_id": map[string]any{"type": "string"}, "text": map[string]any{"type": "string", "minLength": 1, "maxLength": 10000}, "completed": map[string]any{"type": "boolean"}, "position": map[string]any{"type": "integer", "minimum": 0}, "due_date": map[string]any{"type": []string{"string", "null"}, "format": "date", "description": "Use YYYY-MM-DD, null to clear, or omit to preserve."}}, "task_id", "checklist_item_id")), Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true, IdempotentHint: true},
		{Name: "list_spaces", Title: "List Docs spaces", Description: "List Docs spaces visible to the connected actor, including IDs needed for collection and document creation.", InputSchema: object(map[string]any{}), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		{Name: "list_collections", Title: "List Docs collections", Description: "List Docs collections visible to the connected actor, optionally filtered by space.", InputSchema: object(map[string]any{"space_id": map[string]any{"type": "string"}}), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		{Name: "search_icons", Title: "Search Docs icons", Description: "Search the canonical icon catalog before assigning an icon to a Docs space, collection, or document.", InputSchema: object(map[string]any{"query": map[string]any{"type": "string", "maxLength": 100}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "default": 20}}), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		{Name: "get_document", Title: "Get document", Description: "Get one Helpin document by ID.", InputSchema: object(map[string]any{"document_id": map[string]any{"type": "string"}}, "document_id"), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		{Name: "create_space", Title: "Create Docs space", Description: "Create a Docs space in the connected workspace without publishing content.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "slug": map[string]any{"type": "string", "maxLength": 200}, "icon": iconID, "visibility": map[string]any{"type": "string", "enum": []string{model.SpaceVisibilityWorkspaceWide, model.SpaceVisibilityTeamOnly}}, "type": map[string]any{"type": "string", "enum": []string{model.SpaceTypeInternal, model.SpaceTypeExternalCapable}}, "default_review_days": map[string]any{"type": "integer", "minimum": 1, "maximum": 3650}}, "name")), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true, IdempotentHint: true},
		{Name: "create_collection", Title: "Create Docs collection", Description: "Create a collection in an accessible Docs space, optionally nested under another collection.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"space_id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "slug": map[string]any{"type": "string", "maxLength": 200}, "description": map[string]any{"type": "string", "maxLength": 5000}, "icon": iconID, "parent_collection_id": map[string]any{"type": "string"}}, "space_id", "name")), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true, IdempotentHint: true},
		{Name: "update_space", Title: "Update Docs space", Description: "Update bounded metadata for an accessible Docs space without changing its public URL or publishing state.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"space_id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "icon": iconID, "visibility": map[string]any{"type": "string", "enum": []string{model.SpaceVisibilityWorkspaceWide, model.SpaceVisibilityTeamOnly}}, "default_review_days": map[string]any{"type": "integer", "minimum": 1, "maximum": 3650}}, "space_id")), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true, IdempotentHint: true},
		{Name: "update_collection", Title: "Update Docs collection", Description: "Update or reparent an accessible Docs collection.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"collection_id": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}, "description": map[string]any{"type": "string", "maxLength": 5000}, "icon": iconID, "position": map[string]any{"type": "integer", "minimum": 0}, "parent_collection_id": map[string]any{"type": "string"}}, "collection_id")), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true, IdempotentHint: true},
		{Name: "move_document", Title: "Move Docs document", Description: "Move an accessible document to another accessible space or collection.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"document_id": map[string]any{"type": "string"}, "space_id": map[string]any{"type": "string"}, "collection_id": map[string]any{"type": "string"}}, "document_id", "space_id")), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true, IdempotentHint: true},
		{Name: "link_document_to_object", Title: "Link document to object", Description: "Create a validated link between an accessible document and an accessible Helpin object.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"document_id": map[string]any{"type": "string"}, "linked_object_type": map[string]any{"type": "string", "enum": []string{model.LinkedObjectEpic, model.LinkedObjectTask, model.LinkedObjectSupportConversation, model.LinkedObjectDeal, model.LinkedObjectContact, model.LinkedObjectCompany}}, "linked_object_id": map[string]any{"type": "string"}, "link_context": map[string]any{"type": "string", "enum": []string{model.LinkContextAttached, model.LinkContextMentioned, model.LinkContextCreatedFrom, model.LinkContextLinkedInContent}}}, "document_id", "linked_object_type", "linked_object_id")), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true, IdempotentHint: true},
		{Name: "get_crm_contact", Title: "Get CRM contact", Description: "Get one CRM contact by ID.", InputSchema: object(map[string]any{"contact_id": map[string]any{"type": "string"}}, "contact_id"), Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		{Name: "get_crm_deal", Title: "Get CRM deal", Description: "Get one CRM deal by ID.", InputSchema: object(map[string]any{"deal_id": map[string]any{"type": "string"}}, "deal_id"), Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		{Name: "list_support_conversations", Title: "List support conversations", Description: "List support conversations with optional status and search filters.", InputSchema: object(map[string]any{"status": map[string]any{"type": "string"}, "search": map[string]any{"type": "string", "maxLength": 500}, "limit": page, "offset": map[string]any{"type": "integer", "minimum": 0}}), Toolset: MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportRead, Module: model.ModuleSupport},
		{Name: "get_support_conversation", Title: "Get support conversation", Description: "Get one support conversation by ID.", InputSchema: object(map[string]any{"conversation_id": map[string]any{"type": "string"}}, "conversation_id"), Toolset: MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportRead, Module: model.ModuleSupport},
		{Name: "list_conversation_messages", Title: "List support messages", Description: "List public messages in a support conversation. Internal notes are excluded.", InputSchema: object(map[string]any{"conversation_id": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}, "offset": map[string]any{"type": "integer", "minimum": 0}}, "conversation_id"), Toolset: MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportRead, Module: model.ModuleSupport},
		{Name: "list_agents", Title: "List Helpin agents", Description: "List Helpin system and custom agents available to the connected actor.", InputSchema: object(map[string]any{}), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRead, Permission: authorization.PermPMRead},
		{Name: "start_agent_run", Title: "Start agent run", Description: "Start a durable Helpin agent run and return a run handle for polling. Task targets accept a task ID or key such as HEL-12. Agents with repository tools need a task repository: pass repository_id from list_repositories (and optionally base_branch) to set it, as the app's repository picker does.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"agent_id": map[string]any{"type": "string"}, "target_type": map[string]any{"type": "string", "enum": []string{"workspace", "task", "epic", "sprint", "objective", "document", "crm_deal", "crm_contact", "support_conversation"}}, "target_id": map[string]any{"type": "string"}, "additional_context": map[string]any{"type": "string", "maxLength": 20000}, "repository_id": map[string]any{"type": "string", "minLength": 1, "description": "Task targets only. Sets the task's delivery repository before the run; requires PM write access."}, "base_branch": map[string]any{"type": "string", "minLength": 1, "maxLength": 255, "description": "Task targets only, with repository_id. Defaults to the repository's default branch."}}, "agent_id", "target_type", "target_id")), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRun, Permission: authorization.PermPMEdit, Mutating: true, IdempotentHint: true},
		{Name: "get_agent_run", Title: "Get agent run", Description: "Poll a Helpin agent run and return status, output, artifacts, and links.", InputSchema: object(map[string]any{"run_id": map[string]any{"type": "string"}}, "run_id"), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRead, Permission: authorization.PermPMRead},
		{Name: "cancel_agent_run", Title: "Cancel agent run", Description: "Cancel an active Helpin agent run.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"run_id": map[string]any{"type": "string"}}, "run_id")), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRun, Permission: authorization.PermPMEdit, Mutating: true, Destructive: true, IdempotentHint: true},
	}
}

// _mcpTitleAcronyms keeps product acronyms upper case in display titles.
var _mcpTitleAcronyms = map[string]string{"pm": "PM", "crm": "CRM", "mcp": "MCP", "id": "ID", "url": "URL", "api": "API"}

// mcpToolTitle turns a snake_case tool name into a sentence-case display
// title, for example list_crm_companies becomes "List CRM companies".
func mcpToolTitle(name string) string {
	words := strings.Split(strings.TrimSpace(name), "_")
	for index, word := range words {
		if acronym, ok := _mcpTitleAcronyms[word]; ok {
			words[index] = acronym
			continue
		}
		if index == 0 && word != "" {
			words[index] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func cloneMCPSchema(schema map[string]any) map[string]any {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object"}
	}
	var copied map[string]any
	if err := json.Unmarshal(encoded, &copied); err != nil {
		return map[string]any{"type": "object"}
	}
	return copied
}

func withMCPIdempotencyKey(schema map[string]any) map[string]any {
	schema = cloneMCPSchema(schema)
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		properties = make(map[string]any)
		schema["properties"] = properties
	}
	properties["idempotency_key"] = map[string]any{
		"type":        "string",
		"minLength":   8,
		"maxLength":   128,
		"description": "Stable key used to safely retry this mutation.",
	}
	required, _ := schema["required"].([]string)
	if required == nil {
		if values, ok := schema["required"].([]any); ok {
			for _, value := range values {
				if text, ok := value.(string); ok {
					required = append(required, text)
				}
			}
		}
	}
	required = append(required, "idempotency_key")
	schema["required"] = required
	return schema
}

// PublicToolCatalog returns the curated tool definitions without requiring a
// running service, so generated API contracts can be derived from them.
func PublicToolCatalog() []MCPToolDefinition {
	service := &MCPService{commands: NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)}
	return service.buildToolCatalog()
}
