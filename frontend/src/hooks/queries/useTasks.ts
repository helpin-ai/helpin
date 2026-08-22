import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmTaskService } from '@/lib/services/pmTaskService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { CreateTaskRequest, UpdateTaskRequest, TaskUserLinkRequest, TaskLabelLinkRequest } from '@/lib/pmTypes'

interface TaskQueryFilters {
  page?: number
  per_page?: number
  search?: string
  team_id?: string
  epic_id?: string
  sprint_id?: string
  workflow_id?: string
  state_id?: string
  state_type?: string
  task_type?: string
  owner_member_ids?: string
  requester_member_id?: string
  label_id?: string
  priority?: string
  severity?: string
  blocked?: string
  blocking?: string
  archived?: boolean
  contact_id?: string
  company_id?: string
  company_rollup_id?: string
  deal_id?: string
  include_contacts?: boolean
  include_companies?: boolean
  include_deals?: boolean
}

type TaskFilters = TaskQueryFilters

export function useTasks(wsId: string, filters?: TaskFilters, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: queryKeys.pm.tasks(wsId, filters as Record<string, unknown>),
    queryFn: async () => unwrap(await pmTaskService.list(wsId, filters)),
    enabled: !!wsId && (options?.enabled ?? true),
  })
}

export function useTask(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.pm.task(wsId, id),
    queryFn: async () => unwrap(await pmTaskService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useTaskByDisplayId(wsId: string, displayId: number) {
  return useQuery({
    queryKey: queryKeys.pm.taskByDisplayId(wsId, String(displayId)),
    queryFn: async () => unwrap(await pmTaskService.getByDisplayId(wsId, displayId)),
    enabled: !!wsId && !!displayId,
  })
}

export function useTaskActivity(wsId: string, taskId: string, page = 1, perPage = 50) {
  return useQuery({
    queryKey: [...queryKeys.pm.taskActivity(wsId, taskId), page],
    queryFn: async () => unwrap(await pmTaskService.listActivity(wsId, taskId, page, perPage)),
    enabled: !!wsId && !!taskId,
  })
}

export function useCreateTask(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateTaskRequest) => {
      const response = unwrap(await pmTaskService.create(data))
      return response.task
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'tasks'] })
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprintPlanning(wsId) })
      qc.invalidateQueries({
        queryKey: queryKeys.pm.sprintPreviewTasksRoot(wsId),
      })
    },
  })
}

export function useUpdateTask(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateTaskRequest & { id: string }) =>
      unwrap(await pmTaskService.update(wsId, id, data)),
    onSuccess: (result, { id }) => {
      qc.setQueryData(queryKeys.pm.task(wsId, id), result)
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'tasks'] })
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprintPlanning(wsId) })
      qc.invalidateQueries({
        queryKey: queryKeys.pm.sprintPreviewTasksRoot(wsId),
      })
    },
  })
}

export function useDeleteTask(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmTaskService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'tasks'] })
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
      qc.invalidateQueries({ queryKey: queryKeys.pm.sprintPlanning(wsId) })
      qc.invalidateQueries({
        queryKey: queryKeys.pm.sprintPreviewTasksRoot(wsId),
      })
    },
  })
}

export function useMoveTask(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: { id: string; state_id: string; position: number }) =>
      unwrap(await pmTaskService.move(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, id) })
    },
  })
}

export function useReorderTask(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, position }: { id: string; position: number }) =>
      unwrap(await pmTaskService.reorder(wsId, id, { position })),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
    },
  })
}

export function useAddTaskOwner(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ taskId, ...payload }: TaskUserLinkRequest & { taskId: string }) =>
      unwrap(await pmTaskService.addOwner(wsId, taskId, payload)),
    onSuccess: (_, { taskId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) })
    },
  })
}

export function useRemoveTaskOwner(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ taskId, userId }: { taskId: string; userId: string }) =>
      unwrap(await pmTaskService.removeOwner(wsId, taskId, userId)),
    onSuccess: (_, { taskId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) })
    },
  })
}

export function useAddTaskLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ taskId, ...payload }: TaskLabelLinkRequest & { taskId: string }) =>
      unwrap(await pmTaskService.addLabel(wsId, taskId, payload)),
    onSuccess: (_, { taskId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) })
    },
  })
}

export function useRemoveTaskLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ taskId, labelId }: { taskId: string; labelId: string }) =>
      unwrap(await pmTaskService.removeLabel(wsId, taskId, labelId)),
    onSuccess: (_, { taskId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) })
    },
  })
}

export function useSyncTaskLabels(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({
      taskId,
      currentIds,
      nextIds,
    }: {
      taskId: string
      currentIds: string[]
      nextIds: string[]
    }) => {
      await pmTaskService.syncLabels(wsId, taskId, currentIds, nextIds)
    },
    onSuccess: (_, { taskId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) })
    },
  })
}

export function useAddTaskFollower(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ taskId, ...payload }: TaskUserLinkRequest & { taskId: string }) =>
      unwrap(await pmTaskService.addFollower(wsId, taskId, payload)),
    onSuccess: (_, { taskId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) })
    },
  })
}

export function useRemoveTaskFollower(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ taskId, userId }: { taskId: string; userId?: string }) =>
      unwrap(await pmTaskService.removeFollower(wsId, taskId, userId)),
    onSuccess: (_, { taskId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.task(wsId, taskId) })
    },
  })
}
