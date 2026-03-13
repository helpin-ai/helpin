import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { settingsService } from '@/lib/services/settingsService'
import { queryKeys } from '@/lib/queryKeys'
import { unwrap } from '@/lib/queryUtils'
import type { AutomationInventoryResponse, TeamEstimateSettings, TeamFieldVisibility } from '@/lib/types'

export function useWorkspaceSettings(wsId: string) {
  return useQuery({
    queryKey: queryKeys.workspaces.settings(wsId),
    queryFn: async () => unwrap(await settingsService.getAll(wsId)),
    enabled: !!wsId,
    staleTime: 60_000,
  })
}

export function useAIAutomations(wsId: string) {
  return useQuery<AutomationInventoryResponse>({
    queryKey: queryKeys.workspaces.aiAutomations(wsId),
    queryFn: async () => unwrap(await settingsService.getAIAutomations(wsId)),
    enabled: !!wsId,
    staleTime: 60_000,
  })
}

export function useTeamEstimateSettings(wsId: string) {
  const { data } = useWorkspaceSettings(wsId)
  return data?.team_estimate_settings ?? []
}

export function useTeamEstimateSettingsForTeam(wsId: string, teamId: string | undefined | null) {
  const settings = useTeamEstimateSettings(wsId)
  if (!teamId) return null
  return settings.find((s) => s.team_id === teamId) ?? null
}

const DEFAULT_VISIBILITY: Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'> = {
  priority: true,
  story_type: true,
  severity: true,
  labels: true,
  epic: true,
  sprint: true,
  estimate: true,
  due_date: true,
  blocked: true,
  delivery: true,
  dev_history: true,
}

type VisibilityField = keyof typeof DEFAULT_VISIBILITY

export function useTeamFieldVisibility(wsId: string) {
  const { data } = useWorkspaceSettings(wsId)
  return data?.team_field_visibility ?? []
}

export function useTeamFieldVisibilityForTeam(wsId: string, teamId: string | undefined | null) {
  const settings = useTeamFieldVisibility(wsId)
  if (!teamId) return DEFAULT_VISIBILITY
  return settings.find((s) => s.team_id === teamId) ?? DEFAULT_VISIBILITY
}

export function useIsFieldVisible(wsId: string, teamId: string | undefined | null, field: VisibilityField) {
  const vis = useTeamFieldVisibilityForTeam(wsId, teamId)
  return vis[field]
}

export function useUpdateTeamEstimateSettings(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ teamId, settings }: { teamId: string; settings: Partial<TeamEstimateSettings> }) =>
      unwrap(await settingsService.updateTeamEstimateSettings(wsId, teamId, settings)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.workspaces.settings(wsId) })
    },
  })
}

export function useUpdateTeamFieldVisibility(wsId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async ({ teamId, settings }: { teamId: string; settings: Partial<TeamFieldVisibility> }) =>
      unwrap(await settingsService.updateTeamFieldVisibility(wsId, teamId, settings)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.workspaces.settings(wsId) })
    },
  })
}

export function useInvalidateSettings(wsId: string) {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ queryKey: queryKeys.workspaces.settings(wsId) })
}
