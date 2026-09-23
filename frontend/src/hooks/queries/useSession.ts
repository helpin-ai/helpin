import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { workspacesService } from '@/lib/services/workspacesService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { Permission, WorkspaceAccess, WorkspaceMember, WorkspaceModule } from '@/lib/types'
import { useAuthStore } from '@/stores/authStore'

/**
 * useSession fetches the legacy my-membership endpoint.
 * Kept for backward compatibility — prefer useWorkspaceAccess for new code.
 */
export function useSession(wsId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: queryKeys.workspaces.session(wsId),
    queryFn: async () => unwrap(await workspacesService.getMyMembership(wsId)),
    enabled: !!wsId && (options?.enabled ?? true),
    staleTime: 5 * 60_000,
  })
}

/**
 * useWorkspaceAccess fetches the /me endpoint which returns
 * membership, effective permissions, and team memberships.
 * This is the canonical source of truth for frontend access control.
 */
export function useWorkspaceAccess(wsId: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.access(wsId),
    queryFn: async () => unwrap(await workspacesService.getMe(wsId)),
    enabled: !!wsId,
    staleTime: 5 * 60_000,
  })
}

/**
 * usePermissions builds a permission checker from workspace access data.
 * Returns `has(perm)` and `hasAny(perms)` functions plus convenience booleans.
 */
export function usePermissions(access: WorkspaceAccess | null | undefined) {
  const isServerAdmin = useAuthStore((state) => Boolean(state.user?.is_server_admin))
  return useMemo(() => {
    const permSet = new Set<string>(access?.permissions ?? [])
    // Server administration is account-level, not a workspace role; expose it
    // alongside workspace permissions so settings navigation can gate on it.
    if (isServerAdmin) permSet.add('server.admin')
    const moduleSet = new Set<string>(access?.modules ?? [])
    const role = access?.membership?.role ?? ''

    const has = (perm: Permission): boolean => permSet.has(perm)
    const hasAny = (...perms: Permission[]): boolean => perms.some(p => permSet.has(p))
    const canAccessModule = (module: WorkspaceModule): boolean => moduleSet.has(module)
    const isOwner = role === 'owner'
    const isAdmin = role === 'owner' || role === 'admin'
    const teamMemberships = access?.team_memberships ?? []
    const isTeamManager = (teamId: string | null | undefined): boolean =>
      !!teamId && teamMemberships.some(tm => tm.team_id === teamId && tm.role === 'owner')

    return {
      /** Check a single permission */
      has,
      /** Check if at least one permission matches */
      hasAny,
      /** Check module-level access */
      canAccessModule,
      /** All effective permissions as a Set */
      permissionSet: permSet,
      /** All accessible modules as a Set */
      moduleSet,
      /** The actor's workspace role */
      role,
      /** Team memberships from the /me response */
      teamMemberships,
      /** Check if user is a team manager (team owner) for a specific team */
      isTeamManager,
      /** Check if user can manage sprints/objectives in a given team */
      canManageTeam: (teamId: string | null | undefined): boolean =>
        isAdmin || isTeamManager(teamId),

      // ── Convenience booleans (backward-compatible with useSessionRole) ──
      isOwner,
      isAdmin,
      /** Can edit PM content (member+) */
      canEdit: has('pm.edit'),
      /** Can manage settings (admin+) */
      canManageSettings: has('settings.manage'),
      /** Can manage module grants (admin+) */
      canManageModuleAccess: has('module_access.manage'),
      /** Can manage workspace members (admin+) */
      canManageMembers: has('workspace.members.manage'),
      /** Can manage invitations (admin+) */
      canManageInvites: has('workspace.invites.manage'),
      /** Can manage teams (admin+) */
      canManageTeams: has('team.manage'),
      /** Can manage team members (admin+) */
      canManageTeamMembers: has('team.members.manage'),
      /** Can admin PM workflows (admin+) */
      canAdminWorkflows: has('pm.admin.workflows'),
      /** Can admin PM labels (admin+) */
      canAdminLabels: has('pm.admin.labels'),
      /** Can admin PM automations (admin+) */
      canAdminAutomations: has('pm.admin.automations'),
      /** Can import PM data (admin+) */
      canImport: has('pm.import'),
      /** Can delete workspace (owner only) */
      canDeleteWorkspace: has('workspace.delete'),
      /** Can read docs (viewer+) */
      canReadDocs: has('docs.read'),
      /** Can edit docs (member+) */
      canEditDocs: has('docs.edit'),
      /** Can publish docs (member+) */
      canPublishDocs: has('docs.publish'),
      /** Can admin docs (admin+) */
      canAdminDocs: has('docs.admin'),
      /** Accessible workspace modules */
      modules: access?.modules ?? [],
      /** Administers this self-hosted server (Community) */
      isServerAdmin,
    }
  }, [access, isServerAdmin])
}

/**
 * useSessionRole provides backward-compatible role booleans from a WorkspaceMember.
 * @deprecated Prefer usePermissions(useWorkspaceAccess(wsId).data) for new code.
 */
export function useUpdateSupportTaskPreferences(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: {
      support_default_team_id?: string
      support_task_dialog_dismissed?: boolean
    }) => workspacesService.updateSupportTaskPreferences(workspaceId, data).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.access(workspaceId) })
    },
  })
}

export function useSessionRole(membership: WorkspaceMember | null | undefined) {
  const role = membership?.role || ''
  return {
    isOwner: role === 'owner',
    isAdmin: ['owner', 'admin'].includes(role),
    canEdit: ['owner', 'admin', 'member'].includes(role),
  }
}
