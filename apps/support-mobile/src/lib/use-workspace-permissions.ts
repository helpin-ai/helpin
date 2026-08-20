import { useQuery } from '@tanstack/react-query'

import { workspacesService } from '@mobile/lib/services/workspaces-service'
import type { WorkspaceAccess } from '@mobile/lib/types'

export interface SupportPermissionFlags {
  canReadSupport: boolean
  canEditSupport: boolean
  canAdminSupport: boolean
  canReadPM: boolean
  canEditPM: boolean
}

export function supportPermissionFlags(access?: WorkspaceAccess | null): SupportPermissionFlags {
  const permissions = new Set(access?.permissions ?? [])
  const hasSupportModule = access?.modules.includes('support') ?? false
  return {
    canReadSupport: hasSupportModule && permissions.has('support.read'),
    canEditSupport: hasSupportModule && permissions.has('support.edit'),
    canAdminSupport: hasSupportModule && permissions.has('support.admin'),
    canReadPM: (access?.modules.includes('pm') ?? false) && permissions.has('pm.read'),
    canEditPM: (access?.modules.includes('pm') ?? false) && permissions.has('pm.edit'),
  }
}

export function useWorkspacePermissions(workspaceId: string) {
  const accessQuery = useQuery({
    queryKey: ['workspace', workspaceId, 'access'],
    queryFn: async () => {
      const { data, error } = await workspacesService.getMe(workspaceId)
      if (error || !data) throw new Error(error ?? 'Failed to load workspace access')
      return data
    },
    enabled: !!workspaceId,
    staleTime: 5 * 60_000,
  })

  return {
    accessQuery,
    ...supportPermissionFlags(accessQuery.data),
  }
}
