import { api } from '../api';
import type { WorkspaceBillingSummary } from '../types';
import type {
  CheckoutRequest,
  CheckoutResponse,
  Invoice,
  LinkPaymentMethodRequest,
  OrgBillingResponse,
  PaymentMethod,
  PortalResponse,
  SetBillingOwnerRequest,
  SetupIntentResponse,
  UpdateCardRequest,
  UsageMode,
  UsageResponse,
} from '../billingTypes';

export const billingService = {
  // ── Organization-scoped ──
  getOrgBilling: (orgId: string) =>
    api.get<OrgBillingResponse>(`/organizations/${orgId}/billing`),

  listCards: (orgId: string) =>
    api.get<PaymentMethod[]>(`/organizations/${orgId}/billing/cards`),

  createSetupIntent: (orgId: string) =>
    api.post<SetupIntentResponse>(`/organizations/${orgId}/billing/cards/setup-intent`, {}),

  updateCard: (orgId: string, cardId: string, data: UpdateCardRequest) =>
    api.put<PaymentMethod>(`/organizations/${orgId}/billing/cards/${cardId}`, data),

  deleteCard: (orgId: string, cardId: string) =>
    api.del(`/organizations/${orgId}/billing/cards/${cardId}`),

  listInvoices: (orgId: string) =>
    api.get<Invoice[]>(`/organizations/${orgId}/billing/invoices`),

  // ── Workspace-scoped ──
  getWorkspaceBilling: (wsId: string) =>
    api.get<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing`),

  getUsage: (wsId: string, period: string, mode: UsageMode) =>
    api.get<UsageResponse>(
      `/workspaces/${wsId}/billing/usage?period=${encodeURIComponent(period)}&mode=${mode}`,
    ),

  linkPaymentMethod: (wsId: string, data: LinkPaymentMethodRequest) =>
    api.put(`/workspaces/${wsId}/billing/payment-method`, data),

  setBillingOwner: (wsId: string, data: SetBillingOwnerRequest) =>
    api.put(`/workspaces/${wsId}/billing/owner`, data),

  checkout: (wsId: string, data: CheckoutRequest) =>
    api.post<CheckoutResponse>(`/workspaces/${wsId}/billing/checkout`, data),

  portal: (wsId: string, returnUrl?: string) =>
    api.post<PortalResponse>(`/workspaces/${wsId}/billing/portal`, returnUrl ? { return_url: returnUrl } : {}),

  setOnDemand: (wsId: string, enabled: boolean) =>
    api.put(`/workspaces/${wsId}/billing/on-demand`, { enabled }),
};
