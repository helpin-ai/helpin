package authorization

// RBACEngine is the in-memory role-to-permission matrix.
// It does not persist policies to the database.
type RBACEngine struct {
	rolePerms map[string]map[Permission]struct{}
}

// NewRBACEngine builds the engine with the §8.2 permission matrix.
func NewRBACEngine() *RBACEngine {
	viewerPerms := permsSet(
		PermWorkspaceRead, PermSettingsRead, PermWorkspaceMembersRead,
		PermTeamRead, PermTeamMembersRead,
		PermPMRead, PermDocsRead, PermCRMRead, PermSearchRead, PermWSConnect,
		PermNotificationsRead, PermNotificationsManage,
	)

	memberPerms := copyPerms(viewerPerms)
	addPerms(memberPerms, PermPMEdit, PermDocsEdit, PermCRMEdit, PermRewardsRead)

	managerPerms := copyPerms(memberPerms)
	addPerms(managerPerms, PermRewardsManage, PermDocsPublish)

	adminPerms := copyPerms(managerPerms)
	addPerms(adminPerms,
		PermTeamManage, PermTeamMembersManage,
		PermSettingsManage, PermWorkspaceUpdate,
		PermWorkspaceMembersManage, PermWorkspaceInvitesManage,
		PermWorkspaceRolesManage,
		PermPMAdminWorkflows, PermPMAdminLabels, PermPMAdminAutomations, PermPMImport,
		PermDocsAdmin, PermCRMAdmin,
	)

	ownerPerms := copyPerms(adminPerms)
	addPerms(ownerPerms, PermWorkspaceDelete)

	return &RBACEngine{
		rolePerms: map[string]map[Permission]struct{}{
			"viewer":  viewerPerms,
			"member":  memberPerms,
			"manager": managerPerms,
			"admin":   adminPerms,
			"owner":   ownerPerms,
		},
	}
}

// Can checks whether the given role has the specified permission.
func (e *RBACEngine) Can(role string, perm Permission) bool {
	perms, ok := e.rolePerms[role]
	if !ok {
		return false
	}
	_, has := perms[perm]
	return has
}

// CanAny checks whether the given role has at least one of the listed permissions.
func (e *RBACEngine) CanAny(role string, perms ...Permission) bool {
	for _, p := range perms {
		if e.Can(role, p) {
			return true
		}
	}
	return false
}

// PermissionsForRole returns all permissions granted to a role.
func (e *RBACEngine) PermissionsForRole(role string) []Permission {
	perms, ok := e.rolePerms[role]
	if !ok {
		return nil
	}
	result := make([]Permission, 0, len(perms))
	for p := range perms {
		result = append(result, p)
	}
	return result
}

// IsOwnerOnly returns true if the actor's role is "owner".
func (e *RBACEngine) IsOwnerOnly(role string) bool {
	return role == "owner"
}

func permsSet(perms ...Permission) map[Permission]struct{} {
	m := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		m[p] = struct{}{}
	}
	return m
}

func copyPerms(src map[Permission]struct{}) map[Permission]struct{} {
	dst := make(map[Permission]struct{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func addPerms(m map[Permission]struct{}, perms ...Permission) {
	for _, p := range perms {
		m[p] = struct{}{}
	}
}
