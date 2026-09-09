import { api } from '@/lib/api';

export interface PMAISuggestion {
  id: string;
  title: string;
  meeting_id: string;
  meeting_title: string;
  meeting_at: string | null;
  created_at: string;
}
export interface PMAISuggestionDetail extends PMAISuggestion {
  draft_subject: string;
  draft_body: string;
  revision: string;
}
export interface PMAISuggestionsPage {
  data: PMAISuggestion[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}
const query = (ws: string) => `workspace_id=${encodeURIComponent(ws)}`;
export const pmAISuggestionsService = {
  list: (ws: string, page: number) => api.get<PMAISuggestionsPage>(`/pm/ai-suggestions?${query(ws)}&page=${page}`),
  detail: (ws: string, id: string) => api.get<PMAISuggestionDetail>(`/pm/ai-suggestions/${encodeURIComponent(id)}?${query(ws)}`),
  decide: (ws: string, id: string, decision: 'accept' | 'dismiss', revision: string) =>
    api.post<{ id: string; status: string; execution_status: string }>(`/pm/ai-suggestions/${encodeURIComponent(id)}/${decision}?${query(ws)}`, { revision }),
};

export interface RoutingRecheckResult {
  billing_error?: string;
  failure?: string;
  items: { id: string; title: string; outcome: 'internal' | 'customer' | 'uncertain' | 'missing_transcript' | 'not_eligible' | 'changed' | 'retry_needed' }[];
  next_cursor?: string;
}
export const recheckSuggestionRouting = (ws: string, cursor?: string) =>
  api.post<RoutingRecheckResult>(`/pm/ai-suggestions/recheck-routing?${query(ws)}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, {});
