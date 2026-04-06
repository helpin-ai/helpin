import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { workspacesService } from '@/lib/services/workspacesService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'

export function useWorkspaces(organizationId?: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.all(organizationId),
    queryFn: async () => unwrap(await workspacesService.list(organizationId)),
    staleTime: 60_000,
  })
}

export function useWorkspaceBySlug(slug: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.bySlug(slug),
    queryFn: async () => unwrap(await workspacesService.getBySlug(slug)),
    enabled: !!slug,
    staleTime: 60_000,
  })
}

export function useWorkspaceMembers(wsId: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.members(wsId),
    queryFn: async () => unwrap(await workspacesService.listMembers(wsId)),
    enabled: !!wsId,
  })
}

export function useAssignableMembers(wsId: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.assignableMembers(wsId),
    queryFn: async () => unwrap(await workspacesService.listAssignableMembers(wsId)),
    enabled: !!wsId,
  })
}

export function useCreateWorkspace() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: { name: string; slug: string; workspace_key: string; organization_id: string; description?: string; website_url?: string; timezone?: string }) =>
      unwrap(await workspacesService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
    },
  })
}

export function useUpdateWorkspace() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ wsId, ...data }: { wsId: string; name?: string; slug?: string; website_url?: string }) =>
      unwrap(await workspacesService.update(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
    },
  })
}

export function useDeleteWorkspace() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (wsId: string) => unwrap(await workspacesService.delete(wsId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
    },
  })
}
