import { useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import { workspacesService } from '@/lib/services/workspacesService'

export function useWorkspaces() {
  return useQuery({
    queryKey: queryKeys.workspaces.all(),
    queryFn: async () => unwrap(await workspacesService.list()),
    staleTime: 60_000,
  })
}
