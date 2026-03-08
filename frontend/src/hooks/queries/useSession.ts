import { useQuery } from '@tanstack/react-query'
import { workspacesService } from '@/lib/services/workspacesService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { WorkspaceMember } from '@/lib/types'

export function useSession(wsId: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.session(wsId),
    queryFn: async () => unwrap(await workspacesService.getMyMembership(wsId)),
    enabled: !!wsId,
    staleTime: 5 * 60_000,
  })
}

export function useSessionRole(membership: WorkspaceMember | null | undefined) {
  const role = membership?.role || ''
  return {
    isOwner: role === 'owner',
    isAdmin: ['owner', 'admin'].includes(role),
    isManager: ['owner', 'admin', 'manager'].includes(role),
    canEdit: ['owner', 'admin', 'manager'].includes(role),
  }
}
