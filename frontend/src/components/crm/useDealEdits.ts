import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { crmDealService } from '@/lib/services/crmService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrapRequired } from '@/lib/queryUtils';
import type {
  CRMDeal,
  CRMPaginatedResponse,
  CRMPipelineStage,
  UpdateCRMDealRequest,
} from '@/lib/crmTypes';

const EMPTY_PATCHES: Record<string, Partial<CRMDeal>> = {};

/** Shared board/list edits: rebase pending patches over current data; rollback only the failed record. */
export function useDealEdits(
  workspaceId: string,
  pipelineId: string | undefined,
  deals: CRMDeal[],
  stages: CRMPipelineStage[],
  onChanged?: (deal: CRMDeal) => void,
) {
  const client = useQueryClient();
  const scope = useMemo(
    () => ({ workspaceId, pipelineId, pending: new Set<string>() }),
    [workspaceId, pipelineId],
  );
  const [state, setState] = useState<{
    scope: object;
    patches: Record<string, Partial<CRMDeal>>;
  }>({ scope, patches: {} });
  const activeScope = useRef<object | null>(null);
  useEffect(() => {
    activeScope.current = scope;
    return () => {
      activeScope.current = null;
    };
  }, [scope]);
  const patches = state.scope === scope ? state.patches : EMPTY_PATCHES;
  const localDeals = useMemo(
    () =>
      deals.map((deal) =>
        patches[deal.id] ? { ...deal, ...patches[deal.id] } : deal,
      ),
    [deals, patches],
  );
  const updateDealField = useCallback(
    async (id: string, patch: Partial<CRMDeal>) => {
      if (scope.pending.has(id)) return false;
      const deal = deals.find((d) => d.id === id);
      if (!deal) return false;
      const stage = patch.stage_id
        ? stages.find((s) => s.id === patch.stage_id)
        : undefined;
      if (patch.stage_id && !stage) {
        toast.error(
          'This stage is no longer available. Refresh and try again.',
        );
        return false;
      }
      const request: UpdateCRMDealRequest = {};
      for (const key of [
        'name',
        'stage_id',
        'amount',
        'currency',
        'close_date',
        'owner_member_id',
        'probability',
      ] as const) {
        if (patch[key] !== undefined)
          Object.assign(request, { [key]: patch[key] });
      }
      scope.pending.add(id);
      setState((previous) => ({
        scope,
        patches: {
          ...(previous.scope === scope ? previous.patches : {}),
          [id]: { ...patch, ...(stage ? { stage } : {}) },
        },
      }));
      try {
        const updated = unwrapRequired(
          await crmDealService.update(workspaceId, id, request),
          'Save deal',
        );
        client.setQueriesData<CRMPaginatedResponse<CRMDeal[]>>(
          { queryKey: queryKeys.crm.deals(workspaceId) },
          (current) => {
            if (!current || !Array.isArray(current.data)) return current;
            return {
              ...current,
              data: current.data.map((item) =>
                item.id === id ? { ...item, ...updated } : item,
              ),
            };
          },
        );
        client.setQueryData(queryKeys.crm.deal(workspaceId, id), updated);
        if (activeScope.current === scope)
          void Promise.resolve(onChanged?.(updated)).catch(() => undefined);
        return true;
      } catch (error) {
        if (activeScope.current === scope)
          toast.error(
            error instanceof Error ? error.message : 'Could not save deal',
          );
        return false;
      } finally {
        scope.pending.delete(id);
        if (activeScope.current === scope)
          setState((previous) => {
            if (previous.scope !== scope) return previous;
            const next = { ...previous.patches };
            delete next[id];
            return { scope, patches: next };
          });
      }
    },
    [client, deals, onChanged, scope, stages, workspaceId],
  );
  return {
    localDeals,
    updateDealField,
    isPending: (id: string) => !!patches[id],
  };
}
