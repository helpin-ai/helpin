import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { pmSprintService } from '@/lib/services/pmSprintService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateSprintRequest, PaginatedResponse, SprintPlanningFilters, SprintPlanningTaskPreview, UpdateSprintRequest } from '@/lib/pmTypes'

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
    placeholderData: (previous) => previous,
  })
}

export function useSprintTasks(wsId: string, sprintId: string) {
  return useQuery({
    queryKey: queryKeys.pm.sprintTasks(wsId, sprintId),
    queryFn: async () => unwrap(await pmSprintService.listTasks(wsId, sprintId)),
    enabled: !!wsId && !!sprintId,
  })
}

export function useInfiniteSprintPreviewTasks(
  wsId: string,
  sprintId: string,
  initialTasks: SprintPlanningTaskPreview[],
  total: number,
  perPage = 20,
) {
  const totalPages = total > 0 ? Math.ceil(total / perPage) : 0
  return useInfiniteQuery({
    queryKey: queryKeys.pm.sprintPreviewTasks(wsId, sprintId),
    queryFn: async ({ pageParam }) =>
      unwrap(await pmSprintService.listPreviewTasks(wsId, sprintId, { page: pageParam as number, per_page: perPage })),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => lastPage.page < lastPage.total_pages ? lastPage.page + 1 : undefined,
    enabled: !!wsId && !!sprintId,
    staleTime: 30_000,
    initialData: {
      pageParams: [1],
      pages: [{
        data: initialTasks,
        total,
        page: 1,
        per_page: perPage,
        total_pages: totalPages,
      } satisfies PaginatedResponse<SprintPlanningTaskPreview[]>],
    },
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
