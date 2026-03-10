import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { pmStoryService } from '@/lib/services/pmStoryService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type {
  CreateStoryRequest,
  UpdateStoryRequest,
  StoryUserLinkRequest,
  StoryLabelLinkRequest,
} from '@/lib/pmTypes'

interface StoryFilters {
  page?: number
  per_page?: number
  team_id?: string
  epic_id?: string
  sprint_id?: string
  workflow_id?: string
  state_id?: string
  story_type?: string
  owner_member_id?: string
  requester_member_id?: string
  label_id?: string
  priority?: string
  severity?: string
  blocked?: string
  blocking?: string
  archived?: boolean
}

export function useStories(wsId: string, filters?: StoryFilters) {
  return useQuery({
    queryKey: queryKeys.pm.stories(wsId, filters as Record<string, unknown>),
    queryFn: async () => unwrap(await pmStoryService.list(wsId, filters)),
    enabled: !!wsId,
  })
}

export function useStory(wsId: string, id: string) {
  return useQuery({
    queryKey: queryKeys.pm.story(wsId, id),
    queryFn: async () => unwrap(await pmStoryService.get(wsId, id)),
    enabled: !!wsId && !!id,
  })
}

export function useStoryByDisplayId(wsId: string, displayId: number) {
  return useQuery({
    queryKey: queryKeys.pm.storyByDisplayId(wsId, String(displayId)),
    queryFn: async () => unwrap(await pmStoryService.getByDisplayId(wsId, displayId)),
    enabled: !!wsId && !!displayId,
  })
}

export function useStoryActivity(wsId: string, storyId: string, page = 1, perPage = 50) {
  return useQuery({
    queryKey: [...queryKeys.pm.storyActivity(wsId, storyId), page],
    queryFn: async () => unwrap(await pmStoryService.listActivity(wsId, storyId, page, perPage)),
    enabled: !!wsId && !!storyId,
  })
}

export function useCreateStory(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (data: CreateStoryRequest) => unwrap(await pmStoryService.create(data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'stories'] })
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
    },
  })
}

export function useUpdateStory(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: UpdateStoryRequest & { id: string }) =>
      unwrap(await pmStoryService.update(wsId, id, data)),
    onSuccess: (result, { id }) => {
      qc.setQueryData(queryKeys.pm.story(wsId, id), result)
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'stories'] })
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
    },
  })
}

export function useDeleteStory(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => unwrap(await pmStoryService.remove(wsId, id)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'stories'] })
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
    },
  })
}

export function useMoveStory(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, ...data }: { id: string; state_id: string; position: number }) =>
      unwrap(await pmStoryService.move(wsId, id, data)),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, id) })
    },
  })
}

export function useReorderStory(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, position }: { id: string; position: number }) =>
      unwrap(await pmStoryService.reorder(wsId, id, { position })),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pm', wsId, 'board'] })
    },
  })
}

export function useAddStoryOwner(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ storyId, ...payload }: StoryUserLinkRequest & { storyId: string }) =>
      unwrap(await pmStoryService.addOwner(wsId, storyId, payload)),
    onSuccess: (_, { storyId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) })
    },
  })
}

export function useRemoveStoryOwner(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ storyId, userId }: { storyId: string; userId: string }) =>
      unwrap(await pmStoryService.removeOwner(wsId, storyId, userId)),
    onSuccess: (_, { storyId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) })
    },
  })
}

export function useAddStoryLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ storyId, ...payload }: StoryLabelLinkRequest & { storyId: string }) =>
      unwrap(await pmStoryService.addLabel(wsId, storyId, payload)),
    onSuccess: (_, { storyId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) })
    },
  })
}

export function useRemoveStoryLabel(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ storyId, labelId }: { storyId: string; labelId: string }) =>
      unwrap(await pmStoryService.removeLabel(wsId, storyId, labelId)),
    onSuccess: (_, { storyId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) })
    },
  })
}

export function useSyncStoryLabels(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ storyId, currentIds, nextIds }: { storyId: string; currentIds: string[]; nextIds: string[] }) => {
      await pmStoryService.syncLabels(wsId, storyId, currentIds, nextIds)
    },
    onSuccess: (_, { storyId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) })
    },
  })
}

export function useAddStoryFollower(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ storyId, ...payload }: StoryUserLinkRequest & { storyId: string }) =>
      unwrap(await pmStoryService.addFollower(wsId, storyId, payload)),
    onSuccess: (_, { storyId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) })
    },
  })
}

export function useRemoveStoryFollower(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ storyId, userId }: { storyId: string; userId?: string }) =>
      unwrap(await pmStoryService.removeFollower(wsId, storyId, userId)),
    onSuccess: (_, { storyId }) => {
      qc.invalidateQueries({ queryKey: queryKeys.pm.story(wsId, storyId) })
    },
  })
}
