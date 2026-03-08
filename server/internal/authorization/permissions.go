package authorization

// Permission represents a named authorization permission.
type Permission string

// Workspace permissions.
const (
	PermWorkspaceRead         Permission = "workspace.read"
	PermWorkspaceUpdate       Permission = "workspace.update"
	PermWorkspaceDelete       Permission = "workspace.delete"
	PermWorkspaceMembersRead  Permission = "workspace.members.read"
	PermWorkspaceMembersManage Permission = "workspace.members.manage"
	PermWorkspaceInvitesManage Permission = "workspace.invites.manage"
	PermWorkspaceRolesManage   Permission = "workspace.roles.manage"
)

// Settings and team permissions.
const (
	PermSettingsRead       Permission = "settings.read"
	PermSettingsManage     Permission = "settings.manage"
	PermTeamRead           Permission = "team.read"
	PermTeamManage         Permission = "team.manage"
	PermTeamMembersRead    Permission = "team.members.read"
	PermTeamMembersManage  Permission = "team.members.manage"
)

// PM permissions.
const (
	PermPMRead            Permission = "pm.read"
	PermPMEdit            Permission = "pm.edit"
	PermPMAdminWorkflows  Permission = "pm.admin.workflows"
	PermPMAdminLabels     Permission = "pm.admin.labels"
	PermPMAdminAutomations Permission = "pm.admin.automations"
	PermPMImport          Permission = "pm.import"
)

// Other workspace-scoped permissions.
const (
	PermRewardsRead    Permission = "rewards.read"
	PermRewardsManage  Permission = "rewards.manage"
	PermSearchRead     Permission = "search.read"
	PermWSConnect      Permission = "ws.connect"
)

// AllPermissions returns every defined permission for test and introspection use.
func AllPermissions() []Permission {
	return []Permission{
		PermWorkspaceRead, PermWorkspaceUpdate, PermWorkspaceDelete,
		PermWorkspaceMembersRead, PermWorkspaceMembersManage,
		PermWorkspaceInvitesManage, PermWorkspaceRolesManage,
		PermSettingsRead, PermSettingsManage,
		PermTeamRead, PermTeamManage, PermTeamMembersRead, PermTeamMembersManage,
		PermPMRead, PermPMEdit,
		PermPMAdminWorkflows, PermPMAdminLabels, PermPMAdminAutomations, PermPMImport,
		PermRewardsRead, PermRewardsManage,
		PermSearchRead, PermWSConnect,
	}
}
