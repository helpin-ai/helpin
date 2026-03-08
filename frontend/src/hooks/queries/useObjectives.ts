import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmObjectiveService } from '@/lib/services/pmObjectiveService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type {
  CreateObjectiveRequest,
  UpdateObjectiveRequest,
  CreateKeyResultRequest,
  UpdateKeyResultRequest,
} from '@/lib/pmTypes'

interface ObjectiveFilters {
  team_id?: string
  label_id?: string
  objective_type?: string
  state?: string
  archived?: boolean
}

export function useObjectives(wsId: string, filters?: ObjectiveFilters) {
  return useQuery({
    queryKey: [...queryKeys.pm.objectives(wsId), filters],
    queryFn: async () => unwrap(await pmObjectiveService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useObjective(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.pm.objective(wsId, id),
    queryFn: async () => unwrap(await pmObjectiveService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useCreateObjective(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateObjectiveRequest) => unwrap(await pmObjectiveService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objectives(wsId) })
    },
  })
}

export function useUpdateObjective(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateObjectiveRequest & { id: string }) =>
      unwrap(await pmObjectiveService.update(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objectives(wsId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, id) })
    },
  })
}

export function useDeleteObjective(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmObjectiveService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objectives(wsId) })
    },
  })
}

export function useAddObjectiveTeam(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ objectiveId, teamId }: { objectiveId: string; teamId: string }) =>
      unwrap(await pmObjectiveService.addTeam(wsId, objectiveId, teamId)),
    onSuccess: (_, { objectiveId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, objectiveId) })
    },
  })
}

export function useRemoveObjectiveTeam(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ objectiveId, teamId }: { objectiveId: string; teamId: string }) =>
      unwrap(await pmObjectiveService.removeTeam(wsId, objectiveId, teamId)),
    onSuccess: (_, { objectiveId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, objectiveId) })
    },
  })
}

export function useAddObjectiveOwner(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ objectiveId, memberId }: { objectiveId: string; memberId: string }) =>
      unwrap(await pmObjectiveService.addOwner(wsId, objectiveId, memberId)),
    onSuccess: (_, { objectiveId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, objectiveId) })
    },
  })
}

export function useRemoveObjectiveOwner(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ objectiveId, memberId }: { objectiveId: string; memberId: string }) =>
      unwrap(await pmObjectiveService.removeOwner(wsId, objectiveId, memberId)),
    onSuccess: (_, { objectiveId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, objectiveId) })
    },
  })
}

export function useAddObjectiveEpic(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ objectiveId, epicId }: { objectiveId: string; epicId: string }) =>
      unwrap(await pmObjectiveService.addEpic(wsId, objectiveId, epicId)),
    onSuccess: (_, { objectiveId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, objectiveId) })
    },
  })
}

export function useRemoveObjectiveEpic(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ objectiveId, epicId }: { objectiveId: string; epicId: string }) =>
      unwrap(await pmObjectiveService.removeEpic(wsId, objectiveId, epicId)),
    onSuccess: (_, { objectiveId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, objectiveId) })
    },
  })
}

export function useCreateKeyResult(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ objectiveId, ...data }: CreateKeyResultRequest & { objectiveId: string }) =>
      unwrap(await pmObjectiveService.createKeyResult(wsId, objectiveId, data)),
    onSuccess: (_, { objectiveId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.objective(wsId, objectiveId) })
      qc.invalidateQueries({ queryKey: queryKeys.pm.objectives(wsId) })
    },
  })
}

export function useUpdateKeyResult(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateKeyResultRequest & { id: string }) =>
      unwrap(await pmObjectiveService.updateKeyResult(wsId, id, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'objectives'] })
    },
  })
}

export function useDeleteKeyResult(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmObjectiveService.deleteKeyResult(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'objectives'] })
    },
  })
}
