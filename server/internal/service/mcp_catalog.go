package service

import (
	"encoding/json"
	"sort"

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
	MCPScopeCRMRead     = "helpin.crm.read"
	MCPScopeCRMWrite    = "helpin.crm.write"
	MCPScopeSupportRead = "helpin.support.read"
	MCPScopeAgentsRead  = "helpin.agents.read"
	MCPScopeAgentsRun   = "helpin.agents.run"
)

var (
	_defaultMCPToolsets = []string{MCPToolsetContext, MCPToolsetPM, MCPToolsetDocs, MCPToolsetAgents}
	_defaultMCPScopes   = []string{MCPScopeContextRead, MCPScopePMRead, MCPScopeDocsRead, MCPScopeAgentsRead}
	_allMCPToolsets     = []string{MCPToolsetContext, MCPToolsetPM, MCPToolsetDocs, MCPToolsetCRM, MCPToolsetSupport, MCPToolsetAgents}
	_allMCPScopes       = []string{
		MCPScopeContextRead,
		MCPScopePMRead, MCPScopePMWrite,
		MCPScopeDocsRead, MCPScopeDocsWrite,
		MCPScopeCRMRead, MCPScopeCRMWrite,
		MCPScopeSupportRead,
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
		"add_task_comment":      {Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true},
		"update_task_state":     {Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true},
		"search_documents":      {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"list_documents":        {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"read_document":         {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"get_document_blocks":   {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		"create_document":       {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true},
		"update_document_block": {Toolset: MCPToolsetDocs, Scope: MCPScopeDocsWrite, Permission: authorization.PermDocsEdit, Module: model.ModuleDocs, Mutating: true},
		"list_repositories":     {Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermIntegrationsEnumerate},
		"list_contacts":         {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		"list_deals":            {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		"list_buyer_signals":    {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		"add_deal_note":         {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMWrite, Permission: authorization.PermCRMEdit, Module: model.ModuleCRM, Mutating: true},
		"update_deal_stage":     {Toolset: MCPToolsetCRM, Scope: MCPScopeCRMWrite, Permission: authorization.PermCRMEdit, Module: model.ModuleCRM, Mutating: true},
	}

	defs := make([]MCPToolDefinition, 0, len(commandRequirements)+16)
	for _, command := range s.commands.ToolDefinitions() {
		if command.Tool == nil {
			continue
		}
		requirement, ok := commandRequirements[command.Tool.Alias]
		if !ok {
			continue
		}
		requirement.Name = command.Tool.Alias
		requirement.Title = command.Tool.Alias
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
	return []MCPToolDefinition{
		{Name: "get_current_context", Title: "Get current Helpin context", Description: "Return the connected user or service identity, workspace, role, scopes, toolsets, modules, and Helpin URL.", InputSchema: object(map[string]any{}), Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermWorkspaceRead},
		{Name: "search_workspace", Title: "Search Helpin", Description: "Search tasks and documents in the connected Helpin workspace.", InputSchema: object(map[string]any{"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}}, "query"), Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermSearchRead},
		{Name: "get_task", Title: "Get task", Description: "Get one Helpin task by ID.", InputSchema: object(map[string]any{"task_id": map[string]any{"type": "string"}}, "task_id"), Toolset: MCPToolsetPM, Scope: MCPScopePMRead, Permission: authorization.PermPMRead, Module: model.ModulePM},
		{Name: "get_document", Title: "Get document", Description: "Get one Helpin document by ID.", InputSchema: object(map[string]any{"document_id": map[string]any{"type": "string"}}, "document_id"), Toolset: MCPToolsetDocs, Scope: MCPScopeDocsRead, Permission: authorization.PermDocsRead, Module: model.ModuleDocs},
		{Name: "get_crm_contact", Title: "Get CRM contact", Description: "Get one CRM contact by ID.", InputSchema: object(map[string]any{"contact_id": map[string]any{"type": "string"}}, "contact_id"), Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		{Name: "get_crm_deal", Title: "Get CRM deal", Description: "Get one CRM deal by ID.", InputSchema: object(map[string]any{"deal_id": map[string]any{"type": "string"}}, "deal_id"), Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM},
		{Name: "list_support_conversations", Title: "List support conversations", Description: "List support conversations with optional status and search filters.", InputSchema: object(map[string]any{"status": map[string]any{"type": "string"}, "search": map[string]any{"type": "string", "maxLength": 500}, "limit": page}), Toolset: MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportRead, Module: model.ModuleSupport},
		{Name: "get_support_conversation", Title: "Get support conversation", Description: "Get one support conversation by ID.", InputSchema: object(map[string]any{"conversation_id": map[string]any{"type": "string"}}, "conversation_id"), Toolset: MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportRead, Module: model.ModuleSupport},
		{Name: "list_conversation_messages", Title: "List support messages", Description: "List public messages in a support conversation. Internal notes are excluded.", InputSchema: object(map[string]any{"conversation_id": map[string]any{"type": "string"}}, "conversation_id"), Toolset: MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportRead, Module: model.ModuleSupport},
		{Name: "list_agents", Title: "List Helpin agents", Description: "List Helpin system and custom agents available to the connected actor.", InputSchema: object(map[string]any{}), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRead, Permission: authorization.PermPMRead},
		{Name: "start_agent_run", Title: "Start agent run", Description: "Start a durable Helpin agent run and return a run handle for polling.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"agent_id": map[string]any{"type": "string"}, "target_type": map[string]any{"type": "string", "enum": []string{"workspace", "task", "epic", "document", "crm_deal", "crm_contact", "support_conversation"}}, "target_id": map[string]any{"type": "string"}, "additional_context": map[string]any{"type": "string", "maxLength": 20000}}, "agent_id", "target_type", "target_id")), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRun, Permission: authorization.PermPMEdit, Mutating: true, IdempotentHint: true},
		{Name: "get_agent_run", Title: "Get agent run", Description: "Poll a Helpin agent run and return status, output, artifacts, and links.", InputSchema: object(map[string]any{"run_id": map[string]any{"type": "string"}}, "run_id"), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRead, Permission: authorization.PermPMRead},
		{Name: "cancel_agent_run", Title: "Cancel agent run", Description: "Cancel an active Helpin agent run.", InputSchema: withMCPIdempotencyKey(object(map[string]any{"run_id": map[string]any{"type": "string"}}, "run_id")), Toolset: MCPToolsetAgents, Scope: MCPScopeAgentsRun, Permission: authorization.PermPMEdit, Mutating: true, Destructive: true, IdempotentHint: true},
	}
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
