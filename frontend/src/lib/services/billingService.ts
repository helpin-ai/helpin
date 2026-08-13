import { api } from '../api';
import type { WorkspaceBillingSummary } from '../types';
import type {
  CheckoutRequest,
  CheckoutResponse,
  ConfirmCheckoutRequest,
  Invoice,
  LinkPaymentMethodRequest,
  OrgBillingResponse,
  PaymentMethod,
  PlanChangeRequest,
  PlanChangePreview,
  PortalResponse,
  BillingTestScenarioID,
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

  updateCard: (orgId: string, cardId: string, data: UpdateCardRequest) =>
    api.put<PaymentMethod>(`/organizations/${orgId}/billing/cards/${cardId}`, data),

  deleteCard: (orgId: string, cardId: string) =>
    api.del(`/organizations/${orgId}/billing/cards/${cardId}`),

  listInvoices: (orgId: string) =>
    api.get<Invoice[]>(`/organizations/${orgId}/billing/invoices`),

  // ── Workspace-scoped ──
  getWorkspaceBilling: (wsId: string) =>
    api.get<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing`),

  getUsage: (wsId: string, period: string, mode: UsageMode, start?: string, end?: string) => {
    const params = new URLSearchParams({ period, mode });
    if (start && end) {
      params.set('start', start);
      params.set('end', end);
    }
    return api.get<UsageResponse>(`/workspaces/${wsId}/billing/usage?${params.toString()}`);
  },

  linkPaymentMethod: (wsId: string, data: LinkPaymentMethodRequest) =>
    api.put<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing/payment-method`, data),

  checkout: (wsId: string, data: CheckoutRequest) =>
    api.post<CheckoutResponse>(`/workspaces/${wsId}/billing/checkout`, data),

  confirmCheckout: (wsId: string, data: ConfirmCheckoutRequest) =>
    api.post<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing/confirm-checkout`, data),

  changePlan: (wsId: string, data: PlanChangeRequest) =>
    api.post<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing/change-plan`, data),

  previewPlanChange: (wsId: string, data: PlanChangeRequest) =>
    api.post<PlanChangePreview>(`/workspaces/${wsId}/billing/preview-plan-change`, data),

  resumeSubscription: (wsId: string) =>
    api.post<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing/resume-subscription`, {}),

  portal: (wsId: string, returnUrl?: string) =>
    api.post<PortalResponse>(`/workspaces/${wsId}/billing/portal`, returnUrl ? { return_url: returnUrl } : {}),

  setExtraAIUsage: (wsId: string, enabled: boolean) =>
    api.put<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing/extra-usage`, { enabled }),

  applyTestScenario: (wsId: string, scenario: BillingTestScenarioID) =>
    api.post<WorkspaceBillingSummary>(`/workspaces/${wsId}/billing/test-scenario`, { scenario }),
};
