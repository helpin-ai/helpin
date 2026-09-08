import { api } from '@/lib/api';
import type { CRMSignalDismissalReason, CRMSuggestion } from '@/lib/crmTypes';
import type { CRMSituationCategory, CRMSituationHistory, CRMSituationList } from '@/lib/crmSituationTypes';

export interface SituationListFilters {
  scope?: 'mine' | 'my_teams' | 'unassigned' | 'all';
  state?: 'needs_attention' | 'open' | 'waiting' | 'paused' | 'closed' | 'all';
  category?: CRMSituationCategory | 'all';
  q?: string;
  filter?: string;
  page?: number;
}
function query(ws: string, values: Record<string, string | number | undefined> = {}) {
  const search = new URLSearchParams({ workspace_id: ws });
  for (const [key, value] of Object.entries(values)) if (value !== undefined && value !== '') search.set(key, String(value));
  return `?${search}`;
}

// Lifecycle writes and detail use the same service/query keys as Playbooks.
export const crmSituationService = {
  list: (ws: string, filters: SituationListFilters) => api.get<CRMSituationList>(`/crm/situations${query(ws, { ...filters, page_size: 25 })}`),
  history: (ws: string, id: string, before?: number) => api.get<CRMSituationHistory>(`/crm/situations/${id}/history${query(ws, { before_revision: before, limit: 50 })}`),
  accept: (ws: string, id: string, action: string, revision: string, edits?: Record<string, unknown>) => api.post<CRMSuggestion>(`/crm/situations/${id}/actions/${action}/accept${query(ws)}`, { revision, ...(edits ? { edits } : {}) }),
  dismiss: (ws: string, id: string, action: string, revision: string, reason: CRMSignalDismissalReason) => api.post<CRMSuggestion>(`/crm/situations/${id}/actions/${action}/dismiss${query(ws)}`, { revision, reason }),
};
