import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { crmPlaybookService as service, readPlaybookResponse as read, type PlaybookListFilters, type PlaybookParticipantFilters } from '@/lib/services/crmPlaybookService';
import type { ApplyCRMPlaybookRequest, CreateCRMPlaybookRequest, CRMPlaybookCommandRequest, CRMPlaybookMilestoneRequest } from '@/lib/crmPlaybookTypes';
import type { CRMSituationCommandRequest } from '@/lib/crmSituationTypes';

export function useCRMPlaybooks(ws: string, filters: PlaybookListFilters) {
  return useQuery({ queryKey: [...queryKeys.crm.playbooks(ws), 'list', filters], queryFn: async () => read(await service.list(ws, filters)), enabled: !!ws });
}
export function useCRMPlaybook(ws: string, id: string) {
  return useQuery({ queryKey: queryKeys.crm.playbook(ws, id), queryFn: async () => read(await service.get(ws, id)), enabled: !!ws && !!id });
}
export function useCRMPlaybookTemplates(ws: string) {
  return useQuery({ queryKey: [...queryKeys.crm.playbooks(ws), 'templates'], queryFn: async () => read(await service.templates(ws)), enabled: !!ws });
}
export function useCRMPlaybookParticipants(ws: string, id: string, filters: PlaybookParticipantFilters) {
  return useQuery({ queryKey: [...queryKeys.crm.playbook(ws, id), 'participants', filters], queryFn: async () => read(await service.participants(ws, id, filters)), enabled: !!ws && !!id });
}
export function useCRMPlaybookPreview(ws: string, id: string, revision: number, versionId: string | undefined, page: number) {
  return useQuery({ queryKey: [...queryKeys.crm.playbook(ws, id), 'preview', revision, versionId, page], queryFn: async () => read(await service.preview(ws, id, revision, versionId, page)), enabled: !!ws && !!id, retry: false });
}
export function useCRMPlaybookHistory(ws: string, id: string) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.playbook(ws, id), 'history'],
    queryFn: async ({ pageParam }) => read(await service.history(ws, id, pageParam)),
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (last) => last.next_before_revision ?? undefined,
    enabled: !!ws && !!id,
  });
}
export function useCRMPlaybookVersion(ws: string, id: string, versionId?: string) {
  return useQuery({
    queryKey: [...queryKeys.crm.playbook(ws, id), 'version', versionId],
    queryFn: async () => {
      let before: number | undefined;
      do {
        const page = read(await service.versions(ws, id, before));
        const version = page.data.find((entry) => entry.id === versionId);
        if (version) return version;
        if (page.next_before_version == null) break;
        if (before !== undefined && page.next_before_version >= before) throw new Error('Could not load the published version.');
        before = page.next_before_version;
      } while (before !== undefined);
      throw new Error('The published version for this signal could not be found.');
    },
    enabled: !!ws && !!id && !!versionId,
    staleTime: Infinity,
  });
}
export function useCRMPlaybookSignal(ws: string, id: string) {
  return useQuery({ queryKey: queryKeys.crm.situation(ws, id), queryFn: async () => read(await service.signal(ws, id)), enabled: !!ws && !!id, refetchInterval: 15_000 });
}

type PlaybookWrite =
  | { kind: 'create'; body: CreateCRMPlaybookRequest }
  | { kind: 'command'; id: string; body: CRMPlaybookCommandRequest }
  | { kind: 'apply'; id: string; body: ApplyCRMPlaybookRequest }
  | { kind: 'milestone'; id: string; signalId: string; body: CRMPlaybookMilestoneRequest }
  | { kind: 'signal'; signalId: string; body: CRMSituationCommandRequest };

export function useCRMPlaybookWrite(ws: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (request: PlaybookWrite) => {
      switch (request.kind) {
        case 'create': return read(await service.create(ws, request.body));
        case 'command': return read(await service.command(ws, request.id, request.body));
        case 'apply': return read(await service.apply(ws, request.id, request.body));
        case 'milestone': return read(await service.milestone(ws, request.id, request.signalId, request.body));
        case 'signal': return read(await service.signalCommand(ws, request.signalId, request.body));
      }
    },
    retry: false,
    // Command replays contain an original receipt, not current state. Always refetch.
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: queryKeys.crm.playbooks(ws) }),
        client.invalidateQueries({ queryKey: queryKeys.crm.situations(ws) }),
      ]);
    },
  });
}
