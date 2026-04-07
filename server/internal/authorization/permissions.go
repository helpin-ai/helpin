package authorization

// Permission represents a named authorization permission.
type Permission string

// Workspace permissions.
const (
	PermWorkspaceRead          Permission = "workspace.read"
	PermWorkspaceUpdate        Permission = "workspace.update"
	PermWorkspaceDelete        Permission = "workspace.delete"
	PermWorkspaceMembersRead   Permission = "workspace.members.read"
	PermWorkspaceMembersManage Permission = "workspace.members.manage"
	PermWorkspaceInvitesManage Permission = "workspace.invites.manage"
	PermWorkspaceRolesManage   Permission = "workspace.roles.manage"
)

// Settings and team permissions.
const (
	PermSettingsRead       Permission = "settings.read"
	PermSettingsManage     Permission = "settings.manage"
	PermModuleAccessManage Permission = "module_access.manage"
	PermTeamRead           Permission = "team.read"
	PermTeamManage         Permission = "team.manage"
	PermTeamMembersRead    Permission = "team.members.read"
	PermTeamMembersManage  Permission = "team.members.manage"
)

// PM permissions.
const (
	PermPMRead             Permission = "pm.read"
	PermPMEdit             Permission = "pm.edit"
	PermPMAdminWorkflows   Permission = "pm.admin.workflows"
	PermPMAdminLabels      Permission = "pm.admin.labels"
	PermPMAdminAutomations Permission = "pm.admin.automations"
	PermPMImport           Permission = "pm.import"
)

// Docs permissions.
const (
	PermDocsRead    Permission = "docs.read"
	PermDocsEdit    Permission = "docs.edit"
	PermDocsPublish Permission = "docs.publish"
	PermDocsAdmin   Permission = "docs.admin"
	PermDocsImport  Permission = "docs.import"
)

// CRM permissions.
const (
	PermCRMRead  Permission = "crm.read"
	PermCRMEdit  Permission = "crm.edit"
	PermCRMAdmin Permission = "crm.admin"
)

// Support permissions.
const (
	PermSupportRead  Permission = "support.read"
	PermSupportEdit  Permission = "support.edit"
	PermSupportAdmin Permission = "support.admin"
)

// Notification permissions.
const (
	PermNotificationsRead   Permission = "notifications.read"
	PermNotificationsManage Permission = "notifications.manage"
)

// Other workspace-scoped permissions.
const (
	PermSearchRead Permission = "search.read"
	PermWSConnect  Permission = "ws.connect"
)

// AllPermissions returns every defined permission for test and introspection use.
func AllPermissions() []Permission {
	return []Permission{
		PermWorkspaceRead, PermWorkspaceUpdate, PermWorkspaceDelete,
		PermWorkspaceMembersRead, PermWorkspaceMembersManage,
		PermWorkspaceInvitesManage, PermWorkspaceRolesManage,
		PermSettingsRead, PermSettingsManage, PermModuleAccessManage,
		PermTeamRead, PermTeamManage, PermTeamMembersRead, PermTeamMembersManage,
		PermPMRead, PermPMEdit,
		PermPMAdminWorkflows, PermPMAdminLabels, PermPMAdminAutomations, PermPMImport,
		PermDocsRead, PermDocsEdit, PermDocsPublish, PermDocsAdmin, PermDocsImport,
		PermCRMRead, PermCRMEdit, PermCRMAdmin,
		PermSupportRead, PermSupportEdit, PermSupportAdmin,
		PermNotificationsRead, PermNotificationsManage,
		PermSearchRead, PermWSConnect,
	}
}
