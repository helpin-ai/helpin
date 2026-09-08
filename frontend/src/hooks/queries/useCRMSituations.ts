import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { crmSituationService as service, type SituationListFilters } from '@/lib/services/crmSituationService';
import { readPlaybookResponse as read } from '@/lib/services/crmPlaybookService';
import type { CRMSignalDismissalReason } from '@/lib/crmTypes';
import { crmSignalInboxService as inbox } from '@/lib/services/crmSignalInboxService';
import type { SignalsSearch } from '@/lib/crmSignalInboxQueryBuilder';

export function useCRMSignalInbox(ws: string, search: SignalsSearch) {
  return useQuery({ queryKey: [...queryKeys.crm.situations(ws), 'inbox', search], queryFn: async () => read(await inbox.list(ws, search)), enabled: !!ws });
}
export function useCRMInboxRecommendation(ws: string, id: string) {
  return useQuery({ queryKey: [...queryKeys.crm.situations(ws), 'recommendation', id], queryFn: async () => read(await inbox.recommendation(ws, id)), enabled: !!ws && !!id });
}
export function useCRMInboxSignalGroup(ws: string, id: string) {
  return useQuery({ queryKey: [...queryKeys.crm.situations(ws), 'evidence-group', id], queryFn: async () => read(await inbox.evidence(ws, id)), enabled: !!ws && !!id });
}

export function useCRMSituations(ws: string, filters: SituationListFilters) {
  return useQuery({ queryKey: [...queryKeys.crm.situations(ws), 'list', filters], queryFn: async () => read(await service.list(ws, filters)), enabled: !!ws });
}
export function useCRMSituationHistory(ws: string, id: string) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.crm.situation(ws, id), 'history'],
    queryFn: async ({ pageParam }) => read(await service.history(ws, id, pageParam)),
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (last) => last.next_before_revision ?? undefined,
    enabled: !!ws && !!id,
  });
}
export function useCRMSituationDecision(ws: string, id?: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (request: { actionId: string; revision: string } & ({ kind: 'accept'; edits?: Record<string, unknown> } | { kind: 'dismiss'; reason: CRMSignalDismissalReason })) =>
      read(await (id ? request.kind === 'accept' ? service.accept(ws, id, request.actionId, request.revision, request.edits) : service.dismiss(ws, id, request.actionId, request.revision, request.reason) : request.kind === 'accept' ? inbox.accept(ws, request.actionId, request.revision, request.edits) : inbox.dismiss(ws, request.actionId, request.revision, request.reason))),
    retry: false,
    // A timeout can occur after the executor has started. Refetch, never replay an approval.
    onSettled: async () => { await Promise.all([
      client.invalidateQueries({ queryKey: queryKeys.crm.situations(ws) }),
      client.invalidateQueries({ queryKey: queryKeys.crm.playbooks(ws) }),
      client.invalidateQueries({ queryKey: ['crm', ws, 'suggestions'] }),
      client.invalidateQueries({ queryKey: queryKeys.crm.deals(ws) }),
    ]); },
  });
}
