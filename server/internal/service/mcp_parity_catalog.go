package service

import (
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// parityMCPCommandRequirements exposes existing, agent-tested internal
// commands through public MCP for parity with Linear and Plane. Each entry
// binds the command to a toolset, OAuth scope, RBAC permission, and module.
// Customer-visible sends, enrichment, and deletes stay excluded.
func parityMCPCommandRequirements() map[string]MCPToolDefinition {
	pmRead := MCPToolDefinition{Toolset: MCPToolsetPM, Scope: MCPScopePMRead, Permission: authorization.PermPMRead, Module: model.ModulePM}
	pmWrite := MCPToolDefinition{Toolset: MCPToolsetPM, Scope: MCPScopePMWrite, Permission: authorization.PermPMEdit, Module: model.ModulePM, Mutating: true}
	crmRead := MCPToolDefinition{Toolset: MCPToolsetCRM, Scope: MCPScopeCRMRead, Permission: authorization.PermCRMRead, Module: model.ModuleCRM}
	crmWrite := MCPToolDefinition{Toolset: MCPToolsetCRM, Scope: MCPScopeCRMWrite, Permission: authorization.PermCRMEdit, Module: model.ModuleCRM, Mutating: true}
	supportRead := MCPToolDefinition{Toolset: MCPToolsetSupport, Scope: MCPScopeSupportRead, Permission: authorization.PermSupportRead, Module: model.ModuleSupport}
	supportWrite := MCPToolDefinition{Toolset: MCPToolsetSupport, Scope: MCPScopeSupportWrite, Permission: authorization.PermSupportEdit, Module: model.ModuleSupport, Mutating: true}
	contextRead := MCPToolDefinition{Toolset: MCPToolsetContext, Scope: MCPScopeContextRead, Permission: authorization.PermWorkspaceMembersRead}

	return map[string]MCPToolDefinition{
		// Projects: epics, sprints, objectives, labels, workflows.
		"list_epics":                      pmRead,
		"get_epic":                        pmRead,
		"update_epic":                     pmWrite,
		"list_sprints":                    pmRead,
		"get_sprint":                      pmRead,
		"list_sprint_tasks":               pmRead,
		"create_sprint":                   pmWrite,
		"update_sprint":                   pmWrite,
		"list_objectives":                 pmRead,
		"get_objective":                   pmRead,
		"create_objective":                pmWrite,
		"update_objective":                pmWrite,
		"update_key_result":               pmWrite,
		"list_pm_labels":                  pmRead,
		"ensure_task_label":               pmWrite,
		"list_team_workflows_with_stages": pmRead,

		// Workspace context.
		"list_workspace_members": contextRead,

		// CRM.
		"get_crm_company":             crmRead,
		"list_crm_companies":          crmRead,
		"list_crm_pipelines":          crmRead,
		"list_crm_associations":       crmRead,
		"add_crm_activity":            crmWrite,
		"update_crm_contact":          crmWrite,
		"update_crm_company":          crmWrite,
		"update_crm_deal":             crmWrite,
		"link_crm_objects":            crmWrite,
		"unlink_crm_association":      crmWrite,
		"set_primary_contact_company": crmWrite,

		// Support: organize conversations. Replies are never sent through MCP.
		"list_support_inboxes":                supportRead,
		"list_support_tags":                   supportRead,
		"list_support_assignees":              supportRead,
		"assign_support_conversation":         supportWrite,
		"move_support_conversation":           supportWrite,
		"add_support_conversation_tag":        supportWrite,
		"remove_support_conversation_tag":     supportWrite,
		"link_support_conversation_task":      supportWrite,
		"link_support_conversation_contact":   supportWrite,
		"update_support_conversation_subject": supportWrite,
	}
}
