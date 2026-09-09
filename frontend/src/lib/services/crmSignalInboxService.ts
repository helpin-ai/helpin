import { api } from '@/lib/api';
import type { CRMSignalAccountStory, CRMSignalDismissalReason, CRMSuggestion } from '@/lib/crmTypes';
import type { CRMSignalInboxList, CRMInboxRecommendation } from '@/lib/crmSignalInboxTypes';
import { parseSignalsSearch, type SignalsSearch } from '@/lib/crmSignalInboxQueryBuilder';

function query(ws: string, values: Record<string, string | number | undefined> = {}) {
  const search = new URLSearchParams({ workspace_id: ws });
  for (const [key, value] of Object.entries(values)) if (value !== undefined && value !== '') search.set(key, String(value));
  return `?${search}`;
}
export const crmSignalInboxService = {
  list: (ws: string, input: SignalsSearch) => {
    const search = parseSignalsSearch({ ...input });
    return api.get<CRMSignalInboxList>(`/crm/signal-inbox${query(ws, {
      scope: search.scope || 'all', state: search.state || 'needs_attention', category: search.category || 'all',
      q: search.q?.trim(), sort: search.sort || 'priority', page: search.page || 1, page_size: 25,
    })}`);
  },
  evidence: (ws: string, id: string) => api.get<CRMSignalAccountStory>(`/crm/signal-inbox/evidence/${encodeURIComponent(id)}${query(ws)}`),
  recommendation: (ws: string, id: string) => api.get<CRMInboxRecommendation>(`/crm/signal-inbox/recommendations/${encodeURIComponent(id)}${query(ws)}`),
  accept: (ws: string, id: string, revision: string, edits?: Record<string, unknown>) => api.post<CRMSuggestion>(`/crm/signal-inbox/recommendations/${encodeURIComponent(id)}/accept${query(ws)}`, { revision, ...(edits ? { edits } : {}) }),
  dismiss: (ws: string, id: string, revision: string, reason: CRMSignalDismissalReason) => api.post<CRMSuggestion>(`/crm/signal-inbox/recommendations/${encodeURIComponent(id)}/dismiss${query(ws)}`, { revision, reason }),
};
