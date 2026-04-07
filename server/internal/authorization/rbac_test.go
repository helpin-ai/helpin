package authorization

import (
	"testing"
)

func TestRBACEngine_RolePermissionMatrix(t *testing.T) {
	e := NewRBACEngine()

	// Every role gets these permissions.
	universalPerms := []Permission{
		PermWorkspaceRead, PermSettingsRead, PermWorkspaceMembersRead,
		PermTeamRead, PermTeamMembersRead, PermPMRead, PermSearchRead, PermWSConnect,
	}

	for _, role := range []string{"viewer", "member", "admin", "owner"} {
		for _, perm := range universalPerms {
			if !e.Can(role, perm) {
				t.Errorf("role %q should have permission %q", role, perm)
			}
		}
	}
}

func TestRBACEngine_ViewerRestrictions(t *testing.T) {
	e := NewRBACEngine()

	denied := []Permission{
		PermPMEdit,
		PermTeamManage, PermTeamMembersManage,
		PermSettingsManage, PermModuleAccessManage, PermWorkspaceUpdate, PermWorkspaceDelete,
		PermWorkspaceMembersManage, PermWorkspaceInvitesManage, PermWorkspaceRolesManage,
		PermPMAdminWorkflows, PermPMAdminLabels, PermPMAdminAutomations, PermPMImport,
	}
	for _, perm := range denied {
		if e.Can("viewer", perm) {
			t.Errorf("viewer should NOT have permission %q", perm)
		}
	}
}

func TestRBACEngine_MemberPermissions(t *testing.T) {
	e := NewRBACEngine()

	// Member gets pm.edit and docs.publish on top of viewer.
	if !e.Can("member", PermPMEdit) {
		t.Error("member should have pm.edit")
	}
	if !e.Can("member", PermDocsPublish) {
		t.Error("member should have docs.publish")
	}

	// Member does NOT get admin perms.
	denied := []Permission{
		PermTeamManage, PermSettingsManage, PermModuleAccessManage,
		PermWorkspaceUpdate, PermWorkspaceDelete,
		PermPMAdminWorkflows, PermPMAdminLabels, PermPMAdminAutomations, PermPMImport,
	}
	for _, perm := range denied {
		if e.Can("member", perm) {
			t.Errorf("member should NOT have permission %q", perm)
		}
	}
}

func TestRBACEngine_AdminPermissions(t *testing.T) {
	e := NewRBACEngine()

	adminPerms := []Permission{
		PermTeamManage, PermTeamMembersManage, PermSettingsManage, PermModuleAccessManage,
		PermWorkspaceUpdate, PermWorkspaceMembersManage,
		PermWorkspaceInvitesManage, PermWorkspaceRolesManage,
		PermPMAdminWorkflows, PermPMAdminLabels, PermPMAdminAutomations, PermPMImport,
		PermPMEdit, PermDocsPublish,
	}
	for _, perm := range adminPerms {
		if !e.Can("admin", perm) {
			t.Errorf("admin should have permission %q", perm)
		}
	}

	// Admin does NOT get workspace.delete.
	if e.Can("admin", PermWorkspaceDelete) {
		t.Error("admin should NOT have workspace.delete")
	}
}

func TestRBACEngine_OwnerPermissions(t *testing.T) {
	e := NewRBACEngine()

	// Owner gets everything including workspace.delete.
	for _, perm := range AllPermissions() {
		if !e.Can("owner", perm) {
			t.Errorf("owner should have permission %q", perm)
		}
	}
}

func TestRBACEngine_OwnerOnlyActions(t *testing.T) {
	e := NewRBACEngine()

	// workspace.delete is owner-only.
	if e.Can("admin", PermWorkspaceDelete) {
		t.Error("admin should NOT have workspace.delete")
	}
	if !e.Can("owner", PermWorkspaceDelete) {
		t.Error("owner should have workspace.delete")
	}

	if !e.IsOwnerOnly("owner") {
		t.Error("IsOwnerOnly should return true for owner")
	}
	if e.IsOwnerOnly("admin") {
		t.Error("IsOwnerOnly should return false for admin")
	}
}

func TestRBACEngine_CanAny(t *testing.T) {
	e := NewRBACEngine()

	if !e.CanAny("viewer", PermPMRead, PermPMEdit) {
		t.Error("viewer should have at least pm.read")
	}

	if e.CanAny("viewer", PermPMEdit, PermSettingsManage) {
		t.Error("viewer should NOT have pm.edit or settings.manage")
	}
}

func TestRBACEngine_UnknownRole(t *testing.T) {
	e := NewRBACEngine()

	if e.Can("unknown_role", PermWorkspaceRead) {
		t.Error("unknown role should have no permissions")
	}
	if e.CanAny("unknown_role", PermWorkspaceRead) {
		t.Error("unknown role should have no permissions")
	}
}

func TestRBACEngine_PermissionsForRole(t *testing.T) {
	e := NewRBACEngine()

	viewerPerms := e.PermissionsForRole("viewer")
	if len(viewerPerms) != 13 {
		t.Errorf("viewer should have 13 permissions, got %d", len(viewerPerms))
	}

	ownerPerms := e.PermissionsForRole("owner")
	allPerms := AllPermissions()
	if len(ownerPerms) != len(allPerms) {
		t.Errorf("owner should have %d permissions, got %d", len(allPerms), len(ownerPerms))
	}

	unknownPerms := e.PermissionsForRole("unknown")
	if unknownPerms != nil {
		t.Error("unknown role should return nil permissions")
	}
}

func TestRBACEngine_RoleHierarchyIsAdditive(t *testing.T) {
	e := NewRBACEngine()

	roles := []string{"viewer", "member", "admin", "owner"}
	for i := 1; i < len(roles); i++ {
		lower := roles[i-1]
		higher := roles[i]
		for _, perm := range e.PermissionsForRole(lower) {
			if !e.Can(higher, perm) {
				t.Errorf("role %q should include all permissions of %q, missing %q", higher, lower, perm)
			}
		}
	}
}
