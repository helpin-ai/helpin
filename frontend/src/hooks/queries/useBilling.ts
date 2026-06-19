import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { billingService } from '@/lib/services/billingService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type {
  CheckoutRequest,
  LinkPaymentMethodRequest,
  SetBillingOwnerRequest,
  UpdateCardRequest,
  UsageMode,
} from '@/lib/billingTypes';

export function useOrgBilling(orgId?: string) {
  return useQuery({
    queryKey: queryKeys.billing.org(orgId ?? ''),
    queryFn: async () => unwrap(await billingService.getOrgBilling(orgId!)),
    enabled: !!orgId,
    staleTime: 30_000,
  });
}

export function useWorkspaceBilling(wsId?: string) {
  return useQuery({
    queryKey: queryKeys.billing.workspace(wsId ?? ''),
    queryFn: async () => unwrap(await billingService.getWorkspaceBilling(wsId!)),
    enabled: !!wsId,
    staleTime: 60_000,
  });
}

export function useBillingCards(orgId?: string) {
  return useQuery({
    queryKey: queryKeys.billing.cards(orgId ?? ''),
    queryFn: async () => unwrap(await billingService.listCards(orgId!)),
    enabled: !!orgId,
    staleTime: 30_000,
  });
}

export function useBillingInvoices(orgId?: string) {
  return useQuery({
    queryKey: queryKeys.billing.invoices(orgId ?? ''),
    queryFn: async () => unwrap(await billingService.listInvoices(orgId!)),
    enabled: !!orgId,
    staleTime: 60_000,
  });
}

export function useWorkspaceUsage(wsId?: string, period?: string, mode: UsageMode = 'daily') {
  return useQuery({
    queryKey: queryKeys.billing.usage(wsId ?? '', period ?? '', mode),
    queryFn: async () => unwrap(await billingService.getUsage(wsId!, period!, mode)),
    enabled: !!wsId && !!period,
    staleTime: 60_000,
  });
}

function useInvalidateOrgBilling(orgId?: string) {
  const qc = useQueryClient();
  return () => {
    if (!orgId) return;
    qc.invalidateQueries({ queryKey: queryKeys.billing.org(orgId) });
    qc.invalidateQueries({ queryKey: queryKeys.billing.cards(orgId) });
  };
}

export function useCreateSetupIntent(orgId?: string) {
  return useMutation({
    mutationFn: async () => unwrap(await billingService.createSetupIntent(orgId!)),
  });
}

export function useUpdateCard(orgId?: string) {
  const invalidate = useInvalidateOrgBilling(orgId);
  return useMutation({
    mutationFn: async ({ cardId, data }: { cardId: string; data: UpdateCardRequest }) =>
      unwrap(await billingService.updateCard(orgId!, cardId, data)),
    onSuccess: invalidate,
  });
}

export function useDeleteCard(orgId?: string) {
  const invalidate = useInvalidateOrgBilling(orgId);
  return useMutation({
    mutationFn: async (cardId: string) => {
      const res = await billingService.deleteCard(orgId!, cardId);
      if (res.error) throw new Error(res.error);
      return res.data;
    },
    onSuccess: invalidate,
  });
}

export function useLinkPaymentMethod(orgId?: string) {
  const invalidate = useInvalidateOrgBilling(orgId);
  return useMutation({
    mutationFn: async ({ wsId, data }: { wsId: string; data: LinkPaymentMethodRequest }) => {
      const res = await billingService.linkPaymentMethod(wsId, data);
      if (res.error) throw new Error(res.error);
      return res.data;
    },
    onSuccess: invalidate,
  });
}

export function useSetBillingOwner(orgId?: string) {
  const invalidate = useInvalidateOrgBilling(orgId);
  return useMutation({
    mutationFn: async ({ wsId, data }: { wsId: string; data: SetBillingOwnerRequest }) => {
      const res = await billingService.setBillingOwner(wsId, data);
      if (res.error) throw new Error(res.error);
      return res.data;
    },
    onSuccess: invalidate,
  });
}

export function useSetOnDemand(orgId?: string) {
  const invalidate = useInvalidateOrgBilling(orgId);
  return useMutation({
    mutationFn: async ({ wsId, enabled }: { wsId: string; enabled: boolean }) => {
      const res = await billingService.setOnDemand(wsId, enabled);
      if (res.error) throw new Error(res.error);
      return res.data;
    },
    onSuccess: invalidate,
  });
}

/**
 * Checkout hook. Supports two call styles:
 *  - org page:      useBillingCheckout(); mutate({ wsId, data })
 *  - settings page: useBillingCheckout(wsId); mutateAsync({ plan, interval, return_url })
 */
export function useBillingCheckout(boundWsId?: string) {
  return useMutation({
    mutationFn: async (
      vars: { wsId: string; data: CheckoutRequest } | CheckoutRequest,
    ) => {
      const wsId = 'wsId' in vars ? vars.wsId : boundWsId!;
      const data = 'data' in vars ? vars.data : vars;
      return unwrap(await billingService.checkout(wsId, data));
    },
  });
}

/**
 * Portal hook. Supports:
 *  - org page:      useBillingPortal(); mutate(wsId)
 *  - settings page: useBillingPortal(wsId); mutateAsync(returnUrl?)
 */
export function useBillingPortal(boundWsId?: string) {
  return useMutation({
    mutationFn: async (arg?: string) => {
      // If a wsId was bound, treat the arg as a return URL; otherwise the arg is the wsId.
      const wsId = boundWsId ?? arg!;
      const returnUrl = boundWsId ? arg : undefined;
      return unwrap(await billingService.portal(wsId, returnUrl));
    },
  });
}

/** Alias used by the workspace settings billing page. */
export function useSetBillingOnDemand(wsId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (enabled: boolean) => {
      const res = await billingService.setOnDemand(wsId!, enabled);
      if (res.error) throw new Error(res.error);
      return res.data;
    },
    onSuccess: () => {
      if (wsId) qc.invalidateQueries({ queryKey: queryKeys.billing.workspace(wsId) });
    },
  });
}
