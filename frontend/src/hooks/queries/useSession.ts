import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { workspacesService } from '@/lib/services/workspacesService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { Permission, WorkspaceAccess, WorkspaceMember } from '@/lib/types'

/**
 * useSession fetches the legacy my-membership endpoint.
 * Kept for backward compatibility — prefer useWorkspaceAccess for new code.
 */
export function useSession(wsId: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.session(wsId),
    queryFn: async () => unwrap(await workspacesService.getMyMembership(wsId)),
    enabled: !!wsId,
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
  return useMemo(() => {
    const permSet = new Set<string>(access?.permissions ?? [])
    const role = access?.membership?.role ?? ''

    const has = (perm: Permission): boolean => permSet.has(perm)
    const hasAny = (...perms: Permission[]): boolean => perms.some(p => permSet.has(p))

    return {
      /** Check a single permission */
      has,
      /** Check if at least one permission matches */
      hasAny,
      /** All effective permissions as a Set */
      permissionSet: permSet,
      /** The actor's workspace role */
      role,
      /** Team memberships from the /me response */
      teamMemberships: access?.team_memberships ?? [],

      // ── Convenience booleans (backward-compatible with useSessionRole) ──
      isOwner: role === 'owner',
      isAdmin: role === 'owner' || role === 'admin',
      isManager: role === 'owner' || role === 'admin' || role === 'manager',
      /** Can edit PM content (member+) */
      canEdit: has('pm.edit'),
      /** Can manage settings (admin+) */
      canManageSettings: has('settings.manage'),
      /** Can manage workspace members (admin+) */
      canManageMembers: has('workspace.members.manage'),
      /** Can manage invitations (admin+) */
      canManageInvites: has('workspace.invites.manage'),
      /** Can manage teams (admin+) */
      canManageTeams: has('team.manage'),
      /** Can manage team members (admin+) */
      canManageTeamMembers: has('team.members.manage'),
      /** Can read rewards (member+) */
      canReadRewards: has('rewards.read'),
      /** Can manage rewards (manager+) */
      canManageRewards: has('rewards.manage'),
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
      /** Can publish docs (manager+) */
      canPublishDocs: has('docs.publish'),
      /** Can admin docs (admin+) */
      canAdminDocs: has('docs.admin'),
    }
  }, [access])
}

/**
 * useSessionRole provides backward-compatible role booleans from a WorkspaceMember.
 * @deprecated Prefer usePermissions(useWorkspaceAccess(wsId).data) for new code.
 */
export function useSessionRole(membership: WorkspaceMember | null | undefined) {
  const role = membership?.role || ''
  return {
    isOwner: role === 'owner',
    isAdmin: ['owner', 'admin'].includes(role),
    isManager: ['owner', 'admin', 'manager'].includes(role),
    canEdit: ['owner', 'admin', 'manager', 'member'].includes(role),
  }
}
