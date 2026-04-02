import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmSprintService } from '@/lib/services/pmSprintService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateSprintRequest, SprintPlanningFilters, UpdateSprintRequest } from '@/lib/pmTypes'

interface SprintFilters {
  team_id?: string
  status?: string
  archived?: boolean
}

export function useSprints(wsId: string, filters?: SprintFilters) {
  return useQuery({
    queryKey: [...queryKeys.pm.sprints(wsId), filters],
    queryFn: async () => unwrap(await pmSprintService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useSprint(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.pm.sprint(wsId, id),
    queryFn: async () => unwrap(await pmSprintService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useSprintPlanningWorkspace(wsId: string, filters?: SprintPlanningFilters) {
  return useQuery({
    queryKey: queryKeys.pm.sprintPlanning(wsId, filters as Record<string, unknown> | undefined),
    queryFn: async () => unwrap(await pmSprintService.planningWorkspace(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useSprintTasks(wsId: string, sprintId: string) {
  return useQuery({
    queryKey: queryKeys.pm.sprintTasks(wsId, sprintId),
    queryFn: async () => unwrap(await pmSprintService.listTasks(wsId, sprintId)),
    enabled: !!wsId && !!sprintId,
  })
}

export function useCreateSprint(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateSprintRequest) => unwrap(await pmSprintService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprints(wsId) })
    },
  })
}

export function useUpdateSprint(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateSprintRequest & { id: string }) =>
      unwrap(await pmSprintService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprints(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprintPlanning(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprint(wsId, id) })
    },
  })
}

export function useDeleteSprint(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmSprintService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprints(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprintPlanning(wsId) })
    },
  })
}
