import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { automationService } from '@/lib/services/automationService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type {
  CreateWorkspaceSkillRequest,
  SkillCatalogResponse,
  UpdateWorkspaceSkillRequest,
} from '@/lib/pmTypes';

export function useAutomationSkillCatalog(wsId: string, enabled = true) {
  return useQuery<SkillCatalogResponse>({
    queryKey: queryKeys.automation.skillCatalog(wsId),
    queryFn: async () => unwrap(await automationService.listSkillCatalog(wsId)),
    enabled: !!wsId && enabled,
    staleTime: 60_000,
  });
}

export function useCreateWorkspaceSkill(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (data: CreateWorkspaceSkillRequest) =>
      unwrap(await automationService.createSkill(wsId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.skillCatalog(wsId) });
    },
  });
}

export function useImportWorkspaceSkill(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ file, sourceRuntime }: { file: File; sourceRuntime?: string }) =>
      unwrap(await automationService.importSkill(wsId, file, sourceRuntime)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.skillCatalog(wsId) });
    },
  });
}

export function useUpdateWorkspaceSkill(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ skillId, data }: { skillId: string; data: UpdateWorkspaceSkillRequest }) =>
      unwrap(await automationService.updateSkill(wsId, skillId, data)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.skillCatalog(wsId) });
    },
  });
}

export function useDeleteWorkspaceSkill(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (skillId: string) =>
      unwrap(await automationService.deleteSkill(wsId, skillId)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.automation.skillCatalog(wsId) });
    },
  });
}
